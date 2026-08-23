package exporter

import (
	feature "github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/pkg"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

var mainFromInjectedProject = framework.MainFromInjectedProject

func NewFeature() framework.Feature {
	return feature.NewFeature(featurekit.SpecOptions{FeatureName: framework.InjectedFeatureName()})
}

func newFeature(featureName string) framework.Feature {
	return feature.NewFeature(featurekit.SpecOptions{FeatureName: featureName})
}

func Main() {
	metadata := framework.InjectedProjectMetadata()
	mainFromInjectedProject(newFeature(metadata.FeatureName))
}

func Info() framework.ExporterInfo {
	return framework.ExporterInfoFromInjectedProject(NewFeature())
}

func InfoErr() (framework.ExporterInfo, error) {
	return framework.ExporterInfoFromInjectedProjectErr(NewFeature())
}
