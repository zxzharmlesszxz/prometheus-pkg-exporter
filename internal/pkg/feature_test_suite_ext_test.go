package pkg

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/featuretest"
	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/apt"
	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/pkgcheck"
)

func TestFeatureContract(t *testing.T) {
	suite := NewFeatureTestSuite(NewFeatureTestSpec())
	RegisterFeatureTests(suite)
	suite.RunTests(t)
}

func NewFeatureTestSpec() FeatureTestSpec {
	return FeatureTestSpec{
		SuccessfulSnapshot: func(at time.Time) Snapshot {
			return Snapshot{
				pkg: pkgcheck.Snapshot{
					AttemptTime: at,
					Success:     true,
					Stats: apt.Stats{
						Installed:           1,
						UpgradeMetricsValid: true,
						APTListFilesCount:   map[string]int{"plain": 1},
					},
				},
			}
		},
		FailedSnapshot: func(at time.Time, err error) Snapshot {
			return Snapshot{
				pkg: pkgcheck.Snapshot{
					AttemptTime: at,
					Success:     false,
					Err:         err,
				},
			}
		},
		ContractFlagArgs: []string{
			"--" + testFeatureName + ".command-timeout=7s",
			"--" + testFeatureName + ".reboot-required-path=/tmp/reboot-required",
			"--" + testFeatureName + ".status-database-path=/tmp/dpkg-status",
			"--" + testFeatureName + ".apt-lists-path=/tmp/apt-lists",
		},
		ContractRuntimeConfig: map[string]any{
			"command_timeout":      7 * time.Second,
			"reboot_required_path": "/tmp/reboot-required",
			"status_database_path": "/tmp/dpkg-status",
			"apt_lists_path":       "/tmp/apt-lists",
		},
		SkipContractLastCollectionSuccessMetric: true,
		SkipRegisterCollectorsTest:              true,
		DefaultRuntimeConfig: map[string]any{
			"command_timeout":      DefaultCommandTimeout,
			"reboot_required_path": DefaultRebootRequiredPath,
			"status_database_path": DefaultStatusDatabasePath,
			"apt_lists_path":       DefaultAPTListsPath,
		},
	}
}

func RegisterFeatureTests(suite *FeatureTestSuite) {
	suite.Register("collector_exports_snapshot", func(t *testing.T) { testCollectorExportsSnapshot(t, suite) })
	suite.Register("collector_uses_provided_snapshotter", func(t *testing.T) { testCollectorUsesProvidedSnapshotter(t, suite) })
	suite.Register("collector_uses_cached_snapshot_on_failure", func(t *testing.T) { testCollectorUsesCachedSnapshotOnFailure(t, suite) })
	suite.Register("collector_background_refresh_updates_snapshot_outside_scrape", func(t *testing.T) { testCollectorBackgroundRefreshUpdatesSnapshotOutsideScrape(t, suite) })
	suite.Register("collector_skips_zero_source_timestamps", func(t *testing.T) { testCollectorSkipsZeroSourceTimestamps(t, suite) })
	suite.Register("collector_marks_upgrade_metrics_invalid_without_apt_cache", func(t *testing.T) { testCollectorMarksUpgradeMetricsInvalidWithoutAPTCache(t, suite) })
	suite.Register("exporter_registers_collector_with_fixture_source", func(t *testing.T) { testExporterRegistersCollectorWithFixtureSource(t, suite) })
	suite.Register("exporter_runtime_config_normalizes_values", func(t *testing.T) { testExporterRuntimeConfigNormalizesValues(t, suite) })
	suite.Register("smoke_spec_includes_fixture_args", func(t *testing.T) { testSmokeSpecIncludesFixtureArgs(t, suite) })
}

