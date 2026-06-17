package pkg

import (
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

type featureMetricState struct {
	mu                   sync.Mutex
	lastObservedAttempt  time.Time
	collectionCount      uint64
	collectionSum        float64
	stageCollectionCount map[string]uint64
	stageCollectionSum   map[string]float64
}

func NewFeatureMetricHandlers() featurekit.FeatureMetricHandlers[Snapshot] {
	state := &featureMetricState{}
	return featurekit.FeatureMetricHandlers[Snapshot]{
		Collect:  state.CollectFeatureMetrics,
		LogError: LogFeatureSnapshotError,
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

func LogFeatureSnapshotError(ctx featurekit.FeatureMetricsContext[Snapshot], logger *slog.Logger, snapshot Snapshot) {
	if snapshot.pkg.Err != nil {
		logger.Error(
			ctx.FeatureName+" package data collection failed",
			"err", snapshot.pkg.Err,
		)
	}
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
	for key, value := range state.stageCollectionCount {
		stageCounts[key] = value
	}
	stageSums := make(map[string]float64, len(state.stageCollectionSum))
	for key, value := range state.stageCollectionSum {
		stageSums[key] = value
	}
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
