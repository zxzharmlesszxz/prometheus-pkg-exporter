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

func Main() {
	mainFromInjectedProject(NewFeature())
}

func ExporterInfo() framework.ExporterInfo {
	return framework.ExporterInfoFromInjectedProject(NewFeature())
}
