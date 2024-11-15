# Memex - Content-Addressable Storage System

Memex is a modern content-addressable storage system inspired by Git's internal design but with a focus on flexible content organization and relationship tracking. Unlike traditional hierarchical storage systems, Memex allows for more flexible organization through content linking and metadata.

## Key Design Choices

- **Git-Inspired Storage**: Uses content-addressable storage similar to Git, but without requiring a strict tree structure
- **Filesystem-Based**: Stores content directly in the filesystem for simplicity and reliability
- **Multiple Independent Graphs**: Supports multiple relationship graphs between content, more flexible than a pure Merkle tree
- **Simple File-Based Storage**: Uses straightforward file storage rather than a separate graph database
- **Metadata Separation**: Keeps metadata separate from content for flexibility

## Project Structure

```
.vault/              # Hidden storage directory (like .git)
    objects/         # Content storage
        ab/
            cd1234...
    refs/           # References/graphs
    trees/          # Structure definitions
```

## Features

- **Content-Addressable Storage**: All content is stored and referenced by its hash, ensuring data integrity and deduplication
- **Flexible Organization**: Create arbitrary relationships between content pieces
- **Relationship Tracking**: Build knowledge graphs through content linking
- **Dual Interface**: Use either the CLI tool or web interface
- **Search Capabilities**: Find content based on metadata, content type, or relationships

## Components

- `memex`: CLI tool for content management
- `memexd`: Web server providing HTTP API and web interface
- Content storage with metadata support
- Link system for tracking relationships between content

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
# Add content
memex add <file>

# Show content details
memex show <id>

# Create a link between content
memex link <source-id> <target-id> --type <link-type>

# Search content
memex search <query>
```

### Web Interface

Start the web server:
```bash
memexd -port 8080
```

Then visit `http://localhost:8080` in your browser to access the web interface.

The web interface provides:
- Content browsing and visualization
- File upload and content creation
- Link management
- Search interface

## Architecture

### Storage Layer

The storage system is designed to be:
- Content-addressable for deduplication and integrity
- Filesystem-based for simplicity and reliability
- Metadata-aware for flexible organization
- Link-enabled for relationship tracking

### Core Components

- `storage`: Handles content storage and retrieval
- `core`: Core types and interfaces
- `web/handlers`: Web interface request handlers
- `web/templates`: HTML templates for web interface
- `web/static`: Static assets (CSS, JavaScript)

## Development

### Project Structure

```
.
├── cmd/
│   ├── memex/    # CLI tool
│   └── memexd/   # Web server
├── internal/
│   └── memex/
│       ├── core/     # Core types
│       └── storage/  # Storage implementation
├── web/
│   ├── handlers/   # HTTP handlers
│   ├── static/     # Static assets
│   └── templates/  # HTML templates
└── test/           # Test files
```

### Implementation Details

- Uses filesystem for storage rather than a database
- Content is stored in content-addressable format
- Metadata and links are stored separately
- Multiple independent graphs are possible
- Simple file-based approach for initial implementation

### Running Tests

```bash
go test ./...
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Future Enhancements

- Enhanced graph querying capabilities
- More sophisticated relationship types
- Advanced metadata indexing
- Content versioning
- Collaborative features

## License

This project is licensed under the BSD 3-Clause License - see the [LICENSE](LICENSE) file for details.
