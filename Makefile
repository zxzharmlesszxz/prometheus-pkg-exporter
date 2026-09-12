include Makefile.mk
include Makefile.defaults.mk
-include Makefile.local

.PHONY: help build release release-archives release-checksums release-smoke release-preflight release-version-check release-worktree-check release-tag-check push-release docker-build docker-buildx docker-buildx-push docker-push fmt fmt-check print-ldflags vet staticcheck govulncheck golangci-lint test test-race coverage coverage-check smoke promtool-check promtool-rules-test compose compose-up compose-down compose-logs compose-config examples-check docker-smoke-build docker-smoke-image docker-smoke go-check check full-check clean size require-rendered-template
.SILENT: compose compose-config compose-down compose-logs compose-up size

help: ## Show available make targets.
	@printf "\033[33mUsage:\033[0m\n"
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "};{printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

fmt: ## Format Go files.
	@set -e; \
	files="$$($(GO) list -buildvcs=false -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./...)"; \
	$(GOFMT) -w $$files

fmt-check: ## Check Go formatting.
	@set -e; \
	files="$$($(GO) list -buildvcs=false -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./...)"; \
	test -z "$$($(GOFMT) -l $$files)"

print-ldflags: ## Print linker flags for external build integrations.
	@printf '%s\n' "$(strip $(LDFLAGS))"

build: ## Build the local release binary into dist/.
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_OUTPUT) $(MAIN_PACKAGE)
	$(MAKE) size

release-archives: ## Cross-build release archives into dist/.
	mkdir -p $(DIST_DIR)
	@set -e; \
	for platform in $(PLATFORMS); do \
		goos="$${platform%/*}"; \
		goarch="$${platform#*/}"; \
		archive="$(PROJECT_NAME)_$(VERSION)_$${goos}_$${goarch}"; \
		workdir="$(DIST_DIR)/$$archive"; \
		binary="$(PROJECT_NAME)"; \
		if [ "$$goos" = "windows" ]; then binary="$$binary.exe"; fi; \
		rm -rf "$$workdir"; \
		mkdir -p "$$workdir"; \
		echo "building $$archive"; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS="$$goos" GOARCH="$$goarch" $(GO) build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o "$$workdir/$$binary" $(MAIN_PACKAGE); \
		cp README.md METRICS.md LICENSE "$$workdir/"; \
		cp -R examples "$$workdir/examples"; \
		COPYFILE_DISABLE=1 tar -C $(DIST_DIR) -czf "$(DIST_DIR)/$$archive.tar.gz" "$$archive"; \
		rm -rf "$$workdir"; \
	done

release-checksums: release-archives ## Write SHA256 checksums for release archives.
	@set -e; \
	cd $(DIST_DIR); \
	if command -v sha256sum >/dev/null 2>&1; then \
		sha256sum *.tar.gz > checksums.txt; \
	else \
		shasum -a 256 *.tar.gz > checksums.txt; \
	fi; \
	cat checksums.txt

release: clean release-checksums ## Build release archives and checksums.

release-smoke: release ## Build release archives and smoke-test the native archive.
	@set -e; \
	goos="$$( $(GO) env GOOS )"; \
	goarch="$$( $(GO) env GOARCH )"; \
	archive="$(DIST_DIR)/$(PROJECT_NAME)_$(VERSION)_$${goos}_$${goarch}.tar.gz"; \
	if [ ! -f "$$archive" ]; then \
		echo "skipping release smoke: native archive $$archive was not built"; \
		exit 0; \
	fi; \
	tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp"' EXIT; \
	COPYFILE_DISABLE=1 tar -C "$$tmp" -xzf "$$archive"; \
	binary="$$tmp/$(PROJECT_NAME)_$(VERSION)_$${goos}_$${goarch}/$(PROJECT_NAME)"; \
	if [ "$$goos" = "windows" ]; then binary="$$binary.exe"; fi; \
	"$$binary" --help 2>&1 | grep -F "usage: $(PROJECT_NAME) [<flags>]" >/dev/null; \
	"$$binary" --version 2>&1 | grep -F "$(VERSION)" >/dev/null

release-preflight: release-version-check full-check ## Run all checks required before pushing a release tag. Set VERSION=vX.Y.Z.

release-version-check: ## Validate VERSION for release targets.
	@test -n "$(VERSION)" || { echo "VERSION is required, for example VERSION=v0.1.0" >&2; exit 2; }
	@case "$(VERSION)" in \
		v0.[0-9]*.[0-9]*|v1.[0-9]*.[0-9]*) ;; \
		*) echo "VERSION must look like v0.1.0 or v1.2.3; v2+ requires a /v2 module path" >&2; exit 2 ;; \
	esac

