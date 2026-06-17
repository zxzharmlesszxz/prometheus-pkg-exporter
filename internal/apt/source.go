package apt

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pierrec/lz4/v4"
	"github.com/ulikunitz/xz"
)

type PackageIndex struct {
	Name      string
	Data      []byte
	Priority  int
	Backports bool
}

type PackageIndexStream struct {
	Name      string
	Priority  int
	Backports bool
	Open      func() (io.ReadCloser, error)
}

type releaseMetadata struct {
	Priority  int
	Backports bool
}

type Source interface {
	StatusDatabase(context.Context) ([]byte, error)
	PackageIndexes(context.Context) ([]PackageIndex, error)
}

type PackageIndexStreamer interface {
	PackageIndexStreams(context.Context) ([]PackageIndexStream, error)
}

type PackageIndexRevisioner interface {
	PackageIndexRevision(context.Context) (string, error)
}

type SourceMetadata struct {
	DPKGStatusLastModified    time.Time
	APTListsLastModified      time.Time
	APTListFilesByCompression map[string]int
}

type SourceMetadataProvider interface {
	SourceMetadata(context.Context) (SourceMetadata, error)
}

const (
	defaultStatusDatabasePath = "/var/lib/dpkg/status"
	defaultAPTListsPath       = "/var/lib/apt/lists"
)

type CommandSource struct {
	StatusDatabasePath string
	APTListsPath       string
}

func (s CommandSource) StatusDatabase(context.Context) ([]byte, error) {
	path := s.StatusDatabasePath
	if path == "" {
		path = defaultStatusDatabasePath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read status database %s: %w", path, err)
	}
	return data, nil
}

func (s CommandSource) PackageIndexes(context.Context) ([]PackageIndex, error) {
	dir := s.APTListsPath
	if dir == "" {
		dir = defaultAPTListsPath
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read apt lists dir %s: %w", dir, err)
	}

	releaseMetadata, metaErr := readReleaseMetadata(entries, dir)
	if metaErr != nil {
		slog.Debug("read release metadata", "dir", dir, "err", metaErr)
	}
	var indexes []PackageIndex
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isPackageIndexFile(name) {
			continue
		}
		path := filepath.Join(dir, name)
		data, readErr := readPackageIndexFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read apt list %s: %w", path, readErr)
		}
		metadata := packageIndexMetadata(name, releaseMetadata)
		indexes = append(indexes, PackageIndex{
			Name:      name,
			Data:      data,
			Priority:  metadata.Priority,
			Backports: metadata.Backports,
		})
	}

	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].Name < indexes[j].Name
	})
	return indexes, nil
}

func (s CommandSource) PackageIndexStreams(context.Context) ([]PackageIndexStream, error) {
	dir := s.APTListsPath
	if dir == "" {
		dir = defaultAPTListsPath
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read apt lists dir %s: %w", dir, err)
	}

	releaseMetadata, metaErr := readReleaseMetadata(entries, dir)
	if metaErr != nil {
		slog.Debug("read release metadata", "dir", dir, "err", metaErr)
	}
	var streams []PackageIndexStream
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isPackageIndexFile(name) {
			continue
		}
		path := filepath.Join(dir, name)
		metadata := packageIndexMetadata(name, releaseMetadata)
		streams = append(streams, PackageIndexStream{
			Name:      name,
			Priority:  metadata.Priority,
			Backports: metadata.Backports,
			Open: func() (io.ReadCloser, error) {
				reader, readErr := openPackageIndexFile(path)
				if readErr != nil {
					return nil, fmt.Errorf("read apt list %s: %w", path, readErr)
				}
				return reader, nil
			},
		})
	}

	sort.Slice(streams, func(i, j int) bool {
		return streams[i].Name < streams[j].Name
	})
	return streams, nil
}

func (s CommandSource) PackageIndexRevision(context.Context) (string, error) {
	dir := s.APTListsPath
	if dir == "" {
		dir = defaultAPTListsPath
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read apt lists dir %s: %w", dir, err)
	}

	type fileRevision struct {
		name    string
		size    int64
		modTime int64
	}
	var revisions []fileRevision
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isPackageIndexFile(name) {
			if _, ok := releasePrefix(name); !ok {
				continue
			}
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return "", fmt.Errorf("stat apt list %s: %w", filepath.Join(dir, name), infoErr)
		}
		revisions = append(revisions, fileRevision{
			name:    name,
			size:    info.Size(),
			modTime: info.ModTime().UnixNano(),
		})
	}

	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].name < revisions[j].name
	})
	var b strings.Builder
	for _, revision := range revisions {
		fmt.Fprintf(&b, "%s:%d:%d\n", revision.name, revision.size, revision.modTime)
	}
	return b.String(), nil
}

