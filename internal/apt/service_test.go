package apt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeSource struct {
	status     []byte
	statusErr  error
	indexes    []PackageIndex
	indexesErr error
}

func (s fakeSource) StatusDatabase(context.Context) ([]byte, error) { return s.status, s.statusErr }
func (s fakeSource) PackageIndexes(context.Context) ([]PackageIndex, error) {
	return s.indexes, s.indexesErr
}

type countingSource struct {
	status        []byte
	indexes       []PackageIndex
	revision      string
	statusCalls   int
	indexesCalls  int
	revisionCalls int
}

func (s *countingSource) StatusDatabase(context.Context) ([]byte, error) {
	s.statusCalls++
	return s.status, nil
}

func (s *countingSource) PackageIndexes(context.Context) ([]PackageIndex, error) {
	s.indexesCalls++
	return s.indexes, nil
}

func (s *countingSource) PackageIndexRevision(context.Context) (string, error) {
	s.revisionCalls++
	return s.revision, nil
}

func TestCollect(t *testing.T) {
	t.Parallel()

	source := fakeSource{
		status: []byte(strings.TrimSpace(`
Package: bash
Status: install ok installed

Package: coreutils
Status: install ok installed

Package: broken
Status: install ok unpacked
`)),
		indexes: []PackageIndex{
			{
				Name: "debian_main_Packages",
				Data: []byte(strings.TrimSpace(`
Package: base-files
Version: 1
Architecture: amd64
`)),
			},
			{
				Name: "debian_security_Packages",
				Data: []byte(strings.TrimSpace(`
Package: openssl
Version: 2
Architecture: amd64
`)),
			},
		},
	}

	stats, err := Collect(context.Background(), source, time.Second)
	if err != nil {
		t.Fatalf("Collect() error = %v, want nil", err)
	}

	if stats.Installed != 2 || stats.Upgradable != 0 || stats.SecurityUpgradable != 0 || stats.Broken != 1 {
		t.Fatalf("Collect() = %+v, want {Installed:2 Upgradable:0 SecurityUpgradable:0 Broken:1}", stats)
	}
	if !stats.UpgradeMetricsValid {
		t.Fatal("Collect() UpgradeMetricsValid = false, want true")
	}
}

func TestCollectMarksUpgradeMetricsInvalidWithoutPackageIndexes(t *testing.T) {
	t.Parallel()

	source := fakeSource{
		status: []byte(strings.TrimSpace(`
Package: bash
Status: install ok installed
Version: 5.2
Architecture: amd64
`)),
	}

	stats, err := Collect(context.Background(), source, time.Second)
	if err != nil {
		t.Fatalf("Collect() error = %v, want nil", err)
	}
	if stats.UpgradeMetricsValid {
		t.Fatal("Collect() UpgradeMetricsValid = true, want false")
	}
}

func TestCommandSourceCollectWithEmptyAPTListsMarksUpgradeMetricsInvalid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	statusPath := filepath.Join(dir, "status")
	aptListsPath := filepath.Join(dir, "apt-lists")
	if err := os.WriteFile(statusPath, []byte(strings.TrimSpace(`
Package: bash
Status: install ok installed
Version: 5.2
Architecture: amd64
`)), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if err := os.Mkdir(aptListsPath, 0o755); err != nil {
		t.Fatalf("mkdir apt lists: %v", err)
	}

	stats, err := Collect(context.Background(), CommandSource{
		StatusDatabasePath: statusPath,
		APTListsPath:       aptListsPath,
	}, time.Second)
	if err != nil {
		t.Fatalf("Collect() error = %v, want nil", err)
	}
	if stats.Installed != 1 {
		t.Fatalf("Collect() Installed = %d, want 1", stats.Installed)
	}
	if stats.UpgradeMetricsValid {
		t.Fatal("Collect() UpgradeMetricsValid = true, want false")
	}
	if stats.Upgradable != 0 || stats.SecurityUpgradable != 0 || stats.PinnedUpdates != 0 || stats.BackportsUpdates != 0 {
		t.Fatalf("Collect() upgrade metrics = upgradable:%d security:%d pinned:%d backports:%d; want all 0 with invalid flag",
			stats.Upgradable,
			stats.SecurityUpgradable,
			stats.PinnedUpdates,
			stats.BackportsUpdates,
		)
	}
	if !stats.APTListsLastModified.IsZero() {
		t.Fatalf("Collect() APTListsLastModified = %v, want zero", stats.APTListsLastModified)
	}
	for _, compression := range []string{"plain", "gz", "bz2", "lz4", "xz"} {
		if got := stats.APTListFilesCount[compression]; got != 0 {
			t.Fatalf("Collect() APTListFilesCount[%q] = %d, want 0", compression, got)
		}
	}
}

