package apt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

const (
	defaultPackagePriority                   = 500
	installedPackagePriority                 = 100
	notAutomaticButAutomaticUpgradesPriority = 100
	notAutomaticPriority                     = 1
	forceDowngradePriority                   = 1000
)

type InstalledPackage struct {
	Name         string
	Architecture string
	Version      string
	Status       []string
}

type AvailablePackage struct {
	Name         string
	Architecture string
	Version      string
	Security     bool
	Priority     int
	Backports    bool
}

func CountInstalledPackages(statusDatabase string) int {
	return countInstalledPackages(ParseInstalledPackages(statusDatabase))
}

func countInstalledPackages(installed []InstalledPackage) int {
	count := 0
	for _, pkg := range installed {
		if isInstalledStatus(pkg.Status) {
			count++
		}
	}
	return count
}

func CountBrokenPackages(statusDatabase string) int {
	return countBrokenPackages(ParseInstalledPackages(statusDatabase))
}

func countBrokenPackages(installed []InstalledPackage) int {
	count := 0
	for _, pkg := range installed {
		if isBrokenStatus(pkg.Status) {
			count++
		}
	}
	return count
}

func CountUpgradablePackages(statusDatabase string, packageIndexes []PackageIndex) int {
	installed := ParseInstalledPackages(statusDatabase)
	available := ParseAvailablePackages(packageIndexes)
	return countUpgradable(installed, available, false)
}

func CountSecurityUpgradablePackages(statusDatabase string, packageIndexes []PackageIndex) int {
	installed := ParseInstalledPackages(statusDatabase)
	available := ParseAvailablePackages(packageIndexes)
	return countUpgradable(installed, available, true)
}

func CountPinnedUpdatePackages(statusDatabase string, packageIndexes []PackageIndex) int {
	installed := ParseInstalledPackages(statusDatabase)
	available := ParseAvailablePackages(packageIndexes)
	return countPinnedUpdates(installed, available, false)
}

func CountSecurityPinnedUpdatePackages(statusDatabase string, packageIndexes []PackageIndex) int {
	installed := ParseInstalledPackages(statusDatabase)
	available := ParseAvailablePackages(packageIndexes)
	return countPinnedUpdates(installed, available, true)
}

func CountBackportsUpdatePackages(statusDatabase string, packageIndexes []PackageIndex) int {
	installed := ParseInstalledPackages(statusDatabase)
	available := ParseAvailablePackages(packageIndexes)
	return countBackportsUpdates(installed, available)
}

func ParseInstalledPackages(statusDatabase string) []InstalledPackage {
	var result []InstalledPackage
	for _, entry := range splitParagraphs(statusDatabase) {
		fields := parseDeb822Fields(entry)
		pkg := InstalledPackage{
			Name:         fields["Package"],
			Architecture: fields["Architecture"],
			Version:      fields["Version"],
			Status:       strings.Fields(fields["Status"]),
		}
		if pkg.Name == "" {
			continue
		}
		result = append(result, pkg)
	}
	return result
}

func ParseAvailablePackages(packageIndexes []PackageIndex) []AvailablePackage {
	return parseAvailablePackages(packageIndexes, nil)
}

func ParseAvailablePackagesForInstalled(packageIndexes []PackageIndex, installed []InstalledPackage) []AvailablePackage {
	return parseAvailablePackages(packageIndexes, installedPackageFilter(installed))
}

func ParseAvailablePackagesFromStreams(packageIndexes []PackageIndexStream, installed []InstalledPackage) ([]AvailablePackage, error) {
	return parseAvailablePackagesFromStreams(packageIndexes, installedPackageFilter(installed))
}

func parseAvailablePackages(packageIndexes []PackageIndex, filter map[string]map[string]struct{}) []AvailablePackage {
	var result []AvailablePackage
	for _, index := range packageIndexes {
		security := isSecurityIndex(index.Name)
		priority := normalizedPackagePriority(index.Priority)
		backports := index.Backports || isBackportsIndex(index.Name)
		forEachByteParagraph(index.Data, func(entry []byte) {
			name, architecture, version := parseAvailablePackageFields(entry, filter)
			pkg := AvailablePackage{
				Name:         name,
				Architecture: architecture,
				Version:      version,
				Security:     security,
				Priority:     priority,
				Backports:    backports,
			}
			appendAvailablePackage(&result, pkg, filter)
		})
	}
	return result
}

