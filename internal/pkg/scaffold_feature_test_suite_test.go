package pkg

import (
	"time"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/featuretest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

const (
	testFeatureName      = "pkg"
	testMetricNamespace  = "pkg_exporter"
	testExporterName     = "prometheus-pkg-exporter"
	testRefreshInterval  = time.Minute
	testLastSuccess      = testMetricNamespace + "_last_collection_success"
	testLastTimestamp    = testMetricNamespace + "_last_collection_timestamp_seconds"
	testLastSuccessfulTS = testMetricNamespace + "_last_successful_collection_timestamp_seconds"
)

type FeatureTestSpec = featuretest.FeatureTestSpec[Config, Snapshot]

type FeatureTestSuite = featuretest.FeatureTestSuite[Config, Snapshot]

func NewFeatureTestSuite(spec FeatureTestSpec) *FeatureTestSuite {
	spec.FeatureName = testFeatureName
	spec.MetricNamespace = testMetricNamespace
	spec.ExporterName = testExporterName
	spec.DefaultRefreshInterval = DefaultRefreshInterval
	spec.DefaultFeatureConfigFileName = DefaultFeatureConfigFileName
	if spec.NewFeature == nil {
		spec.NewFeature = newTestExporterWithOptions
	}
	if spec.NewDefaultConfig == nil {
		spec.NewDefaultConfig = NewDefaultConfig
	}
	if spec.FeatureConfigFile == nil {
		spec.FeatureConfigFile = FeatureConfigFile
	}
	if spec.NewConfigFileTarget == nil {
		spec.NewConfigFileTarget = func() any {
			return &featureConfigFile{}
		}
	}
	if spec.MetricSpecs == nil {
		spec.MetricSpecs = featureMetricSpecs
	}
	if spec.MetricsFunc == nil {
		spec.MetricsFunc = func(ctx featurekit.SnapshotMetricsContext[Snapshot]) featurekit.SnapshotMetrics[Snapshot] {
			return featurekit.NewFeatureMetrics(ctx, featureMetricSpecs, NewFeatureMetricHandlers())
		}
	}
	if spec.StatusFunc == nil {
		spec.StatusFunc = FeatureSnapshotStatus
	}
	if spec.DefaultSnapshotter == nil {
		spec.DefaultSnapshotter = NewDefaultSnapshotEngine()
	}
	return featuretest.NewFeatureTestSuite[Config, Snapshot](spec)
}

func newTestExporterWithOptions(options featurekit.SpecOptions) *featurekit.Feature[Config, Snapshot] {
	return NewFeature(options)
}
