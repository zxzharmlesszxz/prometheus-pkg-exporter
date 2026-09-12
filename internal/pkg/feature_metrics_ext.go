package pkg

import (
	"log/slog"
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/apt"
)

type featureMetricState struct {
	mu                   sync.Mutex
	lastObservedAttempt  time.Time
	lastLoggedErrAttempt time.Time
	hasLoggedError       bool
	collectionCount      uint64
	collectionSum        float64
	stageCollectionCount map[string]uint64
	stageCollectionSum   map[string]float64
}

func NewFeatureMetricHandlers() featurekit.FeatureMetricHandlers[Snapshot] {
	state := &featureMetricState{}
	return featurekit.FeatureMetricHandlers[Snapshot]{
		Collect:  state.CollectFeatureMetrics,
		LogError: state.LogFeatureSnapshotError,
	}
}

func (state *featureMetricState) CollectFeatureMetrics(ctx featurekit.FeatureMetricsContext[Snapshot], ch chan<- prometheus.Metric, snapshot Snapshot, _ time.Time) {
	stats := snapshot.pkg.Stats
	observePackageSnapshot(state, snapshot)
	collectionCount, collectionSum, stageCounts, stageSums := packageCollectionObservations(state)

	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricRebootRequired),
		prometheus.GaugeValue,
		framework.BoolFloat(snapshot.pkg.RebootRequired),
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricPackagesInstalled),
		prometheus.GaugeValue,
		float64(stats.Installed),
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricPackagesUpgradable),
		prometheus.GaugeValue,
		float64(stats.Upgradable),
		"all",
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricPackagesUpgradable),
		prometheus.GaugeValue,
		float64(stats.SecurityUpgradable),
		"security",
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricPackagesPinnedUpdates),
		prometheus.GaugeValue,
		float64(stats.PinnedUpdates),
		"all",
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricPackagesPinnedUpdates),
		prometheus.GaugeValue,
		float64(stats.SecurityPinnedUpdates),
		"security",
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricPackagesBackportsUpdates),
		prometheus.GaugeValue,
		float64(stats.BackportsUpdates),
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricPackagesBroken),
		prometheus.GaugeValue,
		float64(stats.Broken),
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricPackageIndexCacheHits),
		prometheus.CounterValue,
		float64(stats.PackageIndexCacheHits),
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricPackageIndexCacheMisses),
		prometheus.CounterValue,
		float64(stats.PackageIndexCacheMisses),
	)
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricUpgradeMetricsValid),
		prometheus.GaugeValue,
		framework.BoolFloat(stats.UpgradeMetricsValid),
	)
	emitSourceHealthMetrics(ctx, ch, stats)
	if !stats.DPKGStatusLastModified.IsZero() {
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricDPKGStatusLastModifiedTimestamp),
			prometheus.GaugeValue,
			framework.UnixTimestamp(stats.DPKGStatusLastModified),
		)
	}
	if !stats.APTListsLastModified.IsZero() {
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricAPTListsLastModifiedTimestamp),
			prometheus.GaugeValue,
			framework.UnixTimestamp(stats.APTListsLastModified),
		)
	}

	for _, compression := range sortedKeys(stats.APTListFilesCount) {
		ch <- prometheus.MustNewConstMetric(
			ctx.Descriptors.Get(metricAPTListsFilesCount),
			prometheus.GaugeValue,
			float64(stats.APTListFilesCount[compression]),
			compression,
		)
	}
	ch <- prometheus.MustNewConstHistogram(
		ctx.Descriptors.Get(metricPackageCollectionDuration),
		collectionCount,
		collectionSum,
		nil,
	)
	for _, stage := range sortedKeysUint64(stageCounts) {
		ch <- prometheus.MustNewConstHistogram(
			ctx.Descriptors.Get(metricCollectionStageDuration),
			stageCounts[stage],
			stageSums[stage],
			nil,
			stage,
		)
	}
}

func emitSourceHealthMetrics(ctx featurekit.FeatureMetricsContext[Snapshot], ch chan<- prometheus.Metric, stats apt.Stats) {
	pairs := []struct {
		metric string
		type_  prometheus.ValueType
		value  float64
	}{
		{metricStatusDatabaseUp, prometheus.GaugeValue, framework.BoolFloat(stats.StatusDatabaseSource.Up)},
		{metricStatusDatabaseValid, prometheus.GaugeValue, framework.BoolFloat(stats.StatusDatabaseSource.Valid)},
		{metricStatusDatabaseReadErrors, prometheus.CounterValue, float64(stats.StatusDatabaseSource.ReadErrorsTotal)},
		{metricStatusDatabaseParseErrors, prometheus.CounterValue, float64(stats.StatusDatabaseSource.ParseErrorsTotal)},
		{metricAPTListsUp, prometheus.GaugeValue, framework.BoolFloat(stats.APTListsSource.Up)},
		{metricAPTListsValid, prometheus.GaugeValue, framework.BoolFloat(stats.APTListsSource.Valid)},
		{metricAPTListsReadErrors, prometheus.CounterValue, float64(stats.APTListsSource.ReadErrorsTotal)},
		{metricAPTListsParseErrors, prometheus.CounterValue, float64(stats.APTListsSource.ParseErrorsTotal)},
	}
	for _, pair := range pairs {
		ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(pair.metric), pair.type_, pair.value)
	}
}

func (state *featureMetricState) LogFeatureSnapshotError(ctx featurekit.FeatureMetricsContext[Snapshot], logger *slog.Logger, snapshot Snapshot) {
	if snapshot.pkg.Err == nil || !state.markErrorAttemptLogged(snapshot.pkg.AttemptTime) {
		return
	}
	logger.Error(
		ctx.FeatureName+" package data collection failed",
		"err", snapshot.pkg.Err,
	)
}

func (state *featureMetricState) markErrorAttemptLogged(attempt time.Time) bool {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.hasLoggedError && attempt.Equal(state.lastLoggedErrAttempt) {
		return false
	}
	state.lastLoggedErrAttempt = attempt
	state.hasLoggedError = true
	return true
}

func observePackageSnapshot(state *featureMetricState, snapshot Snapshot) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if snapshot.pkg.AttemptTime.IsZero() || snapshot.pkg.AttemptTime.Equal(state.lastObservedAttempt) {
		return
	}
	state.lastObservedAttempt = snapshot.pkg.AttemptTime
	state.collectionCount++
	state.collectionSum += snapshot.pkg.CollectionDurationSeconds
	if state.stageCollectionCount == nil {
		state.stageCollectionCount = map[string]uint64{}
	}
	if state.stageCollectionSum == nil {
		state.stageCollectionSum = map[string]float64{}
	}
	for stage, duration := range snapshot.pkg.Stats.StageDurations {
		state.stageCollectionCount[stage]++
		state.stageCollectionSum[stage] += duration
	}
}

func packageCollectionObservations(state *featureMetricState) (uint64, float64, map[string]uint64, map[string]float64) {
	state.mu.Lock()
	defer state.mu.Unlock()

	stageCounts := make(map[string]uint64, len(state.stageCollectionCount))
	maps.Copy(stageCounts, state.stageCollectionCount)
	stageSums := make(map[string]float64, len(state.stageCollectionSum))
	maps.Copy(stageSums, state.stageCollectionSum)
	return state.collectionCount, state.collectionSum, stageCounts, stageSums
}

func sortedKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysUint64(values map[string]uint64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
