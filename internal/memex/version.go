package memex

// Version information set by goreleaser
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Version returns the current version of memex
func Version() string {
	return version
}

// BuildInfo returns detailed build information
func BuildInfo() string {
	return "Version: " + version + "\nCommit: " + commit + "\nBuild Date: " + date
}
