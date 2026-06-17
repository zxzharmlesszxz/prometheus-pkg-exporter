package exporter

import (
	"testing"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/adaptertest"
)

func TestExporterAdapter(t *testing.T) {
	adaptertest.RunInjectedAdapterContract(t, adaptertest.InjectedAdapterContractConfig{
		NewFeature:   NewFeature,
		Main:         Main,
		ExporterInfo: ExporterInfo,
		ReplaceMainFromInjectedProject: func(fn adaptertest.MainFromInjectedProjectFunc) func() {
			oldMain := mainFromInjectedProject
			mainFromInjectedProject = fn
			return func() {
				mainFromInjectedProject = oldMain
			}
		},
	})
}
