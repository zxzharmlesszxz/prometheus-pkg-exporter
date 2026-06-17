package apt

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestCountInstalledPackages(t *testing.T) {
	t.Parallel()

	input := strings.TrimSpace(`
Package: bash
Status: install ok installed

Package: coreutils
Status: install ok installed

Package: held
Status: hold ok installed

Package: broken-unpacked
Status: install ok unpacked

Package: removed
Status: deinstall ok config-files
`)
	if got := CountInstalledPackages(input); got != 3 {
		t.Fatalf("CountInstalledPackages() = %d, want 3", got)
	}
}

func TestCountUpgradablePackages(t *testing.T) {
	t.Parallel()

	statusDatabase := strings.TrimSpace(`
Package: base-files
Status: install ok installed
Version: 12.9
Architecture: amd64

Package: openssl
Status: install ok installed
Version: 3.0.16
Architecture: amd64
`)
	indexes := []PackageIndex{
		{
			Name: "debian_main_Packages",
			Data: []byte(strings.TrimSpace(`
Package: base-files
Version: 13.0
Architecture: amd64

Package: openssl
Version: 3.0.17
Architecture: amd64
`)),
		},
	}
	if got := CountUpgradablePackages(statusDatabase, indexes); got != 2 {
		t.Fatalf("CountUpgradablePackages() = %d, want 2", got)
	}
}

func TestCountUpgradablePackagesIgnoresLowerPriorityBackports(t *testing.T) {
	t.Parallel()

	statusDatabase := strings.TrimSpace(`
Package: base-files
Status: install ok installed
Version: 12.9
Architecture: amd64
`)
	indexes := []PackageIndex{
		{
			Name:     "debian_main_Packages",
			Priority: 500,
			Data: []byte(strings.TrimSpace(`
Package: base-files
Version: 12.9
Architecture: amd64
`)),
		},
		{
			Name:     "debian_backports_Packages",
			Priority: 100,
			Data: []byte(strings.TrimSpace(`
Package: base-files
Version: 13.0
Architecture: amd64
`)),
		},
	}
	if got := CountUpgradablePackages(statusDatabase, indexes); got != 0 {
		t.Fatalf("CountUpgradablePackages() = %d, want 0", got)
	}
	if got := CountPinnedUpdatePackages(statusDatabase, indexes); got != 0 {
		t.Fatalf("CountPinnedUpdatePackages() = %d, want 0", got)
	}
	if got := CountBackportsUpdatePackages(statusDatabase, indexes); got != 1 {
		t.Fatalf("CountBackportsUpdatePackages() = %d, want 1", got)
	}
}

func TestCountUpgradablePackagesCountsBackportsWhenInstalledFromBackports(t *testing.T) {
	t.Parallel()

	statusDatabase := strings.TrimSpace(`
Package: base-files
Status: install ok installed
Version: 13.0
Architecture: amd64
`)
	indexes := []PackageIndex{
		{
			Name:     "debian_main_Packages",
			Priority: 500,
			Data: []byte(strings.TrimSpace(`
Package: base-files
Version: 12.9
Architecture: amd64
`)),
		},
		{
			Name:     "debian_backports_Packages",
			Priority: 100,
			Data: []byte(strings.TrimSpace(`
Package: base-files
Version: 13.1
Architecture: amd64
`)),
		},
	}
	if got := CountUpgradablePackages(statusDatabase, indexes); got != 1 {
		t.Fatalf("CountUpgradablePackages() = %d, want 1", got)
	}
	if got := CountPinnedUpdatePackages(statusDatabase, indexes); got != 0 {
		t.Fatalf("CountPinnedUpdatePackages() = %d, want 0", got)
	}
	if got := CountBackportsUpdatePackages(statusDatabase, indexes); got != 1 {
		t.Fatalf("CountBackportsUpdatePackages() = %d, want 1", got)
	}
}

func TestCountSecurityUpgradablePackages(t *testing.T) {
	t.Parallel()

	statusDatabase := strings.TrimSpace(`
Package: openssl
Status: install ok installed
Version: 3.0.16
Architecture: amd64

Package: openssh-server
Status: install ok installed
Version: 1:9.7
Architecture: amd64

Package: bash
Status: install ok installed
Version: 5.2
Architecture: amd64
`)
	indexes := []PackageIndex{
		{
			Name: "debian_security_Packages",
			Data: []byte(strings.TrimSpace(`
Package: openssl
Version: 3.0.17
Architecture: amd64

Package: openssh-server
Version: 1:9.8
Architecture: amd64
`)),
		},
		{
			Name: "debian_main_Packages",
			Data: []byte(strings.TrimSpace(`
Package: bash
Version: 5.3
Architecture: amd64
`)),
		},
	}
	if got := CountSecurityUpgradablePackages(statusDatabase, indexes); got != 2 {
		t.Fatalf("CountSecurityUpgradablePackages() = %d, want 2", got)
	}
}

