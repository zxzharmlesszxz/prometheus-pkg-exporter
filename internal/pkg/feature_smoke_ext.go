package pkg

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

func FeatureSmoke(ctx featurekit.SmokeContext[Config]) featurekit.SmokeSpec {
	return featurekit.SmokeSpec{
		ServerArgs: []string{
			"--" + ctx.FeatureName + ".config-file=../examples/" + DefaultFeatureConfigFileName,
			"--" + ctx.FeatureName + ".reboot-required-path=../testdata/reboot-required",
			"--" + ctx.FeatureName + ".status-database-path=../testdata/dpkg-status",
			"--" + ctx.FeatureName + ".apt-lists-path=../testdata/apt-lists",
		},
		WantMetrics: []string{
			featurekit.FeatureMetricName("", "", metricPackagesInstalled, featureMetricSpecs),
			featurekit.FeatureMetricName("", "", metricPackagesUpgradable, featureMetricSpecs) + `{category="all"}`,
			featurekit.FeatureMetricName("", "", metricPackagesUpgradable, featureMetricSpecs) + `{category="security"}`,
			featurekit.FeatureMetricName("", "", metricRebootRequired, featureMetricSpecs) + " 1",
		},
		RejectMetrics: []string{
			featurekit.FeatureMetricName("", "", metricRebootRequired, featureMetricSpecs) + " 0",
		},
	}
}
