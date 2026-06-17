package pkgcheck

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/apt"
)

const defaultRebootRequiredPath = "/run/reboot-required"

type Checker struct {
	snapshotter Snapshotter
	observer    *ObservedSnapshotter
}

func NewChecker(source apt.Source, commandTimeout time.Duration, rebootRequiredPath string) Checker {
	snapshotter := aptSnapshotter{
		collector:          apt.NewCollector(source, commandTimeout),
		rebootRequiredPath: rebootRequiredPath,
	}
	return Checker{
		snapshotter: snapshotter,
		observer:    NewObservedSnapshotter(snapshotter),
	}
}

type aptSnapshotter struct {
	collector          *apt.Collector
	rebootRequiredPath string
}

func (c Checker) Snapshot(ctx context.Context, attemptTime time.Time) Snapshot {
	return c.observer.Snapshot(ctx, attemptTime)
}

func (s aptSnapshotter) Snapshot(ctx context.Context) (Snapshot, error) {
	start := time.Now()
	stats, collectErr := s.collector.Collect(ctx)
	rebootRequired, rebootErr := RebootRequired(s.rebootRequiredPath)
	err := errors.Join(collectErr, rebootErr)
	return Snapshot{
		Success:                   err == nil,
		Err:                       err,
		Stats:                     stats,
		RebootRequired:            rebootRequired,
		CollectionDurationSeconds: time.Since(start).Seconds(),
	}, err
}

func RebootRequired(path string) (bool, error) {
	if path == "" {
		path = defaultRebootRequiredPath
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat reboot-required marker %s: %w", path, err)
	}
	return true, nil
}

type ObservedSnapshotter struct {
	snapshotter Snapshotter

	mu            sync.Mutex
	lastSnapshot  Snapshot
	hasSuccessful bool
}

func NewObservedSnapshotter(snapshotter Snapshotter) *ObservedSnapshotter {
	return &ObservedSnapshotter{snapshotter: snapshotter}
}

func (s *ObservedSnapshotter) Snapshot(ctx context.Context, attemptTime time.Time) Snapshot {
	snapshot, err := s.snapshotter.Snapshot(ctx)
	snapshot.AttemptTime = attemptTime
	snapshot.Success = err == nil
	snapshot.Err = err
	if err == nil {
		s.mu.Lock()
		s.lastSnapshot = snapshot
		s.hasSuccessful = true
		s.mu.Unlock()
		return snapshot
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasSuccessful {
		return snapshot
	}
	cached := s.lastSnapshot
	cached.AttemptTime = attemptTime
	cached.Success = false
	cached.Err = err
	cached.CollectionDurationSeconds = snapshot.CollectionDurationSeconds
	return cached
}