release-worktree-check: ## Verify the worktree is clean before release.
	@test -z "$$(git status --porcelain)" || { echo "worktree has uncommitted changes; commit or stash before release" >&2; exit 2; }

release-tag-check: release-version-check ## Verify release tag does not already exist locally or remotely.
	@if git rev-parse -q --verify "refs/tags/$(VERSION)" >/dev/null; then \
		echo "local tag already exists: $(VERSION)" >&2; \
		exit 2; \
	fi
	@if git ls-remote --exit-code --tags origin "refs/tags/$(VERSION)" >/dev/null 2>&1; then \
		echo "remote tag already exists: $(VERSION)" >&2; \
		exit 2; \
	fi

push-release: release-worktree-check release-tag-check release-preflight ## Run release preflight, tag HEAD, and push the release tag. Set VERSION=vX.Y.Z.
	@current_branch="$$(git branch --show-current)"; \
	if [ "$$current_branch" != "main" ]; then \
		echo "push-release must run from main, got $$current_branch" >&2; \
		exit 2; \
	fi
	git push origin HEAD:main
	git tag -a "$(VERSION)" -m "Release $(VERSION)"
	git push origin "$(VERSION)"

vet: ## Run go vet.
	$(GO) vet -buildvcs=false ./...

staticcheck: ## Run staticcheck.
	GOFLAGS="$(strip $(GOFLAGS) $(STATICCHECK_GOFLAGS))" $(STATICCHECK) ./...

govulncheck: ## Run Go vulnerability analysis.
	$(GOVULNCHECK) ./...

golangci-lint: ## Run golangci-lint.
	$(GOLANGCI_LINT) run ./...

test: ## Run Go tests.
	$(GO) test -buildvcs=false -ldflags "$(LDFLAGS)" -count=1 ./...

test-race: ## Run Go tests with the race detector.
	@packages="$$($(GO) list -buildvcs=false -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./...)"; \
	if [ -z "$$packages" ]; then \
		echo "no test packages"; \
		exit 0; \
	fi; \
	CGO_ENABLED=1 $(GO) test -buildvcs=false -race -ldflags "$(LDFLAGS)" -count=1 $$packages

coverage: ## Run tests with coverage and write coverage reports.
	$(GO) test -buildvcs=false -ldflags "$(LDFLAGS)" -covermode=atomic -coverprofile=$(COVERAGE_PROFILE) -count=1 ./...
	$(GO) tool cover -func=$(COVERAGE_PROFILE) | tee $(COVERAGE_REPORT)

coverage-check: coverage ## Enforce the coverage threshold.
	@coverage="$$(awk '/^total:/ {gsub(/%/, "", $$3); print $$3}' $(COVERAGE_REPORT))"; \
	awk -v coverage="$$coverage" -v threshold="$(COVERAGE_THRESHOLD)" 'BEGIN { \
		if (coverage + 0 < threshold + 0) { \
			printf "coverage %.1f%% is below %.1f%%\n", coverage, threshold; \
			exit 1; \
		} \
		printf "coverage %.1f%% meets threshold %.1f%%\n", coverage, threshold; \
	}'

smoke: ## Build and smoke-test the local binary.
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -buildvcs=false -trimpath -ldflags "$(SMOKE_LDFLAGS)" -o $(SMOKE_BINARY) $(MAIN_PACKAGE)
	RUN_BINARY_SMOKE=1 GO="$(GO)" EXPORTER_SMOKE_BINARY="$(SMOKE_BINARY)" $(GO) test -buildvcs=false -ldflags "$(SMOKE_LDFLAGS)" ./smoke -run TestBinarySmoke -count=1

require-rendered-template:
	@if [ "$(SCAFFOLD_RENDERED)" != "true" ]; then \
		echo "scaffold template must be rendered before running build or compose targets" >&2; \
		exit 2; \
	fi

define docker_build_args
--build-arg VERSION="$(VERSION)"
--build-arg BRANCH="$(BRANCH)"
--build-arg REVISION="$(REVISION)"
--build-arg BUILD_USER="$(BUILD_USER)"
--build-arg BUILD_DATE="$(BUILD_DATE)"
-t $(DOCKER_IMAGE)
endef

docker-build: require-rendered-template ## Build the Docker image.
	$(DOCKER) build \
		$(strip $(docker_build_args)) \
		.

