# Metrics

## Health Metrics

### `pkg_exporter_last_collection_success`

- Type: gauge
- Value:
  - `1` if the latest refresh cycle completed successfully
  - `0` if the latest refresh cycle failed
- Labels: none
- Notes:
  - When cached package data exists, the exporter continues to expose the last successful business metrics even if this metric becomes `0`
  - Collection runs in the background; scrapes report the latest completed refresh state

### `pkg_exporter_last_collection_timestamp_seconds`

- Type: gauge
- Value: Unix timestamp of the last package data collection attempt
- Labels: none
- Notes:
  - updated on both successful and failed refresh attempts
  - `0` before the first collection attempt

### `pkg_exporter_last_successful_collection_timestamp_seconds`

- Type: gauge
- Value: Unix timestamp of the last successful package data collection
- Labels: none
- Notes:
  - remains at the previous successful timestamp when later refresh attempts fail
  - `0` before the first successful refresh
  - business package metrics are omitted until the first successful refresh

### `pkg_exporter_collection_duration_seconds`

- Type: histogram
- Value: framework refresh duration in seconds
- Labels: none
- Notes:
  - framework-owned metric added by the shared exporter framework
  - measures the outer snapshot refresh cycle
  - use `pkg_exporter_package_collection_duration_seconds` for apt/dpkg domain collection timing

## Business Metrics

### `os_reboot_required`

- Type: gauge
- Value:
  - `1` if `/run/reboot-required` exists
  - `0` otherwise
- Labels: none
- Notes: host reboot requirement state

### `apt_packages_installed_count`

- Type: gauge
- Value: number of installed packages from dpkg status
- Labels: none
- Notes: current package inventory size

### `apt_packages_upgradable_count`

- Type: gauge
- Value: number of upgradable packages in the selected category
- Labels:
  - `category`
- Notes:
  - exported categories are `all` and `security`
  - upgrade candidates are selected from local apt metadata using release priorities, so lower-priority backports are not counted as normal upgrades

### `apt_packages_pinned_updates_count`

- Type: gauge
- Value: number of packages with a newer available version that is not selected by apt policy
- Labels:
  - `category`
- Notes:
  - exported categories are `all` and `security`
  - this captures updates hidden by lower-priority non-backports archives, apt pinning, or held packages
  - backports updates are excluded and reported by `apt_packages_backports_updates_count`
  - a package can be counted here even when a lower version is available as a normal upgrade candidate

### `apt_packages_backports_updates_count`

- Type: gauge
- Value: number of packages with a newer available version in backports archives
- Labels: none
- Notes:
  - this counts backports availability separately from apt policy selection
  - packages already installed from backports are counted when a newer backports version exists

### `apt_packages_broken_count`

- Type: gauge
- Value: number of broken packages detected from dpkg status
- Labels: none
- Notes: derived from dpkg status

### `pkg_exporter_upgrade_metrics_valid`

- Type: gauge
- Value:
  - `1` when apt package index cache is available and upgrade-related metrics are valid
  - `0` when apt package index cache is missing and upgrade-related metrics should be treated as unknown
- Labels: none
- Notes:
  - applies to `apt_packages_upgradable_count`, `apt_packages_pinned_updates_count`, and `apt_packages_backports_updates_count`

## Diagnostics Metrics

### `pkg_exporter_package_collection_duration_seconds`

- Type: histogram
- Value: package data collection duration in seconds
- Labels: none
- Notes:
  - measures package-specific apt/dpkg collection and reboot marker lookup
  - only new refreshes are observed; serving cached snapshots between refresh intervals does not add observations

### `pkg_exporter_collection_stage_duration_seconds`

- Type: histogram
- Value: package data collection stage duration in seconds
- Labels:
  - `stage`
- Stages:
  - `status_read`
  - `status_parse`
  - `index_revision`
  - `index_read`
  - `index_parse`
  - `count`
- Notes:
  - cache hits still observe `index_revision`, but do not observe `index_read` or `index_parse`

### `pkg_exporter_package_index_cache_hits_total`

- Type: counter
- Value: cumulative number of package index cache hits
- Labels: none

### `pkg_exporter_package_index_cache_misses_total`

- Type: counter
- Value: cumulative number of package index cache misses
- Labels: none

### `pkg_exporter_apt_lists_last_modified_timestamp_seconds`

- Type: gauge
- Value: Unix timestamp of the newest relevant file in `/var/lib/apt/lists`
- Labels: none
- Notes:
  - considers package index files and release metadata files used by the exporter
  - not emitted when no apt lists timestamp is available

### `pkg_exporter_apt_lists_files_count`

- Type: gauge
- Value: number of apt package index files grouped by compression
- Labels:
  - `compression`
- Exported compression labels:
  - `plain`
  - `gz`
  - `bz2`
  - `lz4`
  - `xz`
- Notes:
  - `sum by (instance) (pkg_exporter_apt_lists_files_count) == 0` means apt package index cache is missing
  - use `pkg_exporter_upgrade_metrics_valid` when filtering upgrade-related panels and alerts

### `pkg_exporter_dpkg_status_last_modified_timestamp_seconds`

- Type: gauge
- Value: Unix timestamp of `/var/lib/dpkg/status` last modification
- Labels: none
- Notes:
  - not emitted when no dpkg status timestamp is available
