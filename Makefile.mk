# Rendered exporter contract. Keep this file scaffold-owned.
override SCAFFOLD_RENDERED := true
override GO_MODULE := github.com/zxzharmlesszxz/prometheus-pkg-exporter
override FRAMEWORK_MODULE := github.com/zxzharmlesszxz/prometheus-exporter-framework
override COVERAGE_PROFILE := coverage.out
override COVERAGE_REPORT := coverage.txt
override PROJECT_NAME := prometheus-pkg-exporter
override PROJECT_DESC := Prometheus Pkg Exporter
override FEATURE_NAME := pkg
override FEATURE_NAMESPACE := pkg
override METRIC_NAMESPACE := pkg_exporter
override DEFAULT_PORT := :9888
override FEATURE_CONFIG_FILE := prometheus-pkg-exporter.yml
override FEATURE_CONFIG_PATH := examples/$(FEATURE_CONFIG_FILE)
override FEATURE_CONFIG_CONTAINER_PATH := /etc/prometheus/$(FEATURE_CONFIG_FILE)
override MAIN_PACKAGE := ./cmd
override DIST_DIR := dist
define exporter_ldflags
-s -w \
 -X 'github.com/prometheus/common/version.Version=$(1)' \
 -X 'github.com/prometheus/common/version.Branch=$(2)' \
 -X 'github.com/prometheus/common/version.Revision=$(3)' \
 -X 'github.com/prometheus/common/version.BuildUser=$(4)' \
 -X 'github.com/prometheus/common/version.BuildDate=$(5)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedExporterName=$(PROJECT_NAME)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedExporterDescription=$(PROJECT_DESC)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedFeatureName=$(FEATURE_NAME)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedMetricNamespace=$(METRIC_NAMESPACE)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedListenAddress=$(DEFAULT_PORT)' \
 -X '$(GO_MODULE)/internal/$(FEATURE_NAME).DefaultFeatureConfigFileName=$(FEATURE_CONFIG_FILE)'
endef
override LDFLAGS = $(call exporter_ldflags,$(VERSION),$(BRANCH),$(REVISION),$(BUILD_USER),$(BUILD_DATE))
override DOCKER_PROJECT_NAME := $(PROJECT_NAME)
override DOCKER_ENTRYPOINT_NAME := $(PROJECT_NAME)
override DOCKER_SMOKE_METRIC := $(METRIC_NAMESPACE)_upgrade_metrics_valid
override DOCKER_SMOKE_RUN_OPTIONS := -v "$(CURDIR)/$(FEATURE_CONFIG_PATH):$(FEATURE_CONFIG_CONTAINER_PATH):ro"
override DOCKER_SMOKE_EXPORTER_ARGS := --$(FEATURE_NAME).config-file=$(FEATURE_CONFIG_CONTAINER_PATH)
override DOCKER_SMOKE_EXTRA_METRICS :=
override DOCKER_SMOKE_PORT := 9900
override SMOKE_LDFLAGS = $(call exporter_ldflags,$(SMOKE_VERSION),$(SMOKE_BRANCH),$(SMOKE_REVISION),$(SMOKE_BUILD_USER),$(SMOKE_BUILD_DATE))
