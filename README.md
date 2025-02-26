# Memex - Personal Knowledge Graph

[![Go Report Card](https://goreportcard.com/badge/github.com/aviaryan/memex)](https://goreportcard.com/report/github.com/aviaryan/memex)
[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](LICENSE)
[![GitHub release](https://img.shields.io/github/v/release/aviaryan/memex)](https://github.com/aviaryan/memex/releases)

Memex is a graph-oriented data management tool.

## Key Features

- **DAG-Based Storage**: Content organized as nodes in a directed acyclic graph
- **Content-Addressable**: All content stored and referenced by hash for integrity and deduplication
- **Flexible Linking**: Create typed, directional relationships between content
- **Transaction System**: Cryptographic verification of all graph modifications with hash chain
- **Module System**: Extend functionality through Go packages
- **Dual Interface**: Use either CLI tool or web interface
- **Single File Storage**: All data contained in one .mx file for easy backup and portability

## Installation

### From Pre-built Binaries

Download the latest pre-built binaries for your platform from the [GitHub Releases](https://github.com/systemshift/memex/releases) page.

#### Linux and macOS
```bash
# Download and extract the archive
tar xzf memex_<OS>_<ARCH>.tar.gz

# Move binaries to your PATH
sudo mv memex /usr/local/bin/
sudo mv memexd /usr/local/bin/

# Verify installation
memex --version
memexd --version
```

#### Windows
1. Download the ZIP archive for Windows
2. Extract the contents
3. Add the extracted directory to your PATH
4. Verify installation by running `memex --version` and `memexd --version`

### Build from Source

```bash
# Build the CLI tool
go build -o ~/bin/memex ./cmd/memex

# Build the web server
go build -o ~/bin/memexd ./cmd/memexd
```

## Usage

### Command Line Interface

```bash
# Create a new repository
memex init myrepo

# Connect to existing repository
memex connect myrepo.mx

# Show repository status
memex status

# Show version information
memex version

# Add a file
memex add document.txt

# Create a note (opens editor)
memex

# Create a link between nodes
memex link <source-id> <target-id> <type> [note]

# Show links for a node
memex links <id>

# Delete a node
memex delete <id>

# Verify transaction history
memex verify

# List installed modules
memex module list

# Run a module command
memex <module-id> <command> [args...]
```

### Web Interface

Start the web server:
```bash
memexd -addr :3000 -path myrepo.mx
```

Then visit `http://localhost:3000` to access the web interface, which provides:
- Graph visualization
- File upload
- Link management
- Content search
- Node metadata viewing
- Transaction history viewing
- Module management

## Project Structure

```
.
├── cmd/                    # Command-line tools
│   ├── memex/             # CLI tool
│   └── memexd/            # Web server
├── internal/              # Internal packages
│   └── memex/
│       ├── core/          # Core types and interfaces
│       ├── storage/       # Storage implementation
│       ├── transaction/   # Transaction system
│       ├── commands.go    # CLI commands
│       ├── config.go      # Configuration
│       └── editor.go      # Text editor
├── pkg/                   # Public API
│   └── module/           # Module system
├── test/                  # Test files
└── docs/                  # Documentation
```

## Architecture

### Storage Format (.mx file)

- Fixed-size header with metadata
- Content chunks with reference counting
- Node data (DAG nodes)
- Edge data (DAG edges)
- Index for efficient lookup
- Transaction log for action history

### Node Types

- **Files**: External content added to the graph
- **Notes**: Text content created within Memex
- Each node has:
  - Unique ID
  - Content (stored as chunks)
  - Metadata
  - Links to other nodes

### Link System

- Directional relationships between nodes
- Typed links (e.g., "references", "relates-to")
- Optional metadata/notes on links
- Maintains acyclic property

### Module System

- Extend functionality through Go packages
- Standard interface for all modules
- Built-in base implementation
- Repository operations (nodes, links)
- Command system integration
- Metadata support for module data

Module commands:
```bash
# Install a module
memex module install <source>

# Remove a module
memex module remove <module-id>

# List installed modules
memex module list

# Run a module command
memex <module-id> <command> [args...]
```

### Content Storage

- Content split into chunks:
  - Small content (≤1024 bytes): Word-based chunks
  - Large content (>1024 bytes): Fixed-size chunks
- Each chunk identified by SHA-256 hash
- Reference counting for chunk management
- Automatic content deduplication
- Similar content detection through shared chunks

### Transaction System

- Cryptographic verification of all graph modifications
- Hash chain of actions (like Git commits)
- State consistency validation
- Support for future branching/merging
- Audit trail of all changes

## Development

### Building

```bash
# Get dependencies
go mod download

# Build everything
go build ./...

# Run tests
go test ./...
```

### Creating Modules

See [Module Guide](docs/MODULE.md) for detailed instructions on creating modules.

Quick example:
```go
package myplugin

import "memex/pkg/module"

type MyModule struct {
    *module.Base
}

func New() module.Module {
    return &MyModule{
        Base: module.NewBase("mymodule", "My Module", "Description"),
    }
}
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for a specific package
go test ./internal/memex/storage/...
```

## Documentation

- [API Documentation](docs/API.md): HTTP API endpoints and usage
- [Design Document](docs/DESIGN.md): Architecture and design decisions
- [Development Guide](docs/DEVELOPMENT.md): Setup and contribution guidelines
- [Module Guide](docs/MODULE.md): Creating and using modules
- [Storage Implementation](docs/STORAGE.md): Detailed explanation of the storage system
- [Transaction System](docs/TRANSACTION.md): Graph modification tracking and verification
- [Migration Guide](docs/MIGRATION.md): Graph import/export and content migration

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Future Enhancements

- ~~Graph import/export capabilities~~ ✓
- ~~Subgraph selection and migration~~ ✓
- Advanced graph queries
- Content versioning
- Collaborative features
- Export/import to other formats
- Graph visualization improvements
- Search enhancements
- Remote graph synchronization
- Smarter chunking algorithms
- Similarity detection tuning
- Transaction branching and merging
- Distributed verification
- Time travel through graph history
- Additional module types

## License

This project is licensed under the BSD 3-Clause License - see the [LICENSE](LICENSE) file for details.
