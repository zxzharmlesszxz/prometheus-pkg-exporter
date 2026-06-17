# Architecture

`prometheus-pkg-exporter` is a thin concrete exporter built on `prometheus-exporter-framework`.

## Package Layout

- `cmd`
  Minimal process entrypoint in scaffold-owned `scaffold_main.go`.
- `internal/exporter`
  Thin scaffold-owned exporter adapter that creates the package feature and delegates injected project metadata to the framework.
- `internal/pkg`
  Package feature package. `scaffold_feature.go` implements the stable
  `featurekit.FeatureContract` bridge supported by the current framework
  release and wires config-file flags, runtime config, collector construction,
  metrics, snapshot status, and smoke behavior through package hook functions.
  Scaffold-owned files use the `scaffold_*.go` suffix; package-specific
  defaults and hook functions live in adjacent non-scaffold feature files.
  `snapshot_types.go` owns the feature snapshot wrapper around
  `internal/pkgcheck.Snapshot`.
- `internal/pkgcheck`
  Package snapshot engine: apt collector orchestration, reboot-required marker
  checks, snapshot caching after successful refreshes, and package snapshot
  types.
- `internal/apt`
  Local apt/dpkg metadata readers plus parsers for package state and apt-like upgrade candidate selection.
- `smoke`
  Binary smoke tests that build the real executable and verify CLI, HTTP, and metric behavior.

## Data Flow

1. `cmd/scaffold_main.go` delegates to `internal/exporter.Main()`, which calls `framework.MainFromInjectedProject`.
   The Makefile injects the metric namespace as `pkg_exporter` and the framework still sets the CLI name from the executable file name.
2. Framework `featurekit.Feature` registers common flags and delegates package-specific flags through extension hooks:
   - `--pkg.refresh-interval`
   - `--pkg.config-file`
   - `--pkg.command-timeout`
   - `--pkg.reboot-required-path`
   - `--pkg.status-database-path`
   - `--pkg.apt-lists-path`
3. `internal/exporter` creates the package feature through `pkg.NewFeature(...)`
   and framework-injected feature metadata.
4. The package feature extension builds an `apt.CommandSource` from the configured paths unless a test source was injected, then builds a feature snapshotter and feature metrics.
5. `framework.SnapshotCollector` refreshes package data in a background worker every `--pkg.refresh-interval`; scrapes read the latest completed snapshot.
6. Snapshot refresh reads dpkg status, apt package indexes, apt release metadata, source file timestamps, and the reboot-required marker.
7. Package upgrade candidates are selected with apt-like release priority semantics.
8. The collector exports package state, reboot state, collection health, source timestamps, package index cache metrics, and package collection duration histograms. The shared framework also exports the outer exporter collection duration histogram.

## Source Semantics

- `/var/lib/dpkg/status` provides installed package inventory and broken package state.
- `*_Packages` files provide available package versions.
- `*_InRelease` and `*_Release` files provide release priority metadata.
- Package upgrade candidates are selected using apt-like priority semantics. Default archives use priority `500`; `NotAutomatic: yes` archives use priority `1`; `NotAutomatic: yes` plus `ButAutomaticUpgrades: yes` archives, such as Debian backports, use priority `100`.
- Newer versions that are present in lower-priority non-backports archives but are not selected as policy candidates are counted separately as pinned updates.
- Held installed packages are also counted as pinned updates when a newer non-backports version exists.
- Newer versions from archives whose release metadata or apt list path contains `backports` are counted separately as backports updates.
- `/run/reboot-required` provides reboot state.

All inputs are host-local. The exporter does not call remote APIs.

## Failure Semantics

If refresh fails before the first successful snapshot, the exporter exposes collection health and duration metrics, but no package business metrics.

If refresh fails after at least one successful snapshot, the exporter serves cached package business metrics and sets:

- `pkg_exporter_last_collection_success = 0`

The last successful timestamp remains unchanged until a later successful refresh.

The `/healthz` endpoint remains `200 OK` while the process is alive even if the latest collection failed.

## Metric Semantics

### Package State Metrics

- `apt_packages_installed_count`
  Number of installed packages from dpkg status.
- `apt_packages_upgradable_count{category="all"}`
  Total packages with an upgrade candidate after apt-like release priority selection.
- `apt_packages_upgradable_count{category="security"}`
  Upgradable packages whose selected candidate is present in a security archive.
- `apt_packages_pinned_updates_count{category="all"}`
  Packages with a newer available version in a non-backports archive that is not selected by apt policy because a higher-priority candidate wins, or because the installed package is held.
- `apt_packages_pinned_updates_count{category="security"}`
  Pinned updates whose newer available version is present in a non-backports security archive.
- `apt_packages_backports_updates_count`
  Packages with a newer available version in a backports archive, regardless of whether that version is the selected apt policy candidate.
- `apt_packages_broken_count`
  Number of broken packages inferred from dpkg status.
- `pkg_exporter_upgrade_metrics_valid`
  `1` when local apt package indexes are present; `0` when upgrade-related metrics should be treated as unknown because apt package indexes are missing.
- `os_reboot_required`
  `1` when the reboot-required marker file exists, otherwise `0`.

### Collection Health

- `pkg_exporter_last_collection_success`
  `1` when the most recent snapshot refresh succeeded, otherwise `0`.
- `pkg_exporter_last_collection_timestamp_seconds`
  Unix timestamp of the latest refresh attempt.
- `pkg_exporter_last_successful_collection_timestamp_seconds`
  Unix timestamp of the latest successful refresh, or `0` before the first success.
- `pkg_exporter_collection_duration_seconds`
  Framework-owned histogram of outer snapshot refresh durations.
- `pkg_exporter_package_collection_duration_seconds`
  Histogram of package-specific apt/dpkg collection durations.
- `pkg_exporter_collection_stage_duration_seconds{stage}`
  Histograms for internal collection stages.

## Grafana Dashboard

The example Grafana dashboard uses the v2 dashboard model and `TabsLayout`.

- `Overview`
  Contains exporter collection status, package status cards, a per-instance `Package Inventory` table, collapsed historical package graphs, apt cache freshness, and package collection internals.
  `Main Metrics` contains only the status cards and table; time-series package graphs live under the collapsed `Historical Graph` row.
- `Runtime`
  Contains the exporter runtime snapshot, Go/process runtime graphs, and framework collection panels.
- `Scrape`
  Contains Prometheus scrape health panels. Dashboard-level cursor synchronization is enabled so scrape graphs share crosshair movement.

The `Package Inventory` table joins by `instance` and shows only useful fields: exporter version, collection success, reboot state, installed/broken/update counts, last successful collection age, apt cache validity, and apt/dpkg source ages. Build metadata labels such as branch, goarch, goos, revision, tags, and Go version are hidden.

## Testing Strategy

The project uses:

- parser and service unit tests in `internal/apt`
- snapshot engine tests in `internal/pkgcheck`
- feature-backed collector tests in `internal/pkg`
- binary smoke tests in `smoke`

[`testdata/`](testdata) contains canned apt, dpkg, and reboot marker fixtures so the test suite does not depend on the host package manager state.
