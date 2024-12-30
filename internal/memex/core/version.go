package core

import (
	"fmt"
	"strconv"
	"strings"
)

// RepositoryVersion represents the version of a repository
type RepositoryVersion struct {
	Major uint8 // Major version for incompatible changes
	Minor uint8 // Minor version for backwards-compatible changes
}

// String returns the string representation of the version
func (v RepositoryVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// ParseVersion parses a version string into a RepositoryVersion
func ParseVersion(version string) (RepositoryVersion, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 2 {
		return RepositoryVersion{}, fmt.Errorf("invalid version format: %s", version)
	}

	major, err := strconv.ParseUint(parts[0], 10, 8)
	if err != nil {
		return RepositoryVersion{}, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.ParseUint(parts[1], 10, 8)
	if err != nil {
		return RepositoryVersion{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	return RepositoryVersion{
		Major: uint8(major),
		Minor: uint8(minor),
	}, nil
}

// IsCompatible checks if the given version is compatible with this version
func (v RepositoryVersion) IsCompatible(other RepositoryVersion) bool {
	// Major version must match exactly
	// Minor version of repository must be <= current version
	return v.Major == other.Major && v.Minor >= other.Minor
}

// CurrentVersion is the current repository format version
var CurrentVersion = RepositoryVersion{
	Major: 1,
	Minor: 0,
}

// MinimumVersion is the minimum supported repository format version
var MinimumVersion = RepositoryVersion{
	Major: 1,
	Minor: 0,
}