func parseAvailablePackagesFromStreams(packageIndexes []PackageIndexStream, filter map[string]map[string]struct{}) ([]AvailablePackage, error) {
	var result []AvailablePackage
	for _, index := range packageIndexes {
		reader, err := index.Open()
		if err != nil {
			return nil, err
		}
		parseErr := parseAvailablePackageReader(reader, index.Name, index.Priority, index.Backports, filter, &result)
		closeErr := reader.Close()
		if parseErr != nil {
			return nil, parseErr
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close package index %s: %w", index.Name, closeErr)
		}
	}
	return result, nil
}

func parseAvailablePackageReader(
	reader io.Reader,
	indexName string,
	priority int,
	backports bool,
	filter map[string]map[string]struct{},
	result *[]AvailablePackage,
) error {
	security := isSecurityIndex(indexName)
	priority = normalizedPackagePriority(priority)
	backports = backports || isBackportsIndex(indexName)

	var name string
	var architecture string
	var version string
	flush := func() {
		appendAvailablePackage(result, AvailablePackage{
			Name:         name,
			Architecture: architecture,
			Version:      version,
			Security:     security,
			Priority:     priority,
			Backports:    backports,
		}, filter)
		name = ""
		architecture = ""
		version = ""
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			flush()
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			continue
		}
		field, value, ok := bytes.Cut(line, []byte(":"))
		if !ok {
			continue
		}
		value = bytes.TrimSpace(value)
		switch {
		case bytes.Equal(field, []byte("Package")):
			name = string(value)
		case bytes.Equal(field, []byte("Architecture")):
			architecture = string(value)
		case bytes.Equal(field, []byte("Version")):
			version = string(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("parse package index %s: %w", indexName, err)
	}
	flush()
	return nil
}

func appendAvailablePackage(result *[]AvailablePackage, pkg AvailablePackage, filter map[string]map[string]struct{}) {
	if pkg.Name == "" || pkg.Version == "" {
		return
	}
	if pkg.Architecture == "" {
		pkg.Architecture = "all"
	}
	if filter != nil && !packageFilterAllowsArch(filter, pkg.Name, pkg.Architecture) {
		return
	}
	*result = append(*result, pkg)
}

func installedPackageFilter(installed []InstalledPackage) map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{})
	for _, pkg := range installed {
		if !isInstalledStatus(pkg.Status) {
			continue
		}
		if pkg.Name == "" {
			continue
		}
		arch := pkg.Architecture
		if arch == "" {
			arch = "all"
		}
		arches := result[pkg.Name]
		if arches == nil {
			arches = make(map[string]struct{}, 2)
			result[pkg.Name] = arches
		}
		arches[arch] = struct{}{}
		arches["all"] = struct{}{}
	}
	return result
}

func countUpgradable(installed []InstalledPackage, available []AvailablePackage, securityOnly bool) int {
	return countUpgradableByKey(installed, availableByPackageKey(available), securityOnly)
}

func countUpgradableByKey(installed []InstalledPackage, availableByKey map[packageKey][]AvailablePackage, securityOnly bool) int {
	count := 0
	for _, pkg := range installed {
		if !isInstalledStatus(pkg.Status) {
			continue
		}
		if isHeldStatus(pkg.Status) {
			continue
		}

		candidate, ok := bestUpgradeCandidate(pkg, packageCandidates(pkg, availableByKey))
		if !ok {
			continue
		}
		if securityOnly && !candidate.Security {
			continue
		}
		if CompareVersions(candidate.Version, pkg.Version) > 0 {
			count++
		}
	}
	return count
}

func countPinnedUpdates(installed []InstalledPackage, available []AvailablePackage, securityOnly bool) int {
	return countPinnedUpdatesByKey(installed, availableByPackageKey(available), securityOnly)
}

func countPinnedUpdatesByKey(installed []InstalledPackage, availableByKey map[packageKey][]AvailablePackage, securityOnly bool) int {
	count := 0
	for _, pkg := range installed {
		if !isInstalledStatus(pkg.Status) {
			continue
		}
		if hasPinnedUpdate(pkg, packageCandidates(pkg, availableByKey), securityOnly) {
			count++
		}
	}
	return count
}

func countBackportsUpdates(installed []InstalledPackage, available []AvailablePackage) int {
	return countBackportsUpdatesByKey(installed, availableByPackageKey(available))
}

func countBackportsUpdatesByKey(installed []InstalledPackage, availableByKey map[packageKey][]AvailablePackage) int {
	count := 0
	for _, pkg := range installed {
		if !isInstalledStatus(pkg.Status) {
			continue
		}
		if hasBackportsUpdate(pkg, packageCandidates(pkg, availableByKey)) {
			count++
		}
	}
	return count
}