docker-buildx: require-rendered-template ## Build a multi-platform Docker image with buildx.
	$(DOCKER) buildx build \
		--platform $(DOCKER_PLATFORMS) \
		$(strip $(docker_build_args)) \
		.

docker-buildx-push: require-rendered-template ## Build and push a multi-platform Docker image with buildx.
	$(DOCKER) buildx build \
		--push \
		--platform $(DOCKER_PLATFORMS) \
		$(strip $(docker_build_args)) \
		.

docker-push: ## Push the Docker image.
	$(DOCKER) push $(DOCKER_IMAGE)

promtool-check: ## Validate the Prometheus config and rules used by Docker Compose.
	$(MAKE) compose COMPOSE_ARGS="run --rm --no-deps --entrypoint promtool prometheus check config /etc/prometheus/prometheus.yml"

promtool-rules-test: ## Test bundled Prometheus alert rules when present.
	@tests="$$(find examples/prometheus/tests -maxdepth 1 -name '*.test.yml' -exec basename {} \; 2>/dev/null | sort)"; \
	if [ -z "$$tests" ]; then \
		echo "no Prometheus rule tests found"; \
		exit 0; \
	fi; \
	$(MAKE) compose COMPOSE_ARGS="run --rm --no-deps --workdir /etc/prometheus/rules/tests --entrypoint promtool prometheus test rules $$tests"

compose: require-rendered-template ## Run Docker Compose with Makefile.mk variables. Override COMPOSE_ARGS as needed.
	COMPOSE_PROJECT_NAME="$(COMPOSE_PROJECT_NAME)" \
	VERSION="$(VERSION)" \
	BRANCH="$(BRANCH)" \
	REVISION="$(REVISION)" \
	BUILD_USER="$(BUILD_USER)" \
	BUILD_DATE="$(BUILD_DATE)" \
	PROJECT_NAME="$(PROJECT_NAME)" \
	PROJECT_DESC="$(PROJECT_DESC)" \
	FEATURE_NAME="$(FEATURE_NAME)" \
	FEATURE_CONFIG_FILE="$(FEATURE_CONFIG_FILE)" \
	FEATURE_CONFIG_PATH="$(FEATURE_CONFIG_PATH)" \
	FEATURE_CONFIG_CONTAINER_PATH="$(FEATURE_CONFIG_CONTAINER_PATH)" \
	COMPOSE_EXPORTER_HOST_PORT="$(COMPOSE_EXPORTER_HOST_PORT)" \
	PROMETHEUS_IMAGE="$(PROMETHEUS_IMAGE)" \
	GRAFANA_IMAGE="$(GRAFANA_IMAGE)" \
	GRAFANA_ADMIN_USER="$(GRAFANA_ADMIN_USER)" \
	GRAFANA_ADMIN_PASSWORD="$(GRAFANA_ADMIN_PASSWORD)" \
	$(DOCKER_COMPOSE) $(COMPOSE_ARGS)

compose-up: ## Start the Docker Compose example.
	$(MAKE) compose COMPOSE_ARGS="up --build"

compose-down: ## Stop the Docker Compose example.
	$(MAKE) compose COMPOSE_ARGS="down --remove-orphans"

compose-logs: ## Follow Docker Compose logs.
	$(MAKE) compose COMPOSE_ARGS="logs -f"

compose-config: ## Validate the Docker Compose example.
	$(MAKE) compose COMPOSE_ARGS="config >/dev/null"

examples-check: promtool-check promtool-rules-test compose-config ## Validate shipped Prometheus and Compose examples.

docker-smoke-build: require-rendered-template ## Build the Docker image used by docker-smoke.
	$(MAKE) docker-build \
		DOCKER_IMAGE=$(SMOKE_DOCKER_IMAGE) \
		VERSION="$(SMOKE_VERSION)" \
		BRANCH="$(SMOKE_BRANCH)" \
		REVISION="$(SMOKE_REVISION)" \
		BUILD_USER="$(SMOKE_BUILD_USER)" \
		BUILD_DATE="$(SMOKE_BUILD_DATE)"

