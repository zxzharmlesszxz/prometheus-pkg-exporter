package main

import (
	"os"
	"strings"
	"testing"
)

func TestDockerComposeContract(t *testing.T) {
	data, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)

	for _, forbidden := range []string{
		"COMPOSE_EXPORTER_PORT",
		"__" + "PROJECT_NAME" + "__",
		"__" + "FEATURE_NAME" + "__",
		"__" + "FEATURE_CONFIG_FILE" + "__",
		"__" + "DEFAULT_PORT" + "__",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("docker-compose.yml contains forbidden token %q", forbidden)
		}
	}

	for _, required := range []string{
		"COMPOSE_EXPORTER_HOST_PORT",
		"PROMETHEUS_IMAGE",
		"GRAFANA_IMAGE",
		"DS_PROMETHEUS",
		"./examples/prometheus/:/etc/prometheus/rules/:ro",
		"./examples/grafana/:/var/lib/grafana/dashboards/:ro",
		"./examples/grafana/alerting/:/etc/grafana/provisioning/alerting/:ro",
		"--web.listen-address=:9888",
		"exporter:9888",
	} {
		if !strings.Contains(raw, required) {
			t.Fatalf("docker-compose.yml does not contain required token %q", required)
		}
	}

	if _, err := os.Stat("examples/grafana/alerting"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
