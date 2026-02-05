package updater

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version with optional pre-release suffix.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Raw        string
}

var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9.-]+))?$`)

// ParseVersion parses a semantic version string.
// Accepts formats: "1.2.3", "v1.2.3", "1.2.3-beta.1".
func ParseVersion(versionStr string) (*Version, error) {
	matches := semverRegex.FindStringSubmatch(versionStr)
	if matches == nil {
		return nil, fmt.Errorf("invalid version format: %s", versionStr)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	preRelease := ""
	if len(matches) > 4 {
		preRelease = matches[4]
	}

	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
		Raw:        versionStr,
	}, nil
}

// String returns the version as a string in "vX.Y.Z" format.
func (v *Version) String() string {
	s := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		s += "-" + v.PreRelease
	}

	return s
}

// Compare compares two versions.
// Returns:
//
//	-1 if v < other
//	 0 if v == other
//	 1 if v > other
func (v *Version) Compare(other *Version) int {
	// Compare major version
	if cmp := compareInts(v.Major, other.Major); cmp != 0 {
		return cmp
	}

	// Compare minor version
	if cmp := compareInts(v.Minor, other.Minor); cmp != 0 {
		return cmp
	}

	// Compare patch version
	if cmp := compareInts(v.Patch, other.Patch); cmp != 0 {
		return cmp
	}

	// Handle pre-release comparison
	return comparePreReleaseStrings(v.PreRelease, other.PreRelease)
}

// compareInts returns -1 if a < b, 1 if a > b, 0 if equal.
func compareInts(first, second int) int {
	switch {
	case first < second:
		return -1
	case first > second:
		return 1
	default:
		return 0
	}
}

// comparePreReleaseStrings handles pre-release comparison logic.
// Pre-release versions have lower precedence than release versions.
func comparePreReleaseStrings(first, second string) int {
	switch {
	case first == "" && second != "":
		// Release > pre-release
		return 1
	case first != "" && second == "":
		// Pre-release < release
		return -1
	case first != "" && second != "":
		return comparePreRelease(first, second)
	default:
		return 0
	}
}

// LessThan returns true if v < other.
func (v *Version) LessThan(other *Version) bool {
	return v.Compare(other) < 0
}

// GreaterThan returns true if v > other.
func (v *Version) GreaterThan(other *Version) bool {
	return v.Compare(other) > 0
}

// Equal returns true if v == other.
func (v *Version) Equal(other *Version) bool {
	return v.Compare(other) == 0
}

// IsPreRelease returns true if the version has a pre-release tag (e.g., beta.1, alpha).
func (v *Version) IsPreRelease() bool {
	return v.PreRelease != ""
}

// InferredChannel returns the likely channel based on the version's pre-release tag.
// Returns "stable" for release versions, or extracts the channel identifier from
// the pre-release tag (e.g., "beta" from "beta.1" or "beta1").
// Versions with "custom-" prefix always return "custom" channel.
func (v *Version) InferredChannel() string {
	if v.PreRelease == "" {
		return "stable"
	}

	prerelease := strings.ToLower(v.PreRelease)

	// Custom uploads are prefixed with "custom-" and should only match custom channel
	if strings.HasPrefix(prerelease, "custom") {
		return "custom"
	}

	// Check for known channel prefixes
	knownChannels := []string{"alpha", "beta", "dev", "rc", "nightly"}
	for _, channel := range knownChannels {
		if strings.HasPrefix(prerelease, channel) {
			return channel
		}
	}

	// Fallback: extract the first part before any dot
	parts := strings.Split(prerelease, ".")
	if len(parts) > 0 {
		return parts[0]
	}

	return "unknown"
}

// comparePreRelease compares pre-release identifiers.
// Follows semver spec: identifiers are compared left to right.
func comparePreRelease(first, second string) int {
	partsA := strings.Split(first, ".")
	partsB := strings.Split(second, ".")

	for idx := 0; idx < len(partsA) && idx < len(partsB); idx++ {
		if cmp := comparePreReleasePart(partsA[idx], partsB[idx]); cmp != 0 {
			return cmp
		}
	}

	// Fewer identifiers = lower precedence
	return compareInts(len(partsA), len(partsB))
}

// comparePreReleasePart compares a single pre-release identifier part.
func comparePreReleasePart(partA, partB string) int {
	numA, errA := strconv.Atoi(partA)
	numB, errB := strconv.Atoi(partB)

	switch {
	case errA == nil && errB == nil:
		// Both are numeric
		return compareInts(numA, numB)
	case errA == nil:
		// Numeric identifiers have lower precedence than alphanumeric
		return -1
	case errB == nil:
		return 1
	default:
		// Both are alphanumeric, compare as strings
		return strings.Compare(partA, partB)
	}
}
