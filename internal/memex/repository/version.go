package repository

// VersionInfo contains version information for a repository
type VersionInfo struct {
	FormatVersion uint8
	FormatMinor   uint8
	MemexVersion  string
}

// GetVersionInfo returns version information for the repository
func (r *Repository) GetVersionInfo() VersionInfo {
	return VersionInfo{
		FormatVersion: r.header.FormatVersion,
		FormatMinor:   r.header.FormatMinor,
		MemexVersion:  string(r.header.MemexVersion[:]),
	}
}
