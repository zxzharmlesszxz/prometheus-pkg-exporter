# prometheus-pkg-exporter

`prometheus-pkg-exporter` exposes local Debian package state as Prometheus metrics.

It is built as a thin exporter on top of `prometheus-exporter-framework`.

## Inputs

By default, the exporter reads host-local package state from:

- `/var/lib/dpkg/status`
- `/var/lib/apt/lists/*_Packages*`
- `/var/lib/apt/lists/*_InRelease` and `/var/lib/apt/lists/*_Release`
- `/run/reboot-required`

The exporter is intended for Debian-family systems where Prometheus needs to scrape package health directly from the node.
It does not call external APIs.

Useful flags:

```bash
--pkg.config-file
--pkg.refresh-interval
--pkg.command-timeout
--pkg.reboot-required-path
--pkg.status-database-path
--pkg.apt-lists-path
--web.listen-address
--web.telemetry-path
--web.enable-pprof
--log.level
--log.format
```

By default, the exporter listens on `:9888`, refreshes package data every `1m`, and uses a `15s` timeout for each local package data read.
If `/etc/prometheus/prometheus-pkg-exporter.yml` exists, it is loaded as the package config file; if it is missing, defaults and flags are used.
Package refresh uses the template `SnapshotCollector` background worker; scrapes return the last collected snapshot.

## Configuration Example

The YAML config file accepts these package-specific keys:

```yaml
command_timeout: 15s
reboot_required_path: /run/reboot-required
status_database_path: /var/lib/dpkg/status
apt_lists_path: /var/lib/apt/lists
```

When the exporter runs in a container, mount the host package state and point the
paths at the mounted tree:

```yaml
command_timeout: 15s
reboot_required_path: /host/run/reboot-required
status_database_path: /host/var/lib/dpkg/status
apt_lists_path: /host/var/lib/apt/lists
```

## Local Run

Run against local package state:

```bash
make build
./dist/prometheus-pkg-exporter \
  --web.listen-address=:9888
```

Run against fixture or mounted paths:

```bash
./dist/prometheus-pkg-exporter \
  --pkg.status-database-path=testdata/dpkg-status \
  --pkg.apt-lists-path=testdata/apt-lists \
  --pkg.reboot-required-path=testdata/reboot-required
```

`pprof` is disabled by default. Enable it explicitly only when you need runtime profiling:

```bash
./dist/prometheus-pkg-exporter \
  --web.listen-address=:9888 \
  --web.enable-pprof
```

## Metrics

The exporter exposes package inventory counts, upgrade availability, broken package state, reboot requirement state, and collection health.
Upgrade availability is computed from local apt metadata and honors release priority markers such as `NotAutomatic` and `ButAutomaticUpgrades`, so backports are not counted as normal upgrades unless apt policy would make them candidates.
Compressed apt package indexes are supported for plain, gzip, bzip2, lz4, and xz apt list files.
Framework collection timing is exported as `pkg_exporter_collection_duration_seconds`; package-specific apt/dpkg collection timing is exported separately as `pkg_exporter_package_collection_duration_seconds`.

Example output:

```code
apt_packages_installed_count 4
apt_packages_upgradable_count{category="all"} 3
apt_packages_upgradable_count{category="security"} 2
apt_packages_pinned_updates_count{category="all"} 1
apt_packages_pinned_updates_count{category="security"} 0
apt_packages_backports_updates_count 1
apt_packages_broken_count 0
pkg_exporter_upgrade_metrics_valid 1
os_reboot_required 1
pkg_exporter_last_collection_success 1
pkg_exporter_last_collection_timestamp_seconds 1742812800
pkg_exporter_last_successful_collection_timestamp_seconds 1742812800
```

The full metric contract lives in [`METRICS.md`](METRICS.md).

## Grafana

An example dashboard is available at [`examples/grafana/prometheus-pkg-exporter.json`](examples/grafana/prometheus-pkg-exporter.json).
It uses the Grafana v2 dashboard model with `Overview`, `Runtime`, and `Scrape` tabs.
`Overview` contains package status cards, a per-instance `Package Inventory` table, collapsed historical graphs, apt cache freshness, and package collection internals.
`Runtime` contains exporter process/runtime panels and framework collection metrics.
`Scrape` contains Prometheus scrape health panels with synchronized crosshair movement.

## Docker Compose

The repository includes [`docker-compose.yml`](docker-compose.yml) for local testing.
It starts:

- `exporter`
- `prometheus`
- `grafana`

```bash
make compose
```

Endpoints:

- `http://localhost:9888`
- `http://localhost:9888/metrics`
- `http://localhost:9888/healthz`
- `http://localhost:9090`
- `http://localhost:3000`

Grafana credentials:

- username: `admin`
- password: `admin`

For a direct Docker build, run:

```bash
make docker-build
```

The image reads package state from the container filesystem by default.
For host monitoring, mount the host package database and apt lists into the container and point the path flags at those mounts.

## Tests

```bash
make go-check
```

The repository also includes a `Makefile` based on the scaffold maintenance targets and extended for this concrete exporter:

```bash
make help
make go-check
make check
make docker-smoke
make full-check
```

`make go-check` runs Go-only checks. `make check` also validates the Prometheus and Docker Compose examples, so it requires Docker.

## Scaffold-Owned Go Files

Go files named `scaffold_*.go` are generated contract glue and should stay identical to the scaffold output. Add package-specific behavior in adjacent non-scaffold files such as `internal/pkg/*_ext.go`, `internal/pkgcheck`, and `internal/apt`.

Build local release artifacts:

```bash
make build VERSION=v0.1.0
make release VERSION=v0.1.0
make release-smoke VERSION=v0.1.0
```

This writes binaries, `.tar.gz` release archives, and `checksums.txt` under `dist/`.
By default, `VERSION`, `BRANCH`, and `REVISION` are derived from Git metadata and fall back to `dev` outside a Git checkout.

Build and push a Docker image:

```bash
make docker-build VERSION=v0.1.0 DOCKER_IMAGE=prometheus-pkg-exporter:v0.1.0
make docker-push DOCKER_IMAGE=prometheus-pkg-exporter:v0.1.0
make docker-buildx-push VERSION=v0.1.0 DOCKER_IMAGE=registry.example.com/prometheus-pkg-exporter:v0.1.0
```

The GitLab CI file and GitHub Actions workflow both delegate to the same Makefile targets.
Tags build release archives and can publish a multi-platform Docker image to the GitLab registry or GitHub Container Registry.

Docker image publishing policy:

- branch and pull request pipelines build and smoke-test images but do not push them
- tag pipelines publish only the matching release tag, for example `v0.1.0`
- images are published as multi-architecture `linux/amd64` and `linux/arm64`
- `latest` and commit SHA tags are not published by default
- binary release archives remain the primary non-container distribution

## Architecture

The high-level design is documented in [`ARCHITECTURE.md`](ARCHITECTURE.md).

## Alert Rules

Example Prometheus alert rules live in [`examples/prometheus/prometheus-pkg-exporter.yml`](examples/prometheus/prometheus-pkg-exporter.yml).