func TestCollectReturnsJoinedErrors(t *testing.T) {
	t.Parallel()

	source := fakeSource{
		statusErr: errors.New("status unavailable"),
	}

	_, err := Collect(context.Background(), source, time.Second)
	if err == nil {
		t.Fatal("Collect() error = nil, want non-nil")
	}
}

func TestCollectorCachesPackageIndexesByRevision(t *testing.T) {
	t.Parallel()

	source := &countingSource{
		status: []byte(strings.TrimSpace(`
Package: bash
Status: install ok installed
Version: 5.2
Architecture: amd64
`)),
		indexes: []PackageIndex{
			{
				Name: "debian_main_Packages",
				Data: []byte(strings.TrimSpace(`
Package: bash
Version: 5.3
Architecture: amd64

Package: unused
Version: 1.0
Architecture: amd64
`)),
			},
		},
		revision: "rev1",
	}
	collector := NewCollector(source, time.Second)

	first, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("first Collect() error = %v, want nil", err)
	}
	second, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("second Collect() error = %v, want nil", err)
	}

	if first.Upgradable != 1 || second.Upgradable != 1 {
		t.Fatalf("Collect() Upgradable = %d, %d; want 1, 1", first.Upgradable, second.Upgradable)
	}
	if !first.UpgradeMetricsValid || !second.UpgradeMetricsValid {
		t.Fatalf("Collect() UpgradeMetricsValid = %v, %v; want true, true", first.UpgradeMetricsValid, second.UpgradeMetricsValid)
	}
	if first.PackageIndexCacheHits != 0 || first.PackageIndexCacheMisses != 1 {
		t.Fatalf("first cache stats = hits:%d misses:%d, want hits:0 misses:1", first.PackageIndexCacheHits, first.PackageIndexCacheMisses)
	}
	if second.PackageIndexCacheHits != 1 || second.PackageIndexCacheMisses != 1 {
		t.Fatalf("second cache stats = hits:%d misses:%d, want hits:1 misses:1", second.PackageIndexCacheHits, second.PackageIndexCacheMisses)
	}
	if source.statusCalls != 2 {
		t.Fatalf("StatusDatabase() calls = %d, want 2", source.statusCalls)
	}
	if source.revisionCalls != 2 {
		t.Fatalf("PackageIndexRevision() calls = %d, want 2", source.revisionCalls)
	}
	if source.indexesCalls != 1 {
		t.Fatalf("PackageIndexes() calls = %d, want 1", source.indexesCalls)
	}
}

func TestCollectorInvalidatesCacheWhenInstalledVersionChanges(t *testing.T) {
	t.Parallel()

	source := &countingSource{
		status: []byte(strings.TrimSpace(`
Package: bash
Status: install ok installed
Version: 5.3
Architecture: amd64
`)),
		indexes: []PackageIndex{
			{
				Name: "debian_main_Packages",
				Data: []byte(strings.TrimSpace(`
Package: bash
Version: 5.3
Architecture: amd64
`)),
			},
		},
		revision: "rev1",
	}
	collector := NewCollector(source, time.Second)

	first, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("first Collect() error = %v, want nil", err)
	}
	source.status = []byte(strings.TrimSpace(`
Package: bash
Status: install ok installed
Version: 5.2
Architecture: amd64
`))
	second, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("second Collect() error = %v, want nil", err)
	}

	if first.Upgradable != 0 {
		t.Fatalf("first Collect() Upgradable = %d, want 0", first.Upgradable)
	}
	if second.Upgradable != 1 {
		t.Fatalf("second Collect() Upgradable = %d, want 1", second.Upgradable)
	}
	if first.PackageIndexCacheHits != 0 || first.PackageIndexCacheMisses != 1 {
		t.Fatalf("first cache stats = hits:%d misses:%d, want hits:0 misses:1", first.PackageIndexCacheHits, first.PackageIndexCacheMisses)
	}
	if second.PackageIndexCacheHits != 0 || second.PackageIndexCacheMisses != 2 {
		t.Fatalf("second cache stats = hits:%d misses:%d, want hits:0 misses:2", second.PackageIndexCacheHits, second.PackageIndexCacheMisses)
	}
	if source.indexesCalls != 2 {
		t.Fatalf("PackageIndexes() calls = %d, want 2", source.indexesCalls)
	}
}
