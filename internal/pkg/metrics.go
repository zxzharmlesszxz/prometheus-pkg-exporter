package pkg

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

const (
	metricRebootRequired                  = "reboot_required"
	metricPackagesInstalled               = "packages_installed"
	metricPackagesUpgradable              = "packages_upgradable"
	metricPackagesPinnedUpdates           = "packages_pinned_updates"
	metricPackagesBackportsUpdates        = "packages_backports_updates"
	metricPackagesBroken                  = "packages_broken"
	metricPackageCollectionDuration       = "package_collection_duration"
	metricCollectionStageDuration         = "collection_stage_duration"
	metricPackageIndexCacheHits           = "package_index_cache_hits"
	metricPackageIndexCacheMisses         = "package_index_cache_misses"
	metricUpgradeMetricsValid             = "upgrade_metrics_valid"
	metricDPKGStatusLastModifiedTimestamp = "dpkg_status_last_modified_timestamp"
	metricAPTListsLastModifiedTimestamp   = "apt_lists_last_modified_timestamp"
	metricAPTListsFilesCount              = "apt_lists_files_count"
	metricStatusDatabaseUp                = "status_database_up"
	metricStatusDatabaseValid             = "status_database_valid"
	metricStatusDatabaseReadErrors        = "status_database_read_errors"
	metricStatusDatabaseParseErrors       = "status_database_parse_errors"
	metricAPTListsUp                      = "apt_lists_up"
	metricAPTListsValid                   = "apt_lists_valid"
	metricAPTListsReadErrors              = "apt_lists_read_errors"
	metricAPTListsParseErrors             = "apt_lists_parse_errors"
)

var featureMetricSpecs = []featurekit.FeatureMetricSpec{
	{
		ID:    metricRebootRequired,
		Scope: featurekit.MetricScopeAbsolute,
		Name:  "pkg_reboot_required",
		Help:  "Node requires a reboot",
	},
	{
		ID:    metricPackagesInstalled,
		Scope: featurekit.MetricScopeAbsolute,
		Name:  "pkg_packages_installed_count",
		Help:  "Number of installed packages",
	},
	{
		ID:     metricPackagesUpgradable,
		Scope:  featurekit.MetricScopeAbsolute,
		Name:   "pkg_packages_upgradable_count",
		Help:   "Number of upgradable packages by category",
		Labels: []string{"category"},
	},
	{
		ID:     metricPackagesPinnedUpdates,
		Scope:  featurekit.MetricScopeAbsolute,
		Name:   "pkg_packages_pinned_updates_count",
		Help:   "Number of packages with newer available versions that are not selected by apt policy",
		Labels: []string{"category"},
	},
	{
		ID:    metricPackagesBackportsUpdates,
		Scope: featurekit.MetricScopeAbsolute,
		Name:  "pkg_packages_backports_updates_count",
		Help:  "Number of packages with newer available versions in backports archives",
	},
	{
		ID:    metricPackagesBroken,
		Scope: featurekit.MetricScopeAbsolute,
		Name:  "pkg_packages_broken_count",
		Help:  "Number of broken packages",
	},
	{
		ID:    metricPackageCollectionDuration,
		Scope: featurekit.MetricScopeNamespace,
		Name:  "_package_collection_duration_seconds",
		Help:  "Package data collection duration in seconds",
	},
	{
		ID:     metricCollectionStageDuration,
		Scope:  featurekit.MetricScopeNamespace,
		Name:   "_collection_stage_duration_seconds",
		Help:   "Package data collection stage duration in seconds",
		Labels: []string{"stage"},
	},
	{
		ID:    metricPackageIndexCacheHits,
		Scope: featurekit.MetricScopeNamespace,
		Name:  "_package_index_cache_hits_total",
		Help:  "Total number of package index cache hits",
	},
	{
		ID:    metricPackageIndexCacheMisses,
		Scope: featurekit.MetricScopeNamespace,
		Name:  "_package_index_cache_misses_total",
		Help:  "Total number of package index cache misses",
	},
	{
		ID:    metricUpgradeMetricsValid,
		Scope: featurekit.MetricScopeNamespace,
		Name:  "_upgrade_metrics_valid",
		Help:  "Whether upgrade-related package metrics are based on available apt package index cache",
	},
	{
		ID:    metricDPKGStatusLastModifiedTimestamp,
		Scope: featurekit.MetricScopeNamespace,
		Name:  "_dpkg_status_last_modified_timestamp_seconds",
		Help:  "Unix timestamp of the dpkg status database last modification",
	},
	{
		ID:    metricAPTListsLastModifiedTimestamp,
		Scope: featurekit.MetricScopeNamespace,
		Name:  "_apt_lists_last_modified_timestamp_seconds",
		Help:  "Unix timestamp of the newest apt lists file last modification",
	},
	{
		ID:     metricAPTListsFilesCount,
		Scope:  featurekit.MetricScopeNamespace,
		Name:   "_apt_lists_files_count",
		Help:   "Number of apt package index files by compression",
		Labels: []string{"compression"},
	},
	{ID: metricStatusDatabaseUp, Scope: featurekit.MetricScopeAbsolute, Name: "pkg_status_database_up", Help: "Whether the dpkg status database was readable during the latest collection"},
	{ID: metricStatusDatabaseValid, Scope: featurekit.MetricScopeAbsolute, Name: "pkg_status_database_valid", Help: "Whether the dpkg status database was valid during the latest collection"},
	{ID: metricStatusDatabaseReadErrors, Scope: featurekit.MetricScopeAbsolute, Name: "pkg_status_database_read_errors_total", Help: "Total number of dpkg status database read errors"},
	{ID: metricStatusDatabaseParseErrors, Scope: featurekit.MetricScopeAbsolute, Name: "pkg_status_database_parse_errors_total", Help: "Total number of dpkg status database parse errors"},
	{ID: metricAPTListsUp, Scope: featurekit.MetricScopeAbsolute, Name: "pkg_apt_lists_up", Help: "Whether the apt lists source was readable during the latest collection"},
	{ID: metricAPTListsValid, Scope: featurekit.MetricScopeAbsolute, Name: "pkg_apt_lists_valid", Help: "Whether the apt lists source contained valid package indexes during the latest collection"},
	{ID: metricAPTListsReadErrors, Scope: featurekit.MetricScopeAbsolute, Name: "pkg_apt_lists_read_errors_total", Help: "Total number of apt lists read errors"},
	{ID: metricAPTListsParseErrors, Scope: featurekit.MetricScopeAbsolute, Name: "pkg_apt_lists_parse_errors_total", Help: "Total number of apt lists parse errors"},
}