docker-smoke-image: ## Smoke-test an already built Docker image.
	@help_output="$$( $(DOCKER) run --rm $(DOCKER_IMAGE) --help 2>&1 )"; \
	echo "$$help_output" | grep -F "usage: $(DOCKER_ENTRYPOINT_NAME) [<flags>]" >/dev/null || { echo "$$help_output"; exit 1; }
	@version_output="$$( $(DOCKER) run --rm $(DOCKER_IMAGE) --version 2>&1 )"; \
	echo "$$version_output"; \
	echo "$$version_output" | grep -F "$(DOCKER_SMOKE_EXPECTED_VERSION)" >/dev/null; \
	echo "$$version_output" | grep -F "$(DOCKER_SMOKE_EXPECTED_BRANCH)" >/dev/null; \
	echo "$$version_output" | grep -F "$(DOCKER_SMOKE_EXPECTED_REVISION)" >/dev/null
	@cid="$$( $(DOCKER) run -d $(DOCKER_SMOKE_RUN_OPTIONS) $(DOCKER_IMAGE) --log.level=error --web.listen-address=:$(DOCKER_SMOKE_PORT) --$(FEATURE_NAME).refresh-interval=100ms $(DOCKER_SMOKE_EXPORTER_ARGS) )"; \
	trap "$(DOCKER) rm -f $$cid >/dev/null 2>&1 || true" EXIT; \
	i=0; \
	while [ "$$i" -lt 60 ]; do \
		i=$$((i + 1)); \
		if $(DOCKER) run --rm --network container:$$cid $(DOCKER_HTTP_IMAGE) wget -qO- http://127.0.0.1:$(DOCKER_SMOKE_PORT)/healthz 2>/dev/null | grep -qx 'ok'; then \
			break; \
		fi; \
		if [ "$$i" -eq 60 ]; then \
			$(DOCKER) logs "$$cid"; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	metrics=""; \
	i=0; \
	while [ "$$i" -lt 60 ]; do \
		i=$$((i + 1)); \
		metrics="$$( $(DOCKER) run --rm --network container:$$cid $(DOCKER_HTTP_IMAGE) wget -qO- http://127.0.0.1:$(DOCKER_SMOKE_PORT)/metrics )"; \
		if echo "$$metrics" | grep -F '$(METRIC_NAMESPACE)_last_collection_success 1' >/dev/null; then \
			break; \
		fi; \
		if [ "$$i" -eq 60 ]; then \
			$(DOCKER) logs "$$cid"; \
			echo "$$metrics"; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	echo "$$metrics" | grep -F "$(METRIC_NAMESPACE)_build_info" >/dev/null; \
	echo "$$metrics" | grep -F 'version="$(DOCKER_SMOKE_EXPECTED_VERSION)"' >/dev/null; \
	echo "$$metrics" | grep -F 'branch="$(DOCKER_SMOKE_EXPECTED_BRANCH)"' >/dev/null; \
	echo "$$metrics" | grep -F 'revision="$(DOCKER_SMOKE_EXPECTED_REVISION)"' >/dev/null; \
	echo "$$metrics" | grep -F -- '$(DOCKER_SMOKE_METRIC)' >/dev/null; \
	extra_metrics='$(DOCKER_SMOKE_EXTRA_METRICS)'; \
	if [ -n "$$extra_metrics" ]; then \
		old_ifs="$$IFS"; \
		IFS='|'; \
		for metric in $$extra_metrics; do \
			IFS="$$old_ifs"; \
			echo "$$metrics" | grep -F -- "$$metric" >/dev/null; \
			IFS='|'; \
		done; \
		IFS="$$old_ifs"; \
	fi; \
	$(DOCKER) rm -f "$$cid" >/dev/null 2>&1 || true

docker-smoke: docker-smoke-build ## Build and smoke-test the Docker image.
	$(MAKE) docker-smoke-image \
		DOCKER_IMAGE=$(SMOKE_DOCKER_IMAGE) \
		DOCKER_SMOKE_EXPECTED_VERSION="$(SMOKE_VERSION)" \
		DOCKER_SMOKE_EXPECTED_BRANCH="$(SMOKE_BRANCH)" \
		DOCKER_SMOKE_EXPECTED_REVISION="$(SMOKE_REVISION)"

go-check: fmt-check vet staticcheck golangci-lint govulncheck coverage-check smoke test-race ## Run Go checks that do not require Docker.

check: go-check examples-check ## Run the standard maintenance check.

full-check: check docker-smoke release-smoke size ## Run all local checks, Docker smoke, and release smoke.

clean: ## Remove generated local artifacts.
	rm -rf $(DIST_DIR)
	rm -f $(COVERAGE_PROFILE) $(COVERAGE_REPORT)

size:
	@targets="$$(find $(DIST_DIR) -maxdepth 1 \( -name '$(PROJECT_NAME)*' -o -name '$(notdir $(BUILD_OUTPUT))' \) -print 2>/dev/null | sort)"; \
	if [ -n "$$targets" ]; then \
		du -h $$targets; \
	fi
