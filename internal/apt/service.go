package apt

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Stats struct {
	Installed               int
	Upgradable              int
	SecurityUpgradable      int
	PinnedUpdates           int
	SecurityPinnedUpdates   int
	BackportsUpdates        int
	Broken                  int
	UpgradeMetricsValid     bool
	PackageIndexCacheHits   uint64
	PackageIndexCacheMisses uint64
	DPKGStatusLastModified  time.Time
	APTListsLastModified    time.Time
	APTListFilesCount       map[string]int
	StageDurations          map[string]float64
}

type Collector struct {
	source  Source
	timeout time.Duration

	mu                        sync.Mutex
	cachedAvailable           []AvailablePackage
	cachedUpgradeMetricsValid bool
	cachedPackageRevision     string
	cachedInstalledRevision   string
	cacheHits                 uint64
	cacheMisses               uint64
}

func NewCollector(source Source, timeout time.Duration) *Collector {
	if source == nil {
		source = CommandSource{}
	}
	return &Collector{
		source:  source,
		timeout: timeout,
	}
}

func Collect(ctx context.Context, source Source, timeout time.Duration) (Stats, error) {
	return NewCollector(source, timeout).Collect(ctx)
}

func (c *Collector) Collect(ctx context.Context) (Stats, error) {
	stageDurations := make(map[string]float64)
	stats := Stats{StageDurations: stageDurations}
	var errs []error

	var installed []InstalledPackage
	statusLoaded := false
	statusReadStart := time.Now()
	if out, err := readWithTimeout[[]byte](ctx, c.timeout, c.source.StatusDatabase); err != nil {
		stageDurations["status_read"] = time.Since(statusReadStart).Seconds()
		errs = append(errs, fmt.Errorf("status database: %w", err))
	} else {
		stageDurations["status_read"] = time.Since(statusReadStart).Seconds()
		statusParseStart := time.Now()
		installed = ParseInstalledPackages(string(out))
		stageDurations["status_parse"] = time.Since(statusParseStart).Seconds()
		statusLoaded = true
		stats.Installed = countInstalledPackages(installed)
		stats.Broken = countBrokenPackages(installed)
	}

	var available []AvailablePackage
	indexesLoaded := false
	if statusLoaded {
		indexes, upgradeMetricsValid, err := c.availablePackages(ctx, installed, stageDurations)
		if err != nil {
			errs = append(errs, fmt.Errorf("package indexes: %w", err))
		} else {
			available = indexes
			stats.UpgradeMetricsValid = upgradeMetricsValid
			indexesLoaded = true
		}
	} else if _, err := readWithTimeout[[]PackageIndex](ctx, c.timeout, c.source.PackageIndexes); err != nil {
		errs = append(errs, fmt.Errorf("package indexes: %w", err))
	}

	if statusLoaded && indexesLoaded {
		countStart := time.Now()
		availableByKey := availableByPackageKey(available)
		stats.Upgradable = countUpgradableByKey(installed, availableByKey, false)
		stats.SecurityUpgradable = countUpgradableByKey(installed, availableByKey, true)
		stats.PinnedUpdates = countPinnedUpdatesByKey(installed, availableByKey, false)
		stats.SecurityPinnedUpdates = countPinnedUpdatesByKey(installed, availableByKey, true)
		stats.BackportsUpdates = countBackportsUpdatesByKey(installed, availableByKey)
		stageDurations["count"] = time.Since(countStart).Seconds()
	}

	if metadata, err := c.sourceMetadata(ctx); err != nil {
		errs = append(errs, fmt.Errorf("source metadata: %w", err))
	} else if metadata != nil {
		stats.DPKGStatusLastModified = metadata.DPKGStatusLastModified
		stats.APTListsLastModified = metadata.APTListsLastModified
		stats.APTListFilesCount = metadata.APTListFilesByCompression
		stats.UpgradeMetricsValid = aptListFilesTotal(metadata.APTListFilesByCompression) > 0
	}

	stats.PackageIndexCacheHits, stats.PackageIndexCacheMisses = c.packageIndexCacheStats()
	return stats, errors.Join(errs...)
}

