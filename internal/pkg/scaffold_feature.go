package pkg

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

func NewFeature(options featurekit.SpecOptions) *featurekit.Feature[Config, Snapshot] {
	return featurekit.NewSnapshotExtensionFeature(options, featurekit.SnapshotFeatureExtension[Config, Snapshot]{
		DefaultRefreshInterval: DefaultRefreshInterval,
		DefaultConfigFunc:      NewDefaultConfig,
		ConfigFileFunc:         FeatureConfigFile,
		ConfigFlagSpecs:        featureConfigFlagSpecs,
		ValidateConfigFunc:     ValidateFeatureConfig,
		ResolveConfigFunc:      ResolveFeatureConfig,
		RuntimeConfigFunc:      FeatureRuntimeConfigEntries,

		SnapshotEngineFactory: NewSnapshotEngine,
		DefaultSnapshotEngine: NewDefaultSnapshotEngine(),
		StatusFunc:            FeatureSnapshotStatus,

		MetricSpecs:    featureMetricSpecs,
		MetricHandlers: NewFeatureMetricHandlers(),

		SmokeFunc: FeatureSmoke,
	})
}
