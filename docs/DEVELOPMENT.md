# Development Guide

This guide will help you set up your development environment and get started with contributing to Memex.

## Prerequisites

- Go 1.21 or later
- Git

## Development Setup

1. Clone the repository:
```bash
git clone https://github.com/yourusername/memex.git
cd memex
```

2. Install dependencies:
```bash
go mod download
```

3. Build the binaries:
```bash
# Build CLI tool
go build -o ~/bin/memex ./cmd/memex

# Build web server
go build -o ~/bin/memexd ./cmd/memexd
```

## Project Structure

```
.
├── cmd/                    # Command-line tools
│   ├── memex/             # CLI tool
│   └── memexd/            # Web server
├── internal/              # Internal packages
│   └── memex/
│       ├── core/          # Core DAG types
│       ├── storage/       # DAG storage implementation
│       ├── commands.go    # CLI commands
│       ├── config.go      # Configuration
│       └── editor.go      # Text editor
├── pkg/                   # Public API
│   └── memex/            # Client library
├── test/                  # Test files
└── docs/                  # Documentation
```

## Development Workflow

1. Create a new branch for your feature:
```bash
git checkout -b feature/your-feature-name
```

2. Make your changes and write tests

3. Run tests:
```bash
go test ./...
```

4. Format code:
```bash
go fmt ./...
```

## Running the Development Server

1. Create a repository:
```bash
memex init testrepo
```

2. Start the server:
```bash
memexd -addr :3000 -path testrepo.mx
```

3. Access the web interface:
- Open `http://localhost:3000` in your browser

## Testing

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for a specific package
go test ./internal/memex/storage/...
```

### Writing Tests

- Place tests in the test/ directory
- Test both DAG structure and content storage
- Verify link relationships
- Ensure acyclic property is maintained

Example:
```go
func TestAddNode(t *testing.T) {
    tests := []struct {
        name    string
        content []byte
        nodeType string
        meta    map[string]any
        wantErr bool
    }{
        {
            name:     "valid node",
            content:  []byte("test content"),
            nodeType: "file",
            meta:    map[string]any{"filename": "test.txt"},
            wantErr: false,
        },
        // Add more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Common Tasks

### Adding a New Node Type

1. Define type in core/types.go:
```go
// In core/types.go
const (
    NodeTypeFile = "file"
    NodeTypeNote = "note"
    NodeTypeYourType = "yourtype"
)
```

2. Add handling in storage implementation

### Adding a New Link Type

1. Define type constants:
```go
const (
    LinkTypeRef = "ref"
    LinkTypeYourType = "yourtype"
)
```

2. Update link validation if needed

### Adding a New API Endpoint

1. Add handler to cmd/memexd/main.go:
```go
func (s *Server) handleNewEndpoint(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

2. Register route in main():
```go
http.HandleFunc("/newpath", server.handleNewEndpoint)
```

## Best Practices

1. **DAG Operations**
   - Validate links to maintain acyclic property
   - Handle node versions properly
   - Maintain referential integrity

2. **Error Handling**
   - Use meaningful error messages
   - Wrap errors with context
   - Handle all error cases

3. **Documentation**
   - Document node and link types
   - Include graph structure examples
   - Keep documentation up to date

4. **Testing**
   - Test graph operations thoroughly
   - Verify DAG properties
   - Use table-driven tests

5. **Git Commits**
   - Write clear commit messages
   - Keep commits focused
   - Reference issues in commits

## Getting Help

- Check existing documentation
- Look through issues
- Ask questions in discussions