func (c *Collector) availablePackages(ctx context.Context, installed []InstalledPackage, stageDurations map[string]float64) ([]AvailablePackage, bool, error) {
	installedRevision := installedPackageRevision(installed)
	packageRevision, cacheable, err := c.packageIndexRevision(ctx, stageDurations)
	if err != nil {
		return nil, false, err
	}

	if cacheable {
		c.mu.Lock()
		if c.cachedAvailable != nil &&
			c.cachedPackageRevision == packageRevision &&
			c.cachedInstalledRevision == installedRevision {
			available := c.cachedAvailable
			upgradeMetricsValid := c.cachedUpgradeMetricsValid
			c.cacheHits++
			c.mu.Unlock()
			return available, upgradeMetricsValid, nil
		}
		c.cacheMisses++
		c.mu.Unlock()
	} else {
		c.mu.Lock()
		c.cacheMisses++
		c.mu.Unlock()
	}

	available, upgradeMetricsValid, err := c.readAvailablePackages(ctx, installed, stageDurations)
	if err != nil {
		return nil, false, err
	}

	if cacheable {
		c.mu.Lock()
		c.cachedAvailable = available
		c.cachedUpgradeMetricsValid = upgradeMetricsValid
		c.cachedPackageRevision = packageRevision
		c.cachedInstalledRevision = installedRevision
		c.mu.Unlock()
	}
	return available, upgradeMetricsValid, nil
}

func (c *Collector) readAvailablePackages(ctx context.Context, installed []InstalledPackage, stageDurations map[string]float64) ([]AvailablePackage, bool, error) {
	if streamer, ok := c.source.(PackageIndexStreamer); ok {
		indexReadStart := time.Now()
		streams, err := readWithTimeout[[]PackageIndexStream](ctx, c.timeout, streamer.PackageIndexStreams)
		stageDurations["index_read"] = time.Since(indexReadStart).Seconds()
		if err != nil {
			return nil, false, err
		}
		indexParseStart := time.Now()
		available, err := ParseAvailablePackagesFromStreams(streams, installed)
		stageDurations["index_parse"] = time.Since(indexParseStart).Seconds()
		if err != nil {
			return nil, false, err
		}
		return available, len(streams) > 0, nil
	}

	indexReadStart := time.Now()
	indexes, err := readWithTimeout[[]PackageIndex](ctx, c.timeout, c.source.PackageIndexes)
	stageDurations["index_read"] = time.Since(indexReadStart).Seconds()
	if err != nil {
		return nil, false, err
	}
	indexParseStart := time.Now()
	available := ParseAvailablePackagesForInstalled(indexes, installed)
	stageDurations["index_parse"] = time.Since(indexParseStart).Seconds()
	return available, len(indexes) > 0, nil
}

func (c *Collector) packageIndexCacheStats() (uint64, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cacheHits, c.cacheMisses
}

func (c *Collector) packageIndexRevision(ctx context.Context, stageDurations map[string]float64) (string, bool, error) {
	revisioner, ok := c.source.(PackageIndexRevisioner)
	if !ok {
		return "", false, nil
	}
	indexRevisionStart := time.Now()
	revision, err := readWithTimeout[string](ctx, c.timeout, revisioner.PackageIndexRevision)
	stageDurations["index_revision"] = time.Since(indexRevisionStart).Seconds()
	if err != nil {
		return "", false, err
	}
	return revision, true, nil
}

func (c *Collector) sourceMetadata(ctx context.Context) (*SourceMetadata, error) {
	metadataProvider, ok := c.source.(SourceMetadataProvider)
	if !ok {
		return nil, nil
	}
	metadata, err := readWithTimeout[SourceMetadata](ctx, c.timeout, metadataProvider.SourceMetadata)
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}

func aptListFilesTotal(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func installedPackageRevision(installed []InstalledPackage) string {
	var revisions []string
	var b strings.Builder
	for _, pkg := range installed {
		if !isInstalledStatus(pkg.Status) {
			continue
		}
		arch := pkg.Architecture
		if arch == "" {
			arch = "all"
		}
		revisions = append(revisions, fmt.Sprintf("%s:%s:%s\n", pkg.Name, arch, pkg.Version))
	}
	sort.Strings(revisions)
	for _, revision := range revisions {
		b.WriteString(revision)
	}
	return b.String()
}

func readWithTimeout[T any](ctx context.Context, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	if timeout <= 0 {
		return fn(ctx)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(timeoutCtx)
}
