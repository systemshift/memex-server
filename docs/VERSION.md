# Version Compatibility Guide

Memex uses semantic versioning for both the application and repository format to ensure safe upgrades and compatibility.

## Version Types

### Application Version
- The version of the memex binaries (e.g., 1.2.3)
- Follows semantic versioning (MAJOR.MINOR.PATCH)
- Displayed with `memex --version`

### Repository Format Version
- The version of the .mx file format (e.g., 1.0)
- Uses MAJOR.MINOR format
- Stored in repository header
- Checked when opening repositories

## Compatibility Rules

When opening a repository, Memex checks:
1. Repository format version compatibility
2. Records which version of memex created the repository

Version compatibility rules:
- Major version must match exactly (e.g., 1.x can't open 2.x repositories)
- Minor version of the repository must be <= current version
- The version that created a repository is stored in its header

## Version Information

Use `memex version` to check:
- Current memex version
- Repository format version
- Which memex version created the repository

Example output:
```
Memex Version: 1.2.3
Commit: abc123
Build Date: 2024-01-01

Repository Format Version: 1.0
Created by Memex Version: 1.2.0
```

## Version History

### Repository Format Versions

- 1.0: Initial stable format
  - Content-addressable storage
  - DAG structure
  - Transaction log
  - Basic metadata

Future versions will maintain backward compatibility within the same major version.

## Upgrading Repositories

When a new version of Memex is released:

1. If only PATCH version changes:
   - No action needed
   - Full compatibility maintained

2. If MINOR version changes:
   - Repository format is compatible
   - New features may be available
   - No migration needed

3. If MAJOR version changes:
   - Repository format may be incompatible
   - Migration tool will be provided
   - Check release notes for upgrade path
