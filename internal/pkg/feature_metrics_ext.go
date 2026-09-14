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
	mu                     sync.Mutex
	lastObservedAttempt    time.Time
	lastLoggedErrAttempt   time.Time
	hasLoggedError         bool
	collectionCount        uint64
	collectionSum          float64
	collectionBuckets      map[float64]uint64
	stageCollectionCount   map[string]uint64
	stageCollectionSum     map[string]float64
	stageCollectionBuckets map[string]map[float64]uint64
}

var collectionDurationBuckets = []float64{0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 30, 60, 120}

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
	collectionCount, collectionSum, collectionBuckets, stageCounts, stageSums, stageBuckets := packageCollectionObservations(state)

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
		collectionBuckets,
	)
	for _, stage := range sortedKeysUint64(stageCounts) {
		ch <- prometheus.MustNewConstHistogram(
			ctx.Descriptors.Get(metricCollectionStageDuration),
			stageCounts[stage],
			stageSums[stage],
			stageBuckets[stage],
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
	if state.collectionBuckets == nil {
		state.collectionBuckets = newDurationBuckets()
	}
	observeDuration(state.collectionBuckets, snapshot.pkg.CollectionDurationSeconds)
	if state.stageCollectionCount == nil {
		state.stageCollectionCount = map[string]uint64{}
	}
	if state.stageCollectionSum == nil {
		state.stageCollectionSum = map[string]float64{}
	}
	if state.stageCollectionBuckets == nil {
		state.stageCollectionBuckets = map[string]map[float64]uint64{}
	}
	for stage, duration := range snapshot.pkg.Stats.StageDurations {
		state.stageCollectionCount[stage]++
		state.stageCollectionSum[stage] += duration
		if state.stageCollectionBuckets[stage] == nil {
			state.stageCollectionBuckets[stage] = newDurationBuckets()
		}
		observeDuration(state.stageCollectionBuckets[stage], duration)
	}
}

func packageCollectionObservations(state *featureMetricState) (uint64, float64, map[float64]uint64, map[string]uint64, map[string]float64, map[string]map[float64]uint64) {
	state.mu.Lock()
	defer state.mu.Unlock()

	collectionBuckets := cloneDurationBuckets(state.collectionBuckets)
	stageCounts := make(map[string]uint64, len(state.stageCollectionCount))
	maps.Copy(stageCounts, state.stageCollectionCount)
	stageSums := make(map[string]float64, len(state.stageCollectionSum))
	maps.Copy(stageSums, state.stageCollectionSum)
	stageBuckets := make(map[string]map[float64]uint64, len(state.stageCollectionBuckets))
	for stage, buckets := range state.stageCollectionBuckets {
		stageBuckets[stage] = cloneDurationBuckets(buckets)
	}
	return state.collectionCount, state.collectionSum, collectionBuckets, stageCounts, stageSums, stageBuckets
}

func newDurationBuckets() map[float64]uint64 {
	buckets := make(map[float64]uint64, len(collectionDurationBuckets))
	for _, upperBound := range collectionDurationBuckets {
		buckets[upperBound] = 0
	}
	return buckets
}

func observeDuration(buckets map[float64]uint64, duration float64) {
	for _, upperBound := range collectionDurationBuckets {
		if duration <= upperBound {
			buckets[upperBound]++
		}
	}
}

func cloneDurationBuckets(source map[float64]uint64) map[float64]uint64 {
	clone := make(map[float64]uint64, len(source))
	maps.Copy(clone, source)
	return clone
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
