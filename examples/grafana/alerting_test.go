package grafana_test

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

var scaffoldPlaceholderPattern = regexp.MustCompile(`__[A-Z0-9_]+__`)

type prometheusRuleFile struct {
	Groups []struct {
		Rules []struct {
			Alert       string            `yaml:"alert"`
			Expr        string            `yaml:"expr"`
			For         string            `yaml:"for"`
			Labels      map[string]string `yaml:"labels"`
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

type grafanaRuleFile struct {
	Groups []struct {
		Rules []struct {
			Title       string            `yaml:"title"`
			Condition   string            `yaml:"condition"`
			For         string            `yaml:"for"`
			Labels      map[string]string `yaml:"labels"`
			Annotations map[string]string `yaml:"annotations"`
			Data        []struct {
				RefID         string `yaml:"refId"`
				DatasourceUID string `yaml:"datasourceUid"`
				Model         struct {
					Expr       string `yaml:"expr"`
					Expression string `yaml:"expression"`
				} `yaml:"model"`
			} `yaml:"data"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

func TestGrafanaAlertsMirrorPrometheusMetadata(t *testing.T) {
	const prometheusRulesPath = "../prometheus/prometheus-pkg-exporter.yml"
	const grafanaRulesPath = "alerting/prometheus-pkg-exporter.yml"

	if _, err := os.Stat(grafanaRulesPath); errors.Is(err, os.ErrNotExist) {
		t.Skipf("no Grafana alerting rules found at examples/grafana/%s", grafanaRulesPath)
	} else if err != nil {
		t.Fatal(err)
	}

	var prometheus prometheusRuleFile
	readYAML(t, prometheusRulesPath, &prometheus)
	var grafana grafanaRuleFile
	readYAML(t, grafanaRulesPath, &grafana)

	want := make(map[string]struct {
		Expr        string
		For         string
		Severity    string
		Summary     string
		Description string
	})
	for _, group := range prometheus.Groups {
		for _, rule := range group.Rules {
			want[rule.Alert] = struct {
				Expr        string
				For         string
				Severity    string
				Summary     string
				Description string
			}{rule.Expr, rule.For, rule.Labels["severity"], rule.Annotations["summary"], rule.Annotations["description"]}
		}
	}

	seen := make(map[string]bool)
	for _, group := range grafana.Groups {
		for _, rule := range group.Rules {
			expected, ok := want[rule.Title]
			if !ok {
				t.Errorf("Grafana rule %q has no Prometheus counterpart", rule.Title)
				continue
			}
			seen[rule.Title] = true
			if rule.For != expected.For || rule.Labels["severity"] != expected.Severity || rule.Annotations["summary"] != expected.Summary {
				t.Errorf("Grafana rule %q metadata differs from Prometheus", rule.Title)
			}
			wantDescription := strings.Replace(expected.Description, "$value", "$values.A.Value", 1)
			if rule.Annotations["description"] != wantDescription {
				t.Errorf("Grafana rule %q description differs from Prometheus", rule.Title)
			}
			if service := rule.Labels["service"]; service != "" && service != "prometheus-pkg-exporter" {
				t.Errorf("Grafana rule %q service label = %q, want prometheus-pkg-exporter", rule.Title, service)
			}
			if ruleSource := rule.Labels["rule_source"]; ruleSource != "" && ruleSource != "grafana" {
				t.Errorf("Grafana rule %q rule_source label = %q, want grafana", rule.Title, ruleSource)
			}
			if rule.Condition == "" || len(rule.Data) == 0 {
				t.Errorf("Grafana rule %q has invalid query wiring", rule.Title)
				continue
			}
			if !equivalentExpression(expected.Expr, rule.Data) {
				t.Errorf("Grafana rule %q expression differs from Prometheus", rule.Title)
			}
		}
	}

	for name := range want {
		if !seen[name] {
			t.Errorf("Prometheus rule %q has no Grafana counterpart", name)
		}
	}
}

func TestDashboardSanity(t *testing.T) {
	const dashboardPath = "prometheus-pkg-exporter.json"
	const expectedDashboardName = "prometheus-pkg-exporter"

	data, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatal(err)
	}

	var dashboard map[string]any
	if err := json.Unmarshal(data, &dashboard); err != nil {
		t.Fatalf("dashboard JSON is invalid: %v", err)
	}

	raw := string(data)
	if match := scaffoldPlaceholderPattern.FindString(raw); match != "" {
		t.Fatalf("dashboard still contains scaffold placeholder %q", match)
	}
	for _, forbidden := range []string{
		"COMPOSE_EXPORTER_PORT",
		"__" + "PROJECT_NAME" + "__",
		"__" + "FEATURE_NAME" + "__",
		"__" + "METRIC_NAMESPACE" + "__",
		"__" + "FEATURE_NAMESPACE" + "__",
		"__" + "DEFAULT_PORT" + "__",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("dashboard contains forbidden token %q", forbidden)
		}
	}
	if !strings.Contains(raw, "DS_PROMETHEUS") {
		t.Fatalf("dashboard does not reference DS_PROMETHEUS")
	}
	if kind, _ := dashboard["kind"].(string); kind != "Dashboard" {
		t.Fatalf("dashboard kind = %q, want Dashboard", kind)
	}
	metadata, _ := dashboard["metadata"].(map[string]any)
	if name, _ := metadata["name"].(string); name != expectedDashboardName {
		t.Fatalf("dashboard metadata.name = %q, want %q", name, expectedDashboardName)
	}
}

func equivalentExpression(prometheusExpr string, data []struct {
	RefID         string `yaml:"refId"`
	DatasourceUID string `yaml:"datasourceUid"`
	Model         struct {
		Expr       string `yaml:"expr"`
		Expression string `yaml:"expression"`
	} `yaml:"model"`
}) bool {
	want := normalizeExpression(prometheusExpr)

	for _, item := range data {
		if item.DatasourceUID != "DS_PROMETHEUS" || item.Model.Expr == "" {
			continue
		}
		query := normalizeExpression(item.Model.Expr)
		if query == want {
			return true
		}
		for _, condition := range grafanaExpressions(data) {
			expanded := strings.ReplaceAll(condition, "$"+item.RefID, item.Model.Expr)
			expanded = replaceBareRef(expanded, item.RefID, item.Model.Expr)
			if normalizeExpression(expanded) == want {
				return true
			}
		}
	}
	return false
}

func grafanaExpressions(data []struct {
	RefID         string `yaml:"refId"`
	DatasourceUID string `yaml:"datasourceUid"`
	Model         struct {
		Expr       string `yaml:"expr"`
		Expression string `yaml:"expression"`
	} `yaml:"model"`
}) []string {
	var expressions []string
	for _, item := range data {
		if item.Model.Expression != "" {
			expressions = append(expressions, item.Model.Expression)
		}
	}
	return expressions
}

func replaceBareRef(expression, refID, replacement string) string {
	fields := strings.Fields(expression)
	for index, field := range fields {
		if field == refID {
			fields[index] = replacement
		}
	}
	return strings.Join(fields, " ")
}

func normalizeExpression(value string) string {
	return strings.Join(strings.Fields(value), " ")
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