func availableByPackageKey(available []AvailablePackage) map[packageKey][]AvailablePackage {
	result := make(map[packageKey][]AvailablePackage)
	for _, pkg := range available {
		k := packageKey{name: pkg.Name, arch: pkg.Architecture}
		result[k] = append(result[k], pkg)
	}
	return result
}

func packageCandidates(pkg InstalledPackage, available map[packageKey][]AvailablePackage) []AvailablePackage {
	candidates := available[packageKey{name: pkg.Name, arch: pkg.Architecture}]
	if len(candidates) == 0 && pkg.Architecture != "all" {
		candidates = available[packageKey{name: pkg.Name, arch: "all"}]
	}
	return candidates
}

type packageKey struct {
	name string
	arch string
}

func hasPinnedUpdate(installed InstalledPackage, available []AvailablePackage, securityOnly bool) bool {
	if installed.Version == "" {
		return false
	}
	selected := bestPolicyCandidate(installed, available)
	compareVersion := selected.Version
	if isHeldStatus(installed.Status) {
		compareVersion = installed.Version
	}
	for _, candidate := range available {
		if candidate.Backports {
			continue
		}
		if securityOnly && !candidate.Security {
			continue
		}
		if CompareVersions(candidate.Version, compareVersion) > 0 {
			return true
		}
	}
	return false
}

func hasBackportsUpdate(installed InstalledPackage, available []AvailablePackage) bool {
	if installed.Version == "" {
		return false
	}
	for _, candidate := range available {
		if !candidate.Backports {
			continue
		}
		if CompareVersions(candidate.Version, installed.Version) > 0 {
			return true
		}
	}
	return false
}

func bestUpgradeCandidate(installed InstalledPackage, available []AvailablePackage) (AvailablePackage, bool) {
	if installed.Version == "" {
		return AvailablePackage{}, false
	}

	best := bestPolicyCandidate(installed, available)
	if CompareVersions(best.Version, installed.Version) <= 0 {
		return AvailablePackage{}, false
	}
	return best, true
}

func bestPolicyCandidate(installed InstalledPackage, available []AvailablePackage) AvailablePackage {
	best := AvailablePackage{
		Name:         installed.Name,
		Architecture: installed.Architecture,
		Version:      installed.Version,
		Priority:     installedPackagePriority,
	}
	for _, candidate := range available {
		candidate.Priority = normalizedPackagePriority(candidate.Priority)
		if CompareVersions(candidate.Version, installed.Version) < 0 && candidate.Priority < forceDowngradePriority {
			continue
		}
		if isBetterCandidate(candidate, best) {
			best = candidate
			continue
		}
		if sameCandidate(candidate, best) && candidate.Security {
			best.Security = true
		}
	}
	return best
}

func isBetterCandidate(left AvailablePackage, right AvailablePackage) bool {
	if left.Priority != right.Priority {
		return left.Priority > right.Priority
	}
	return CompareVersions(left.Version, right.Version) > 0
}

func sameCandidate(left AvailablePackage, right AvailablePackage) bool {
	return left.Priority == right.Priority && CompareVersions(left.Version, right.Version) == 0
}

func normalizedPackagePriority(priority int) int {
	if priority == 0 {
		return defaultPackagePriority
	}
	return priority
}

func isInstalledStatus(status []string) bool {
	return len(status) == 3 && (status[0] == "install" || status[0] == "hold") && status[2] == "installed"
}

func isHeldStatus(status []string) bool {
	return len(status) == 3 && status[0] == "hold" && status[2] == "installed"
}

func isBrokenStatus(status []string) bool {
	if len(status) != 3 || status[0] != "install" {
		return false
	}
	switch status[2] {
	case "not-installed", "config-files", "installed":
		return false
	default:
		return true
	}
}

func isSecurityIndex(name string) bool {
	return strings.Contains(strings.ToLower(name), "security")
}

func splitParagraphs(input string) []string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n\n")
}

func forEachByteParagraph(input []byte, fn func([]byte)) {
	trimmed := bytes.TrimSpace(input)
	if len(trimmed) == 0 {
		return
	}
	for len(trimmed) > 0 {
		paragraph, rest, ok := bytes.Cut(trimmed, []byte("\n\n"))
		if len(bytes.TrimSpace(paragraph)) > 0 {
			fn(paragraph)
		}
		if !ok {
			return
		}
		trimmed = bytes.TrimLeft(rest, "\n")
	}
}