func TestCountSecurityPinnedUpdatePackages(t *testing.T) {
	t.Parallel()

	statusDatabase := strings.TrimSpace(`
Package: openssl
Status: install ok installed
Version: 3.0.16
Architecture: amd64
`)
	indexes := []PackageIndex{
		{
			Name:     "debian_main_Packages",
			Priority: 500,
			Data: []byte(strings.TrimSpace(`
Package: openssl
Version: 3.0.16
Architecture: amd64
`)),
		},
		{
			Name:     "debian_security_backports_Packages",
			Priority: 100,
			Data: []byte(strings.TrimSpace(`
Package: openssl
Version: 3.0.17
Architecture: amd64
`)),
		},
	}
	if got := CountSecurityUpgradablePackages(statusDatabase, indexes); got != 0 {
		t.Fatalf("CountSecurityUpgradablePackages() = %d, want 0", got)
	}
	if got := CountSecurityPinnedUpdatePackages(statusDatabase, indexes); got != 0 {
		t.Fatalf("CountSecurityPinnedUpdatePackages() = %d, want 0", got)
	}
}

func TestParseAvailablePackagesFromStreamsMatchesByteParser(t *testing.T) {
	t.Parallel()

	installed := ParseInstalledPackages(strings.TrimSpace(`
Package: base-files
Status: install ok installed
Version: 12.9
Architecture: amd64

Package: openssl
Status: install ok installed
Version: 3.0.16
Architecture: amd64
`))
	content := strings.TrimSpace(`
Package: base-files
Version: 13.0
Architecture: amd64
Description: first line
 continuation line

Package: ignored
Version: 1.0
Architecture: amd64

Package: openssl
Version: 3.0.17
Architecture: all
`)
	indexes := []PackageIndex{
		{
			Name:     "debian_security_Packages",
			Priority: 500,
			Data:     []byte(content),
		},
	}
	streams := []PackageIndexStream{
		{
			Name:     indexes[0].Name,
			Priority: indexes[0].Priority,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(content)), nil
			},
		},
	}

	want := ParseAvailablePackagesForInstalled(indexes, installed)
	got, err := ParseAvailablePackagesFromStreams(streams, installed)
	if err != nil {
		t.Fatalf("ParseAvailablePackagesFromStreams() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseAvailablePackagesFromStreams() = %#v, want %#v", got, want)
	}
}

func TestCountPinnedUpdatePackagesExcludesBackports(t *testing.T) {
	t.Parallel()

	statusDatabase := strings.TrimSpace(`
Package: base-files
Status: install ok installed
Version: 12.9
Architecture: amd64
`)
	indexes := []PackageIndex{
		{
			Name:     "debian_main_Packages",
			Priority: 500,
			Data: []byte(strings.TrimSpace(`
Package: base-files
Version: 12.9
Architecture: amd64
`)),
		},
		{
			Name:     "debian_pinned_Packages",
			Priority: 100,
			Data: []byte(strings.TrimSpace(`
Package: base-files
Version: 13.0
Architecture: amd64
`)),
		},
		{
			Name:      "debian_backports_Packages",
			Priority:  100,
			Backports: true,
			Data: []byte(strings.TrimSpace(`
Package: base-files
Version: 14.0
Architecture: amd64
`)),
		},
	}
	if got := CountPinnedUpdatePackages(statusDatabase, indexes); got != 1 {
		t.Fatalf("CountPinnedUpdatePackages() = %d, want 1", got)
	}
	if got := CountBackportsUpdatePackages(statusDatabase, indexes); got != 1 {
		t.Fatalf("CountBackportsUpdatePackages() = %d, want 1", got)
	}
}

func TestHeldPackagesCountAsPinnedUpdates(t *testing.T) {
	t.Parallel()

	statusDatabase := strings.TrimSpace(`
Package: jenkins
Status: hold ok installed
Version: 2.500
Architecture: amd64
`)
	indexes := []PackageIndex{
		{
			Name:     "jenkins_Packages",
			Priority: 500,
			Data: []byte(strings.TrimSpace(`
Package: jenkins
Version: 2.501
Architecture: amd64
`)),
		},
	}
	if got := CountInstalledPackages(statusDatabase); got != 1 {
		t.Fatalf("CountInstalledPackages() = %d, want 1", got)
	}
	if got := CountUpgradablePackages(statusDatabase, indexes); got != 0 {
		t.Fatalf("CountUpgradablePackages() = %d, want 0", got)
	}
	if got := CountPinnedUpdatePackages(statusDatabase, indexes); got != 1 {
		t.Fatalf("CountPinnedUpdatePackages() = %d, want 1", got)
	}
}

func TestCountBrokenPackages(t *testing.T) {
	t.Parallel()

	input := strings.TrimSpace(`
Package: adduser
Status: install ok installed

Package: broken-unpacked
Status: install ok unpacked

Package: half-configured
Status: install ok half-configured

Package: removed
Status: deinstall ok config-files
`)
	if got := CountBrokenPackages(input); got != 2 {
		t.Fatalf("CountBrokenPackages() = %d, want 2", got)
	}
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		left     string
		right    string
		expected int
	}{
		{name: "equal", left: "1.0", right: "1.0", expected: 0},
		{name: "numeric ordering", left: "1.10", right: "1.2", expected: 1},
		{name: "revision ordering", left: "1.0-2", right: "1.0-1", expected: 1},
		{name: "epoch ordering", left: "2:1.0", right: "1:9.9", expected: 1},
		{name: "tilde sorts before release", left: "1.0~rc1", right: "1.0", expected: -1},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := CompareVersions(tc.left, tc.right); got != tc.expected {
				t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.expected)
			}
		})
	}
}
