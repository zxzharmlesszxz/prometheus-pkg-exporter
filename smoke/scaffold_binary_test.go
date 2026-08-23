package smoke

import (
	"os"
	"testing"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/smoketest"

	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/exporter"
)

func TestBinarySmoke(t *testing.T) {
	info, err := exporter.InfoErr()
	if err != nil {
		t.Skipf("skipping binary smoke without injected metadata: %v", err)
	}
	smoke := info.Smoke

	smoketest.RunBinary(t, smoketest.Config{
		ProjectName:         info.Name,
		BinaryPath:          os.Getenv("EXPORTER_SMOKE_BINARY"),
		BuildInfoMetric:     info.Metrics.BuildInfo,
		ForbiddenUsageNames: smoke.ForbiddenUsageNames,
		RenamedExecutable:   smoke.RenamedExecutable,
		ServerArgs: func(_ *testing.T, _ string) []string {
			return append([]string(nil), smoke.ServerArgs...)
		},
		WantMetrics:   smoke.WantMetrics,
		RejectMetrics: smoke.RejectMetrics,
	})
}
