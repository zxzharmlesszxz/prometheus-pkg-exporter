package pkgcheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/apt"
)

type fixtureSource struct {
	dir string
}

func (s fixtureSource) StatusDatabase(context.Context) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.dir, "dpkg-status"))
}

func (s fixtureSource) PackageIndexes(context.Context) ([]apt.PackageIndex, error) {
	dir := filepath.Join(s.dir, "apt-lists")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read fixture dir %s: %w", dir, err)
	}

	var indexes []apt.PackageIndex
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			return nil, fmt.Errorf("read fixture %s: %w", name, readErr)
		}
		indexes = append(indexes, apt.PackageIndex{Name: name, Data: data})
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].Name < indexes[j].Name
	})
	return indexes, nil
}
