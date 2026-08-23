package exporter

import (
	"testing"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/adaptertest"
)

func TestExporterAdapter(t *testing.T) {
	metadata := framework.ProjectMetadata{
		ExporterName:         "prometheus-pkg-exporter",
		ExporterDescription:  "Prometheus Pkg Exporter",
		FeatureName:          "pkg",
		MetricNamespace:      "pkg_exporter",
		DefaultListenAddress: ":9888",
	}

	adaptertest.RunInjectedAdapterContract(t, adaptertest.InjectedAdapterContractConfig{
		NewFeature: func() framework.Feature {
			return newFeature(metadata.FeatureName)
		},
		Main: func() {
			mainFromInjectedProject(newFeature(metadata.FeatureName))
		},
		ExporterInfo: func() framework.ExporterInfo {
			return framework.ExporterInfoFromProjectMetadata(metadata, newFeature(metadata.FeatureName))
		},
		ReplaceMainFromInjectedProject: func(fn adaptertest.MainFromInjectedProjectFunc) func() {
			oldMain := mainFromInjectedProject
			mainFromInjectedProject = fn
			return func() {
				mainFromInjectedProject = oldMain
			}
		},
		Metadata: metadata,
	})
}

func TestInjectedExporterWrappers(t *testing.T) {
	metadata, err := framework.InjectedProjectMetadataErr()
	if err != nil {
		t.Skipf("skipping injected exporter wrappers without injected metadata: %v", err)
	}

	t.Run("new feature", func(t *testing.T) {
		named, ok := NewFeature().(framework.NamedFeature)
		if !ok {
			t.Fatalf("NewFeature() = %T, want framework.NamedFeature", NewFeature())
		}
		if got := named.FeatureName(); got != metadata.FeatureName {
			t.Fatalf("FeatureName() = %q, want %q", got, metadata.FeatureName)
		}
	})

	t.Run("info", func(t *testing.T) {
		assertExporterInfo(t, Info(), metadata)
	})

	t.Run("info err", func(t *testing.T) {
		info, err := InfoErr()
		if err != nil {
			t.Fatalf("InfoErr() error = %v", err)
		}
		assertExporterInfo(t, info, metadata)
	})

	t.Run("main", func(t *testing.T) {
		called := false
		oldMain := mainFromInjectedProject
		mainFromInjectedProject = func(features ...framework.Feature) {
			called = true
			if len(features) != 1 {
				t.Fatalf("features length = %d, want 1", len(features))
			}
			named, ok := features[0].(framework.NamedFeature)
			if !ok {
				t.Fatalf("feature = %T, want framework.NamedFeature", features[0])
			}
			if got := named.FeatureName(); got != metadata.FeatureName {
				t.Fatalf("FeatureName() = %q, want %q", got, metadata.FeatureName)
			}
		}
		t.Cleanup(func() {
			mainFromInjectedProject = oldMain
		})

		Main()
		if !called {
			t.Fatal("framework main was not called")
		}
	})
}

func assertExporterInfo(t *testing.T, info framework.ExporterInfo, metadata framework.ProjectMetadata) {
	t.Helper()

	if info.Name != metadata.ExporterName {
		t.Fatalf("Name = %q, want %q", info.Name, metadata.ExporterName)
	}
	if info.Description != metadata.ExporterDescription {
		t.Fatalf("Description = %q, want %q", info.Description, metadata.ExporterDescription)
	}
	if info.FeatureName != metadata.FeatureName {
		t.Fatalf("FeatureName = %q, want %q", info.FeatureName, metadata.FeatureName)
	}
	if info.MetricNamespace != metadata.MetricNamespace {
		t.Fatalf("MetricNamespace = %q, want %q", info.MetricNamespace, metadata.MetricNamespace)
	}
	if info.DefaultListenAddress != metadata.DefaultListenAddress {
		t.Fatalf("DefaultListenAddress = %q, want %q", info.DefaultListenAddress, metadata.DefaultListenAddress)
	}
}
