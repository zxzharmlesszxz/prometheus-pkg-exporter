package pkgcheck

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/apt"
)

func TestCheckerWithFixtures(t *testing.T) {
	t.Parallel()

	fixtures := filepath.Join("..", "..", "testdata")
	snapshot := NewChecker(
		fixtureSource{dir: fixtures},
		time.Second,
		filepath.Join(fixtures, "reboot-required"),
	).Snapshot(context.Background(), time.Unix(1700000000, 0))

	if !snapshot.Success {
		t.Fatalf("Snapshot() Success = false, err = %v", snapshot.Err)
	}
	if !snapshot.RebootRequired {
		t.Fatal("Snapshot() RebootRequired = false, want true")
	}
	if snapshot.Stats.Installed == 0 {
		t.Fatal("Snapshot() installed package count = 0, want fixture data")
	}
	if !snapshot.Stats.UpgradeMetricsValid {
		t.Fatal("Snapshot() UpgradeMetricsValid = false, want true")
	}
	if snapshot.CollectionDurationSeconds <= 0 {
		t.Fatalf("Snapshot() CollectionDurationSeconds = %v, want positive", snapshot.CollectionDurationSeconds)
	}
}

func TestObservedSnapshotterReturnsCachedSnapshotOnFailure(t *testing.T) {
	t.Parallel()

	start := time.Unix(1700000000, 0)
	snapshotter := &fakePackageSnapshotter{
		snapshot: Snapshot{
			Stats: apt.Stats{Installed: 10},
		},
	}
	observer := NewObservedSnapshotter(snapshotter)
	first := observer.Snapshot(context.Background(), start)
	if !first.Success {
		t.Fatalf("first Snapshot() Success = false, err = %v", first.Err)
	}

	snapshotter.set(Snapshot{}, errors.New("refresh failed"))
	second := observer.Snapshot(context.Background(), start.Add(time.Minute))
	if second.Success {
		t.Fatal("second Snapshot() Success = true, want false")
	}
	if second.Stats.Installed != 10 {
		t.Fatalf("second Snapshot() Installed = %d, want cached 10", second.Stats.Installed)
	}
}

func TestRebootRequired(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "missing-reboot-required")
	got, err := RebootRequired(missing)
	if err != nil {
		t.Fatalf("RebootRequired(missing) error = %v, want nil", err)
	}
	if got {
		t.Fatal("RebootRequired(missing) = true, want false")
	}

	marker := filepath.Join(dir, "reboot-required")
	if err := os.WriteFile(marker, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile(marker) error = %v", err)
	}
	got, err = RebootRequired(marker)
	if err != nil {
		t.Fatalf("RebootRequired(marker) error = %v, want nil", err)
	}
	if !got {
		t.Fatal("RebootRequired(marker) = false, want true")
	}
}

type fakePackageSnapshotter struct {
	snapshot Snapshot
	err      error
}

func (s *fakePackageSnapshotter) Snapshot(context.Context) (Snapshot, error) {
	return s.snapshot, s.err
}

func (s *fakePackageSnapshotter) set(snapshot Snapshot, err error) {
	s.snapshot = snapshot
	s.err = err
}