func testCollectorExportsSnapshot(t *testing.T, suite *FeatureTestSuite) {
	snapshotter := &fakePackageSnapshotter{
		snapshot: pkgcheck.Snapshot{
			CollectionDurationSeconds: 0.5,
			Stats: apt.Stats{
				Installed:               123,
				Upgradable:              3,
				SecurityUpgradable:      1,
				PinnedUpdates:           2,
				SecurityPinnedUpdates:   1,
				BackportsUpdates:        4,
				Broken:                  3,
				UpgradeMetricsValid:     true,
				PackageIndexCacheHits:   7,
				PackageIndexCacheMisses: 2,
				DPKGStatusLastModified:  time.Unix(1000, 0),
				APTListsLastModified:    time.Unix(2000, 0),
				APTListFilesCount:       map[string]int{"plain": 1, "gz": 2, "bz2": 3, "lz4": 4, "xz": 5},
				StageDurations:          map[string]float64{"status_read": 0.01, "status_parse": 0.02, "index_revision": 0.03, "index_read": 0.04, "index_parse": 0.05, "count": 0.06},
			},
			RebootRequired: true,
		},
	}

	collector := suite.NewCollector(testFeatureName, testMetricNamespace, &observedPkgSnapshotter{inner: pkgcheck.NewObservedSnapshotter(snapshotter)}, time.Minute)

	expected := `
# HELP apt_packages_broken_count Number of broken packages
# TYPE apt_packages_broken_count gauge
apt_packages_broken_count 3
# HELP apt_packages_backports_updates_count Number of packages with newer available versions in backports archives
# TYPE apt_packages_backports_updates_count gauge
apt_packages_backports_updates_count 4
# HELP pkg_exporter_apt_lists_files_count Number of apt package index files by compression
# TYPE pkg_exporter_apt_lists_files_count gauge
pkg_exporter_apt_lists_files_count{compression="bz2"} 3
pkg_exporter_apt_lists_files_count{compression="gz"} 2
pkg_exporter_apt_lists_files_count{compression="lz4"} 4
pkg_exporter_apt_lists_files_count{compression="plain"} 1
pkg_exporter_apt_lists_files_count{compression="xz"} 5
# HELP pkg_exporter_apt_lists_last_modified_timestamp_seconds Unix timestamp of the newest apt lists file last modification
# TYPE pkg_exporter_apt_lists_last_modified_timestamp_seconds gauge
pkg_exporter_apt_lists_last_modified_timestamp_seconds 2000
# HELP pkg_exporter_dpkg_status_last_modified_timestamp_seconds Unix timestamp of the dpkg status database last modification
# TYPE pkg_exporter_dpkg_status_last_modified_timestamp_seconds gauge
pkg_exporter_dpkg_status_last_modified_timestamp_seconds 1000
# HELP apt_packages_installed_count Number of installed packages
# TYPE apt_packages_installed_count gauge
apt_packages_installed_count 123
# HELP apt_packages_pinned_updates_count Number of packages with newer available versions that are not selected by apt policy
# TYPE apt_packages_pinned_updates_count gauge
apt_packages_pinned_updates_count{category="all"} 2
apt_packages_pinned_updates_count{category="security"} 1
# HELP os_reboot_required Node requires a reboot
# TYPE os_reboot_required gauge
os_reboot_required 1
# HELP apt_packages_upgradable_count Number of upgradable packages by category
# TYPE apt_packages_upgradable_count gauge
apt_packages_upgradable_count{category="all"} 3
apt_packages_upgradable_count{category="security"} 1
# HELP pkg_exporter_last_collection_success Whether the last pkg data collection succeeded
# TYPE pkg_exporter_last_collection_success gauge
pkg_exporter_last_collection_success 1
# HELP pkg_exporter_package_index_cache_hits_total Total number of package index cache hits
# TYPE pkg_exporter_package_index_cache_hits_total counter
pkg_exporter_package_index_cache_hits_total 7
# HELP pkg_exporter_package_index_cache_misses_total Total number of package index cache misses
# TYPE pkg_exporter_package_index_cache_misses_total counter
pkg_exporter_package_index_cache_misses_total 2
# HELP pkg_exporter_upgrade_metrics_valid Whether upgrade-related package metrics are based on available apt package index cache
# TYPE pkg_exporter_upgrade_metrics_valid gauge
pkg_exporter_upgrade_metrics_valid 1
`

	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected),
		"apt_packages_broken_count",
		"apt_packages_backports_updates_count",
		suite.MetricName("", testMetricNamespace, metricAPTListsFilesCount),
		suite.MetricName("", testMetricNamespace, metricAPTListsLastModifiedTimestamp),
		suite.MetricName("", testMetricNamespace, metricDPKGStatusLastModifiedTimestamp),
		suite.MetricName("", "", metricPackagesInstalled),
		suite.MetricName("", "", metricPackagesPinnedUpdates),
		suite.MetricName("", "", metricRebootRequired),
		suite.MetricName("", "", metricPackagesUpgradable),
		testLastSuccess,
		suite.MetricName("", testMetricNamespace, metricPackageIndexCacheHits),
		suite.MetricName("", testMetricNamespace, metricPackageIndexCacheMisses),
		suite.MetricName("", testMetricNamespace, metricUpgradeMetricsValid),
	); err != nil {
		t.Fatalf("CollectAndCompare() error = %v", err)
	}

	families := gatherCollector(t, collector)
	if got := histogramCount(t, families, suite.MetricName("", testMetricNamespace, metricPackageCollectionDuration), nil); got != 1 {
		t.Fatalf("collection duration observations = %d, want 1", got)
	}
	if got := histogramSum(t, families, suite.MetricName("", testMetricNamespace, metricPackageCollectionDuration), nil); got <= 0 {
		t.Fatalf("collection duration sum = %v, want positive", got)
	}
	if got := histogramCount(t, families, suite.MetricName("", testMetricNamespace, metricCollectionStageDuration), map[string]string{"stage": "status_read"}); got != 1 {
		t.Fatalf("status_read duration observations = %d, want 1", got)
	}
}

