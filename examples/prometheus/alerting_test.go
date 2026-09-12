package prometheus_test

import (
	"os"
	"testing"

	"go.yaml.in/yaml/v3"
)

type prometheusRuleFile struct {
	Groups []struct {
		Rules []struct {
			Alert string `yaml:"alert"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

type prometheusRuleTestFile struct {
	Tests []struct {
		AlertRuleTest []struct {
			AlertName string `yaml:"alertname"`
		} `yaml:"alert_rule_test"`
	} `yaml:"tests"`
}

func TestPrometheusAlertsHaveRuleTests(t *testing.T) {
	const rulesPath = "prometheus-pkg-exporter.yml"
	const testsPath = "tests/prometheus-pkg-exporter.test.yml"

	var rules prometheusRuleFile
	readYAML(t, rulesPath, &rules)
	var tests prometheusRuleTestFile
	readYAML(t, testsPath, &tests)

	want := make(map[string]bool)
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Alert == "" {
				continue
			}
			want[rule.Alert] = false
		}
	}

	seen := make(map[string]bool)
	for _, test := range tests.Tests {
		for _, ruleTest := range test.AlertRuleTest {
			if ruleTest.AlertName == "" {
				continue
			}
			if _, ok := want[ruleTest.AlertName]; !ok {
				t.Errorf("Prometheus rule test %q has no alert rule counterpart", ruleTest.AlertName)
				continue
			}
			want[ruleTest.AlertName] = true
			seen[ruleTest.AlertName] = true
		}
	}

	for name, covered := range want {
		if !covered {
			t.Errorf("Prometheus alert %q has no rule test coverage", name)
		}
	}
	if len(want) > 0 && len(seen) == 0 {
		t.Fatalf("Prometheus alert rules exist, but %s has no alert_rule_test entries", testsPath)
	}
}

func readYAML(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
