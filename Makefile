include Makefile.mk

.PHONY: help build release release-archives release-checksums release-smoke docker-build docker-buildx docker-buildx-push docker-push fmt fmt-check vet staticcheck test test-race coverage coverage-check smoke promtool-check compose compose-up compose-down compose-logs compose-config examples-check docker-smoke-build docker-smoke-image docker-smoke go-check check full-check clean size
.SILENT: compose compose-config compose-down compose-logs compose-up size

help: ## Show available make targets.
	@printf "\033[33mUsage:\033[0m\n"
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "};{printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

fmt: ## Format Go files.
	$(GOFMT) -w $$($(GO) list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./...)

fmt-check: ## Check Go formatting.
	@test -z "$$($(GOFMT) -l $$($(GO) list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./... | tr '\n' ' '))"

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

vet: ## Run go vet.
	$(GO) vet ./...

staticcheck: ## Run staticcheck.
	GOFLAGS="$(strip $(GOFLAGS) $(STATICCHECK_GOFLAGS))" $(STATICCHECK) ./...

test: ## Run Go tests.
	$(GO) test -buildvcs=false -ldflags "$(LDFLAGS)" ./...

test-race: ## Run Go tests with the race detector.
	CGO_ENABLED=1 $(GO) test -buildvcs=false -race -ldflags "$(LDFLAGS)" ./...

coverage: ## Run tests with coverage and write coverage reports.
	$(GO) test -buildvcs=false -ldflags "$(LDFLAGS)" -covermode=atomic -coverprofile=$(COVERAGE_PROFILE) ./...
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

docker-build: ## Build the Docker image.
	$(DOCKER) build \
		--build-arg LDFLAGS="$(LDFLAGS)" \
		--build-arg PROJECT_NAME=$(PROJECT_NAME) \
		-t $(DOCKER_IMAGE) \
		.

docker-buildx: ## Build a multi-platform Docker image with buildx.
	$(DOCKER) buildx build \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg LDFLAGS="$(LDFLAGS)" \
		--build-arg PROJECT_NAME=$(PROJECT_NAME) \
		-t $(DOCKER_IMAGE) \
		.

docker-buildx-push: ## Build and push a multi-platform Docker image with buildx.
	$(DOCKER) buildx build \
		--push \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg LDFLAGS="$(LDFLAGS)" \
		--build-arg PROJECT_NAME=$(PROJECT_NAME) \
		-t $(DOCKER_IMAGE) \
		.

docker-push: ## Push the Docker image.
	$(DOCKER) push $(DOCKER_IMAGE)

promtool-check: ## Validate the Prometheus config and rules used by Docker Compose.
	$(MAKE) compose COMPOSE_ARGS="run --rm --no-deps --entrypoint promtool prometheus check config /etc/prometheus/prometheus.yml"

compose: ## Run Docker Compose with Makefile.mk variables. Override COMPOSE_ARGS as needed.
	COMPOSE_PROJECT_NAME="$(COMPOSE_PROJECT_NAME)" \
	PROJECT_NAME="$(PROJECT_NAME)" \
	PROJECT_DESC="$(PROJECT_DESC)" \
	FEATURE_NAME="$(FEATURE_NAME)" \
	FEATURE_CONFIG_FILE="$(FEATURE_CONFIG_FILE)" \
	FEATURE_CONFIG_PATH="$(FEATURE_CONFIG_PATH)" \
	FEATURE_CONFIG_CONTAINER_PATH="$(FEATURE_CONFIG_CONTAINER_PATH)" \
	COMPOSE_EXPORTER_PORT="$(COMPOSE_EXPORTER_PORT)" \
	LDFLAGS="$(LDFLAGS)" \
	$(DOCKER_COMPOSE) $(COMPOSE_ARGS)

compose-up: ## Start the Docker Compose example.
	$(MAKE) compose COMPOSE_ARGS="up --build"

compose-down: ## Stop the Docker Compose example.
	$(MAKE) compose COMPOSE_ARGS="down --remove-orphans"

compose-logs: ## Follow Docker Compose logs.
	$(MAKE) compose COMPOSE_ARGS="logs -f"

compose-config: ## Validate the Docker Compose example.
	$(MAKE) compose COMPOSE_ARGS="config >/dev/null"

examples-check: promtool-check compose-config ## Validate shipped Prometheus and Compose examples.

docker-smoke-build: ## Build the Docker image used by docker-smoke.
	$(MAKE) docker-build \
		DOCKER_IMAGE=$(SMOKE_DOCKER_IMAGE) \
		LDFLAGS="$(SMOKE_LDFLAGS)"

docker-smoke-image: ## Smoke-test an already built Docker image.
	@help_output="$$( $(DOCKER) run --rm $(DOCKER_IMAGE) --help 2>&1 )"; \
	echo "$$help_output" | grep -F "usage: $(DOCKER_ENTRYPOINT_NAME) [<flags>]" >/dev/null || { echo "$$help_output"; exit 1; }
	@version_output="$$( $(DOCKER) run --rm $(DOCKER_IMAGE) --version 2>&1 )"; \
	echo "$$version_output"; \
	echo "$$version_output" | grep -F "$(DOCKER_SMOKE_EXPECTED_VERSION)" >/dev/null; \
	echo "$$version_output" | grep -F "$(DOCKER_SMOKE_EXPECTED_BRANCH)" >/dev/null; \
	echo "$$version_output" | grep -F "$(DOCKER_SMOKE_EXPECTED_REVISION)" >/dev/null
	@cid="$$( $(DOCKER) run -d $(DOCKER_SMOKE_RUN_OPTIONS) $(DOCKER_IMAGE) --log.level=error --web.listen-address=:9900 --$(FEATURE_NAME).refresh-interval=100ms $(DOCKER_SMOKE_EXPORTER_ARGS) )"; \
	trap "$(DOCKER) rm -f $$cid >/dev/null 2>&1 || true" EXIT; \
	i=0; \
	while [ "$$i" -lt 60 ]; do \
		i=$$((i + 1)); \
		if $(DOCKER) run --rm --network container:$$cid $(DOCKER_HTTP_IMAGE) wget -qO- http://127.0.0.1:9900/healthz 2>/dev/null | grep -qx 'ok'; then \
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
		metrics="$$( $(DOCKER) run --rm --network container:$$cid $(DOCKER_HTTP_IMAGE) wget -qO- http://127.0.0.1:9900/metrics )"; \
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

go-check: fmt-check vet staticcheck coverage-check smoke test-race ## Run Go checks that do not require Docker.

check: go-check examples-check ## Run the standard maintenance check.

full-check: check docker-smoke release-smoke size ## Run all local checks, Docker smoke, and release smoke.

clean: ## Remove generated local artifacts.
	rm -rf $(DIST_DIR)
	rm -f $(COVERAGE_PROFILE) $(COVERAGE_REPORT)

size:
	@du -h dist/$(PROJECT_NAME)*