func testCollectorUsesProvidedSnapshotter(t *testing.T, suite *FeatureTestSuite) {
	now := time.Unix(1700000000, 0)
	snapshotter := suite.NewFakeSnapshotter(Snapshot{})
	snapshotter.Set(Snapshot{
		pkg: pkgcheck.Snapshot{
			AttemptTime: now,
			Success:     true,
			Stats: apt.Stats{
				Installed:           42,
				UpgradeMetricsValid: true,
			},
		},
	})
	collector := suite.NewCollectorWithNow(
		testFeatureName,
		testMetricNamespace,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		snapshotter,
		testRefreshInterval,
		func() time.Time { return now },
	)

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, suite.MetricName("", "", metricPackagesInstalled), nil, 42)
	exportertest.AssertMetricValue(t, families, testLastSuccess, nil, 1)
}

func testCollectorUsesCachedSnapshotOnFailure(t *testing.T, suite *FeatureTestSuite) {
	start := time.Unix(1700000000, 0)
	now := start
	snapshotter := &fakePackageSnapshotter{
		snapshot: pkgcheck.Snapshot{
			Stats: apt.Stats{Installed: 10},
		},
	}
	collector := suite.NewCollectorWithNow(
		testFeatureName,
		testMetricNamespace,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		&observedPkgSnapshotter{inner: pkgcheck.NewObservedSnapshotter(snapshotter)},
		time.Minute,
		func() time.Time { return now },
	)

	families := gatherCollector(t, collector)
	exportertest.AssertMetricValue(t, families, testLastSuccess, nil, 1)
	exportertest.AssertMetricValue(t, families, testLastSuccessfulTS, nil, float64(start.Unix()))
	exportertest.AssertMetricValue(t, families, suite.MetricName("", "", metricPackagesInstalled), nil, 10)

	snapshotter.Set(pkgcheck.Snapshot{}, errors.New("refresh failed"))
	now = start.Add(2 * time.Minute)

	families = gatherCollector(t, collector)
	exportertest.AssertMetricValue(t, families, testLastSuccess, nil, 0)
	exportertest.AssertMetricValue(t, families, testLastTimestamp, nil, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, testLastSuccessfulTS, nil, float64(start.Unix()))
	exportertest.AssertMetricValue(t, families, suite.MetricName("", "", metricPackagesInstalled), nil, 10)
	if got := histogramCount(t, families, suite.MetricName("", testMetricNamespace, metricPackageCollectionDuration), nil); got != 2 {
		t.Fatalf("collection duration observations = %d, want 2", got)
	}
}

