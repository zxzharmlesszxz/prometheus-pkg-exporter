package apt

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dsnet/compress/bzip2"
	"github.com/pierrec/lz4/v4"
	"github.com/ulikunitz/xz"
)

func TestCommandSourceReadsReleasePriority(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "deb.example.org_debian_dists_bookworm-backports_InRelease", `
Origin: Debian Backports
Suite: bookworm-backports
NotAutomatic: yes
ButAutomaticUpgrades: yes
`)
	writeTestFile(t, dir, "deb.example.org_debian_dists_bookworm-backports_main_binary-amd64_Packages", `
Package: base-files
Version: 13.0
Architecture: amd64
`)

	indexes, err := (CommandSource{APTListsPath: dir}).PackageIndexes(context.Background())
	if err != nil {
		t.Fatalf("PackageIndexes() error = %v, want nil", err)
	}
	if len(indexes) != 1 {
		t.Fatalf("PackageIndexes() len = %d, want 1", len(indexes))
	}
	if indexes[0].Priority != 100 {
		t.Fatalf("PackageIndexes()[0].Priority = %d, want 100", indexes[0].Priority)
	}
	if !indexes[0].Backports {
		t.Fatal("PackageIndexes()[0].Backports = false, want true")
	}
}

func TestCommandSourceReadsCompressedPackageIndexes(t *testing.T) {
	t.Parallel()

	const content = `
Package: bash
Version: 5.3
Architecture: amd64
`

	tests := []struct {
		name      string
		fileName  string
		writeFile func(t *testing.T, dir string, name string, content string)
	}{
		{
			name:      "gzip",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.gz",
			writeFile: writeTestGzipFile,
		},
		{
			name:      "bzip",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.bz",
			writeFile: writeTestBzip2File,
		},
		{
			name:      "bzip2",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.bz2",
			writeFile: writeTestBzip2File,
		},
		{
			name:      "lz4",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.lz4",
			writeFile: writeTestLZ4File,
		},
		{
			name:      "xz",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.xz",
			writeFile: writeTestXZFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			tt.writeFile(t, dir, tt.fileName, content)

			indexes, err := (CommandSource{APTListsPath: dir}).PackageIndexes(context.Background())
			if err != nil {
				t.Fatalf("PackageIndexes() error = %v, want nil", err)
			}
			if len(indexes) != 1 {
				t.Fatalf("PackageIndexes() len = %d, want 1", len(indexes))
			}
			if indexes[0].Name != tt.fileName {
				t.Fatalf("PackageIndexes()[0].Name = %q, want %q", indexes[0].Name, tt.fileName)
			}
			if !bytes.Contains(indexes[0].Data, []byte("Package: bash")) {
				t.Fatalf("PackageIndexes()[0].Data missing package data:\n%s", indexes[0].Data)
			}
		})
	}
}

func TestCommandSourceStreamsCompressedPackageIndexes(t *testing.T) {
	t.Parallel()

	const content = `
Package: bash
Version: 5.3
Architecture: amd64
`

	tests := []struct {
		name      string
		fileName  string
		writeFile func(t *testing.T, dir string, name string, content string)
	}{
		{
			name:      "gzip",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.gz",
			writeFile: writeTestGzipFile,
		},
		{
			name:      "bzip",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.bz",
			writeFile: writeTestBzip2File,
		},
		{
			name:      "bzip2",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.bz2",
			writeFile: writeTestBzip2File,
		},
		{
			name:      "lz4",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.lz4",
			writeFile: writeTestLZ4File,
		},
		{
			name:      "xz",
			fileName:  "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.xz",
			writeFile: writeTestXZFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			tt.writeFile(t, dir, tt.fileName, content)

			streams, err := (CommandSource{APTListsPath: dir}).PackageIndexStreams(context.Background())
			if err != nil {
				t.Fatalf("PackageIndexStreams() error = %v, want nil", err)
			}
			if len(streams) != 1 {
				t.Fatalf("PackageIndexStreams() len = %d, want 1", len(streams))
			}
			reader, err := streams[0].Open()
			if err != nil {
				t.Fatalf("Open() error = %v, want nil", err)
			}
			data, err := io.ReadAll(reader)
			if closeErr := reader.Close(); closeErr != nil {
				t.Fatalf("Close() error = %v, want nil", closeErr)
			}
			if err != nil {
				t.Fatalf("ReadAll() error = %v, want nil", err)
			}
			if !bytes.Contains(data, []byte("Package: bash")) {
				t.Fatalf("stream data missing package data:\n%s", data)
			}
		})
	}
}

func TestCommandSourceReportsSourceMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	statusPath := filepath.Join(dir, "status")
	writeTestFile(t, dir, "status", "Package: bash\n")
	writeTestFile(t, dir, "deb.example.org_debian_dists_bookworm_InRelease", "Suite: bookworm\n")
	writeTestFile(t, dir, "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages", "Package: bash\n")
	writeTestGzipFile(t, dir, "deb.example.org_debian_dists_bookworm_main_binary-all_Packages.gz", "Package: bash\n")
	writeTestLZ4File(t, dir, "deb.example.org_debian_dists_bookworm_updates_binary-amd64_Packages.lz4", "Package: bash\n")
	writeTestXZFile(t, dir, "deb.example.org_debian_dists_bookworm_security_binary-amd64_Packages.xz", "Package: bash\n")
	writeTestBzip2File(t, dir, "deb.example.org_debian_dists_bookworm_contrib_binary-amd64_Packages.bz2", `
Package: bash
Version: 5.3
Architecture: amd64
`)

	statusTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	listsTime := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := os.Chtimes(statusPath, statusTime, statusTime); err != nil {
		t.Fatalf("chtimes status: %v", err)
	}
	if err := os.Chtimes(filepath.Join(dir, "deb.example.org_debian_dists_bookworm_updates_binary-amd64_Packages.lz4"), listsTime, listsTime); err != nil {
		t.Fatalf("chtimes apt list: %v", err)
	}

	metadata, err := (CommandSource{StatusDatabasePath: statusPath, APTListsPath: dir}).SourceMetadata(context.Background())
	if err != nil {
		t.Fatalf("SourceMetadata() error = %v, want nil", err)
	}
	if !metadata.DPKGStatusLastModified.Equal(statusTime) {
		t.Fatalf("DPKGStatusLastModified = %v, want %v", metadata.DPKGStatusLastModified, statusTime)
	}
	if !metadata.APTListsLastModified.Equal(listsTime) {
		t.Fatalf("APTListsLastModified = %v, want %v", metadata.APTListsLastModified, listsTime)
	}
	wantCounts := map[string]int{"plain": 1, "gz": 1, "bz2": 1, "lz4": 1, "xz": 1}
	for compression, want := range wantCounts {
		if got := metadata.APTListFilesByCompression[compression]; got != want {
			t.Fatalf("APTListFilesByCompression[%q] = %d, want %d", compression, got, want)
		}
	}
}

func TestCommandSourceReportsEmptyAPTListsMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	statusPath := filepath.Join(dir, "status")
	aptListsPath := filepath.Join(dir, "apt-lists")
	writeTestFile(t, dir, "status", "Package: bash\n")
	if err := os.Mkdir(aptListsPath, 0o755); err != nil {
		t.Fatalf("mkdir apt lists: %v", err)
	}

	metadata, err := (CommandSource{StatusDatabasePath: statusPath, APTListsPath: aptListsPath}).SourceMetadata(context.Background())
	if err != nil {
		t.Fatalf("SourceMetadata() error = %v, want nil", err)
	}
	if metadata.APTListsLastModified != (time.Time{}) {
		t.Fatalf("APTListsLastModified = %v, want zero", metadata.APTListsLastModified)
	}
	for _, compression := range []string{"plain", "gz", "bz2", "lz4", "xz"} {
		if got := metadata.APTListFilesByCompression[compression]; got != 0 {
			t.Fatalf("APTListFilesByCompression[%q] = %d, want 0", compression, got)
		}
	}
}

func TestCommandSourceReportsStatusDatabaseErrors(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing-status")
	if _, err := (CommandSource{StatusDatabasePath: path}).StatusDatabase(context.Background()); err == nil {
		t.Fatal("StatusDatabase() error = nil, want missing status error")
	}
	if _, err := (CommandSource{StatusDatabasePath: path, APTListsPath: t.TempDir()}).SourceMetadata(context.Background()); err == nil {
		t.Fatal("SourceMetadata() error = nil, want missing status error")
	}
}

func TestCommandSourceReportsAPTListsErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	statusPath := filepath.Join(dir, "status")
	writeTestFile(t, dir, "status", "Package: bash\n")
	missingAPTListsPath := filepath.Join(dir, "missing-apt-lists")
	source := CommandSource{StatusDatabasePath: statusPath, APTListsPath: missingAPTListsPath}

	if _, err := source.PackageIndexes(context.Background()); err == nil {
		t.Fatal("PackageIndexes() error = nil, want missing apt lists error")
	}
	if _, err := source.PackageIndexStreams(context.Background()); err == nil {
		t.Fatal("PackageIndexStreams() error = nil, want missing apt lists error")
	}
	if _, err := source.PackageIndexRevision(context.Background()); err == nil {
		t.Fatal("PackageIndexRevision() error = nil, want missing apt lists error")
	}
	if _, err := source.SourceMetadata(context.Background()); err == nil {
		t.Fatal("SourceMetadata() error = nil, want missing apt lists error")
	}
}

func TestCommandSourcePackageIndexRevisionIncludesIndexesAndReleases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "deb.example.org_debian_dists_bookworm_InRelease", "Suite: bookworm\n")
	writeTestFile(t, dir, "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages", "Package: bash\n")
	writeTestFile(t, dir, "unrelated.txt", "ignored\n")
	if err := os.Mkdir(filepath.Join(dir, "partial"), 0o755); err != nil {
		t.Fatalf("mkdir partial: %v", err)
	}

	revision, err := (CommandSource{APTListsPath: dir}).PackageIndexRevision(context.Background())
	if err != nil {
		t.Fatalf("PackageIndexRevision() error = %v, want nil", err)
	}
	for _, want := range []string{
		"deb.example.org_debian_dists_bookworm_InRelease:",
		"deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages:",
	} {
		if !strings.Contains(revision, want) {
			t.Fatalf("PackageIndexRevision() = %q, want entry containing %q", revision, want)
		}
	}
	if strings.Contains(revision, "unrelated.txt") || strings.Contains(revision, "partial") {
		t.Fatalf("PackageIndexRevision() = %q, want unrelated entries ignored", revision)
	}
}

func TestCommandSourceReadsNotAutomaticReleasePriority(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "deb.example.org_debian_dists_bookworm_InRelease", `
NotAutomatic: yes
Archive: bookworm-backports
`)
	writeTestFile(t, dir, "deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages", `
Package: base-files
Version: 13.0
Architecture: amd64
`)

	indexes, err := (CommandSource{APTListsPath: dir}).PackageIndexes(context.Background())
	if err != nil {
		t.Fatalf("PackageIndexes() error = %v, want nil", err)
	}
	if len(indexes) != 1 {
		t.Fatalf("PackageIndexes() len = %d, want 1", len(indexes))
	}
	if indexes[0].Priority != 1 {
		t.Fatalf("PackageIndexes()[0].Priority = %d, want 1", indexes[0].Priority)
	}
	if !indexes[0].Backports {
		t.Fatal("PackageIndexes()[0].Backports = false, want true")
	}
}

func TestCommandSourceReportsInvalidCompressedIndexes(t *testing.T) {
	t.Parallel()

	tests := []string{
		"deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.gz",
		"deb.example.org_debian_dists_bookworm_main_binary-amd64_Packages.xz",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeTestFile(t, dir, name, "not compressed")
			if _, err := (CommandSource{APTListsPath: dir}).PackageIndexes(context.Background()); err == nil {
				t.Fatal("PackageIndexes() error = nil, want invalid compressed index error")
			}
		})
	}
}

func writeTestFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
}

func writeTestGzipFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("compress test file %s: %v", name, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close compressed test file %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
}

func writeTestBzip2File(t *testing.T, dir string, name string, content string) {
	t.Helper()

	var buf bytes.Buffer
	writer, err := bzip2.NewWriter(&buf, nil)
	if err != nil {
		t.Fatalf("create bzip2 writer for test file %s: %v", name, err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("compress test file %s: %v", name, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close compressed test file %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
}

func writeTestLZ4File(t *testing.T, dir string, name string, content string) {
	t.Helper()

	var buf bytes.Buffer
	writer := lz4.NewWriter(&buf)
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("compress test file %s: %v", name, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close compressed test file %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
}

func writeTestXZFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	var buf bytes.Buffer
	writer, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatalf("create xz writer for test file %s: %v", name, err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("compress test file %s: %v", name, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close compressed test file %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
}
