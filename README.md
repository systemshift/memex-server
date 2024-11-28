# Memex - Personal Knowledge Graph

A personal knowledge management system that helps you organize, connect, and explore your digital content through meaningful relationships.

## Key Features

- **DAG-Based Storage**: Content organized as nodes in a directed acyclic graph
- **Content-Addressable**: All content stored and referenced by hash for integrity and deduplication
- **Flexible Linking**: Create typed, directional relationships between content
- **Dual Interface**: Use either CLI tool or web interface
- **Single File Storage**: All data contained in one .mx file for easy backup and portability

## Installation

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

## Project Structure

```
.
├── cmd/                    # Command-line tools
│   ├── memex/             # CLI tool
│   └── memexd/            # Web server
├── internal/              # Internal packages
│   └── memex/
│       ├── core/          # Core types
│       ├── storage/       # Storage implementation
│       ├── commands.go    # CLI commands
│       ├── config.go      # Configuration
│       └── editor.go      # Text editor
├── pkg/                   # Public API
│   └── memex/            # Client library
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

### Content Storage

- Content split into chunks:
  - Small content (≤1024 bytes): Word-based chunks
  - Large content (>1024 bytes): Fixed-size chunks
- Each chunk identified by SHA-256 hash
- Reference counting for chunk management
- Automatic content deduplication
- Similar content detection through shared chunks

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
- [Storage Implementation](docs/STORAGE.md): Detailed explanation of the storage system
- [Migration Guide](docs/MIGRATION.md): Graph import/export and content migration

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Future Enhancements

- Graph import/export capabilities
- Subgraph selection and migration
- Advanced graph queries
- Content versioning
- Collaborative features
- Export/import to other formats
- Graph visualization improvements
- Search enhancements
- Remote graph synchronization
- Smarter chunking algorithms
- Similarity detection tuning

## License

This project is licensed under the BSD 3-Clause License - see the [LICENSE](LICENSE) file for details.