func testCollectorBackgroundRefreshUpdatesSnapshotOutsideScrape(t *testing.T, suite *FeatureTestSuite) {
	snapshotter := &fakePackageSnapshotter{
		snapshot: pkgcheck.Snapshot{
			Stats: apt.Stats{
				Installed:           10,
				UpgradeMetricsValid: true,
				APTListFilesCount:   map[string]int{"plain": 1},
			},
		},
	}
	collector := suite.NewCollector(testFeatureName, testMetricNamespace, &observedPkgSnapshotter{inner: pkgcheck.NewObservedSnapshotter(snapshotter)}, 20*time.Millisecond)
	registry := suite.StartCollector(t, collector)

	exportertest.WaitForMetricValue(t, registry, testLastSuccess, nil, 1)

	snapshotter.Set(pkgcheck.Snapshot{}, errors.New("refresh failed"))
	exportertest.WaitForMetricValue(t, registry, testLastSuccess, nil, 0)

	families := exportertest.Gather(t, registry)
	exportertest.AssertMetricValue(t, families, suite.MetricName("", "", metricPackagesInstalled), nil, 10)
}

func testCollectorSkipsZeroSourceTimestamps(t *testing.T, suite *FeatureTestSuite) {
	collector := suite.NewCollector(testFeatureName, testMetricNamespace, &observedPkgSnapshotter{inner: pkgcheck.NewObservedSnapshotter(&fakePackageSnapshotter{
		snapshot: pkgcheck.Snapshot{},
	})}, time.Minute)

	if err := testutil.CollectAndCompare(collector, strings.NewReader(""),
		suite.MetricName("", testMetricNamespace, metricAPTListsLastModifiedTimestamp),
		suite.MetricName("", testMetricNamespace, metricDPKGStatusLastModifiedTimestamp),
	); err != nil {
		t.Fatalf("CollectAndCompare() error = %v", err)
	}
}

func testCollectorMarksUpgradeMetricsInvalidWithoutAPTCache(t *testing.T, suite *FeatureTestSuite) {
	collector := suite.NewCollector(testFeatureName, testMetricNamespace, &observedPkgSnapshotter{inner: pkgcheck.NewObservedSnapshotter(&fakePackageSnapshotter{
		snapshot: pkgcheck.Snapshot{
			Stats: apt.Stats{
				APTListFilesCount: map[string]int{"plain": 0, "gz": 0, "bz2": 0, "lz4": 0, "xz": 0},
			},
		},
	})}, time.Minute)

	expected := `
# HELP pkg_exporter_upgrade_metrics_valid Whether upgrade-related package metrics are based on available apt package index cache
# TYPE pkg_exporter_upgrade_metrics_valid gauge
pkg_exporter_upgrade_metrics_valid 0
`

	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected), suite.MetricName("", testMetricNamespace, metricUpgradeMetricsValid)); err != nil {
		t.Fatalf("CollectAndCompare() error = %v", err)
	}
}

func testExporterRegistersCollectorWithFixtureSource(t *testing.T, suite *FeatureTestSuite) {
	fixtures := filepath.Join("..", "..", "testdata")
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{"--" + testFeatureName + ".config-file=" + suite.WriteConfig(t, `
status_database_path: `+filepath.Join(fixtures, "dpkg-status")+`
apt_lists_path: `+filepath.Join(fixtures, "apt-lists")+`
reboot_required_path: `+filepath.Join(fixtures, "reboot-required")+`
`)})

	registry := suite.RegisterFeatureCollectors(t, exporter)
	exportertest.WaitForMetricValue(t, registry, testLastSuccess, nil, 1)
	families := exportertest.Gather(t, registry)
	exportertest.AssertMetricValue(t, families, suite.MetricName("", "", metricRebootRequired), nil, 1)
}