func parseAvailablePackageFields(entry []byte, filter map[string]map[string]struct{}) (name string, architecture string, version string) {
	for len(entry) > 0 {
		line, rest, hasMore := bytes.Cut(entry, []byte("\n"))
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' {
			if !hasMore {
				break
			}
			entry = rest
			continue
		}
		key, value, hasField := bytes.Cut(line, []byte(":"))
		if !hasField {
			if !hasMore {
				break
			}
			entry = rest
			continue
		}
		key = bytes.TrimSpace(key)
		switch {
		case bytes.Equal(key, []byte("Package")):
			name = string(bytes.TrimSpace(value))
			if filter != nil {
				if _, ok := filter[name]; !ok {
					return "", "", ""
				}
			}
		case bytes.Equal(key, []byte("Architecture")):
			architecture = string(bytes.TrimSpace(value))
			if filter != nil && name != "" {
				if !packageFilterAllowsArch(filter, name, architecture) {
					return "", "", ""
				}
			}
		case bytes.Equal(key, []byte("Version")):
			version = string(bytes.TrimSpace(value))
		}
		if name != "" && architecture != "" && version != "" {
			break
		}
		if !hasMore {
			break
		}
		entry = rest
	}
	return name, architecture, version
}

func packageFilterAllowsArch(filter map[string]map[string]struct{}, name string, architecture string) bool {
	arches := filter[name]
	if len(arches) == 0 {
		return false
	}
	if architecture == "" {
		architecture = "all"
	}
	_, ok := arches[architecture]
	return ok
}

func parseDeb822Fields(entry string) map[string]string {
	fields := make(map[string]string)
	var current string
	for _, line := range strings.Split(entry, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if current != "" {
				fields[current] += "\n" + strings.TrimSpace(line)
			}
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		current = strings.TrimSpace(key)
		fields[current] = strings.TrimSpace(value)
	}
	return fields
}

func CompareVersions(left string, right string) int {
	leftEpoch, leftVersion, leftRevision := splitVersion(left)
	rightEpoch, rightVersion, rightRevision := splitVersion(right)

	if leftEpoch != rightEpoch {
		if leftEpoch < rightEpoch {
			return -1
		}
		return 1
	}
	if cmp := compareVersionPart(leftVersion, rightVersion); cmp != 0 {
		return cmp
	}
	return compareVersionPart(leftRevision, rightRevision)
}

func splitVersion(version string) (int, string, string) {
	epoch := 0
	if epochPart, rest, ok := strings.Cut(version, ":"); ok {
		if parsed, err := strconv.Atoi(epochPart); err == nil {
			epoch = parsed
		}
		version = rest
	}

	revision := "0"
	if idx := strings.LastIndex(version, "-"); idx >= 0 {
		revision = version[idx+1:]
		version = version[:idx]
	}
	return epoch, version, revision
}

func compareVersionPart(left string, right string) int {
	for left != "" || right != "" {
		leftNonDigit, leftRest := takeWhile(left, func(r rune) bool { return !unicode.IsDigit(r) })
		rightNonDigit, rightRest := takeWhile(right, func(r rune) bool { return !unicode.IsDigit(r) })
		if cmp := compareNonDigit(leftNonDigit, rightNonDigit); cmp != 0 {
			return cmp
		}

		leftDigits, nextLeft := takeWhile(leftRest, func(r rune) bool { return unicode.IsDigit(r) })
		rightDigits, nextRight := takeWhile(rightRest, func(r rune) bool { return unicode.IsDigit(r) })
		if cmp := compareDigit(leftDigits, rightDigits); cmp != 0 {
			return cmp
		}

		left = nextLeft
		right = nextRight
	}
	return 0
}

func takeWhile(value string, predicate func(rune) bool) (string, string) {
	index := 0
	for _, r := range value {
		if !predicate(r) {
			break
		}
		index += len(string(r))
	}
	return value[:index], value[index:]
}

func compareNonDigit(left string, right string) int {
	for left != "" || right != "" {
		leftRune, leftSize := nextRune(left)
		rightRune, rightSize := nextRune(right)
		leftOrder := nonDigitOrder(leftRune)
		rightOrder := nonDigitOrder(rightRune)
		if leftOrder != rightOrder {
			if leftOrder < rightOrder {
				return -1
			}
			return 1
		}
		left = left[leftSize:]
		right = right[rightSize:]
	}
	return 0
}

func compareDigit(left string, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if len(left) != len(right) {
		if len(left) < len(right) {
			return -1
		}
		return 1
	}
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func nextRune(value string) (rune, int) {
	if value == "" {
		return 0, 0
	}
	for _, r := range value {
		return r, len(string(r))
	}
	return 0, 0
}

func nonDigitOrder(r rune) int {
	switch {
	case r == '~':
		return -1
	case r == 0:
		return 0
	case unicode.IsLetter(r):
		return int(r)
	default:
		return int(r) + 256
	}
}