func (s CommandSource) SourceMetadata(context.Context) (SourceMetadata, error) {
	statusPath := s.StatusDatabasePath
	if statusPath == "" {
		statusPath = defaultStatusDatabasePath
	}
	statusInfo, err := os.Stat(statusPath)
	if err != nil {
		return SourceMetadata{}, fmt.Errorf("stat status database %s: %w", statusPath, err)
	}

	aptListsPath := s.APTListsPath
	if aptListsPath == "" {
		aptListsPath = defaultAPTListsPath
	}
	entries, err := os.ReadDir(aptListsPath)
	if err != nil {
		return SourceMetadata{}, fmt.Errorf("read apt lists dir %s: %w", aptListsPath, err)
	}

	metadata := SourceMetadata{
		DPKGStatusLastModified: statusInfo.ModTime(),
		APTListFilesByCompression: map[string]int{
			"plain": 0,
			"gz":    0,
			"bz2":   0,
			"lz4":   0,
			"xz":    0,
		},
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		isPackageIndex := isPackageIndexFile(name)
		if !isPackageIndex {
			if _, ok := releasePrefix(name); !ok {
				continue
			}
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return SourceMetadata{}, fmt.Errorf("stat apt list %s: %w", filepath.Join(aptListsPath, name), infoErr)
		}
		if info.ModTime().After(metadata.APTListsLastModified) {
			metadata.APTListsLastModified = info.ModTime()
		}
		if isPackageIndex {
			metadata.APTListFilesByCompression[packageIndexCompression(name)]++
		}
	}

	return metadata, nil
}

func isPackageIndexFile(name string) bool {
	for _, suffix := range packageIndexFileSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

var packageIndexFileSuffixes = []string{
	"_Packages",
	"_Packages.bz",
	"_Packages.bz2",
	"_Packages.gz",
	"_Packages.lz4",
	"_Packages.xz",
}

func packageIndexCompression(name string) string {
	switch {
	case strings.HasSuffix(name, ".gz"):
		return "gz"
	case strings.HasSuffix(name, ".bz") || strings.HasSuffix(name, ".bz2"):
		return "bz2"
	case strings.HasSuffix(name, ".lz4"):
		return "lz4"
	case strings.HasSuffix(name, ".xz"):
		return "xz"
	default:
		return "plain"
	}
}

func readReleaseMetadata(entries []os.DirEntry, dir string) (map[string]releaseMetadata, error) {
	releases := make(map[string]releaseMetadata)
	var errs []error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		prefix, ok := releasePrefix(name)
		if !ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			errs = append(errs, fmt.Errorf("read release file %s: %w", filepath.Join(dir, name), err))
			continue
		}
		fields := parseDeb822Fields(string(data))
		releases[prefix] = releaseMetadata{
			Priority:  releasePriority(fields),
			Backports: releaseIsBackports(prefix, fields),
		}
	}
	return releases, errors.Join(errs...)
}

func releasePrefix(name string) (string, bool) {
	for _, suffix := range []string{"_InRelease", "_Release"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix), true
		}
	}
	return "", false
}

func packageIndexMetadata(name string, releases map[string]releaseMetadata) releaseMetadata {
	metadata := releaseMetadata{
		Priority:  defaultPackagePriority,
		Backports: isBackportsIndex(name),
	}
	longestPrefix := 0
	for prefix, releaseMetadata := range releases {
		if !strings.HasPrefix(name, prefix+"_") {
			continue
		}
		if len(prefix) <= longestPrefix {
			continue
		}
		metadata = releaseMetadata
		longestPrefix = len(prefix)
	}
	return metadata
}

func releasePriority(fields map[string]string) int {
	if isDeb822Yes(fields["NotAutomatic"]) {
		if isDeb822Yes(fields["ButAutomaticUpgrades"]) {
			return notAutomaticButAutomaticUpgradesPriority
		}
		return notAutomaticPriority
	}
	return defaultPackagePriority
}

func isDeb822Yes(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "yes")
}

func releaseIsBackports(prefix string, fields map[string]string) bool {
	for _, value := range []string{
		prefix,
		fields["Archive"],
		fields["Suite"],
		fields["Codename"],
		fields["Label"],
	} {
		if isBackportsValue(value) {
			return true
		}
	}
	return false
}

func isBackportsIndex(name string) bool {
	return isBackportsValue(name)
}

func isBackportsValue(value string) bool {
	return strings.Contains(strings.ToLower(value), "backports")
}

func readPackageIndexFile(path string) ([]byte, error) {
	reader, err := openPackageIndexFile(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	return io.ReadAll(reader)
}

func openPackageIndexFile(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var reader io.Reader = file
	closeFn := func() error {
		return file.Close()
	}
	switch {
	case strings.HasSuffix(path, ".gz"):
		gzipReader, gzipErr := gzip.NewReader(file)
		if gzipErr != nil {
			_ = file.Close()
			return nil, gzipErr
		}
		reader = gzipReader
		closeFn = func() error {
			gzipErr := gzipReader.Close()
			fileErr := file.Close()
			if gzipErr != nil {
				return gzipErr
			}
			return fileErr
		}
	case strings.HasSuffix(path, ".bz") || strings.HasSuffix(path, ".bz2"):
		reader = bzip2.NewReader(file)
	case strings.HasSuffix(path, ".lz4"):
		reader = lz4.NewReader(file)
	case strings.HasSuffix(path, ".xz"):
		xzReader, xzErr := xz.NewReader(file)
		if xzErr != nil {
			_ = file.Close()
			return nil, xzErr
		}
		reader = xzReader
	}

	return packageIndexReadCloser{
		Reader: bufio.NewReader(reader),
		close:  closeFn,
	}, nil
}

type packageIndexReadCloser struct {
	io.Reader
	close func() error
}

func (r packageIndexReadCloser) Close() error {
	return r.close()
}