func testExporterRuntimeConfigNormalizesValues(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{
		"--" + testFeatureName + ".refresh-interval=0s",
		"--" + testFeatureName + ".command-timeout=0s",
		"--" + testFeatureName + ".reboot-required-path=",
		"--" + testFeatureName + ".status-database-path=",
		"--" + testFeatureName + ".apt-lists-path=",
	})

	config := exporter.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "refresh_interval"); got != DefaultRefreshInterval {
		t.Fatalf("refresh_interval = %v, want %v", got, DefaultRefreshInterval)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "command_timeout"); got != DefaultCommandTimeout {
		t.Fatalf("command_timeout = %v, want %v", got, DefaultCommandTimeout)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "reboot_required_path"); got != DefaultRebootRequiredPath {
		t.Fatalf("reboot_required_path = %q, want %q", got, DefaultRebootRequiredPath)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "status_database_path"); got != DefaultStatusDatabasePath {
		t.Fatalf("status_database_path = %q, want %q", got, DefaultStatusDatabasePath)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "apt_lists_path"); got != DefaultAPTListsPath {
		t.Fatalf("apt_lists_path = %q, want %q", got, DefaultAPTListsPath)
	}
}

func testSmokeSpecIncludesFixtureArgs(t *testing.T, suite *FeatureTestSuite) {
	spec := suite.NewNamedFeature().SmokeSpec()
	for _, want := range []string{
		"--" + testFeatureName + ".config-file=../examples/" + DefaultFeatureConfigFileName,
		"--" + testFeatureName + ".reboot-required-path=../testdata/reboot-required",
		"--" + testFeatureName + ".status-database-path=../testdata/dpkg-status",
		"--" + testFeatureName + ".apt-lists-path=../testdata/apt-lists",
	} {
		if !featuretest.HasString(spec.ServerArgs, want) {
			t.Fatalf("SmokeSpec().ServerArgs = %v, want %q", spec.ServerArgs, want)
		}
	}
	if !featuretest.HasString(spec.WantMetrics, suite.MetricName("", "", metricRebootRequired)+" 1") {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want reboot-required metric", spec.WantMetrics)
	}
	if !featuretest.HasString(spec.RejectMetrics, suite.MetricName("", "", metricRebootRequired)+" 0") {
		t.Fatalf("SmokeSpec().RejectMetrics = %v, want reboot-required reject metric", spec.RejectMetrics)
	}
}

type fakePackageSnapshotter struct {
	mu       sync.Mutex
	snapshot pkgcheck.Snapshot
	err      error
	calls    int
}

func (s *fakePackageSnapshotter) Snapshot(context.Context) (pkgcheck.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.snapshot, s.err
}

func (s *fakePackageSnapshotter) Set(snapshot pkgcheck.Snapshot, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = snapshot
	s.err = err
}

type observedPkgSnapshotter struct {
	inner *pkgcheck.ObservedSnapshotter
}

func (s *observedPkgSnapshotter) Snapshot(ctx context.Context, now time.Time) Snapshot {
	return Snapshot{pkg: s.inner.Snapshot(ctx, now)}
}

func gatherCollector(t *testing.T, collector prometheus.Collector) []*dto.MetricFamily {
	t.Helper()

	registry := prometheus.NewRegistry()
	exportertest.Register(t, registry, collector)
	return exportertest.Gather(t, registry)
}

func histogramCount(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string) uint64 {
	t.Helper()

	histogram := exportertest.Histogram(t, families, name, labels)
	return histogram.GetSampleCount()
}

func histogramSum(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string) float64 {
	t.Helper()

	histogram := exportertest.Histogram(t, families, name, labels)
	return histogram.GetSampleSum()
}
