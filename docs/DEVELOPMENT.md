# Development Guide

This guide will help you set up your development environment and get started with contributing to Memex.

## Prerequisites

- Go 1.21 or later
- Git
- Make (optional, for build scripts)

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
│       ├── core/          # Core types and interfaces
│       └── storage/       # Storage implementation
├── web/                   # Web interface
│   ├── handlers/          # HTTP handlers
│   ├── static/            # Static assets
│   └── templates/         # HTML templates
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

4. Run linter:
```bash
go vet ./...
```

5. Format code:
```bash
go fmt ./...
```

## Running the Development Server

1. Start the server:
```bash
memexd -port 8080
```

2. Access the web interface:
- Open `http://localhost:8080` in your browser
- API endpoints are available at `http://localhost:8080/api/`

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

- Place tests in the same directory as the code they test
- Use table-driven tests when appropriate
- Test both success and error cases
- Use meaningful test names

Example:
```go
func TestAddContent(t *testing.T) {
    tests := []struct {
        name    string
        content []byte
        wantErr bool
    }{
        {
            name:    "valid content",
            content: []byte("test content"),
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

## Debugging

### Web Server

1. Use logging:
```go
log.Printf("Debug: %v", someValue)
```

2. Check server logs:
```bash
memexd -port 8080 2>&1 | tee server.log
```

### CLI Tool

1. Enable verbose output:
```bash
memex -verbose command
```

## Common Tasks

### Adding a New Command

1. Create command file in `cmd/memex/`:
```go
func newYourCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "yourcommand",
        Short: "Short description",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Implementation
            return nil
        },
    }
}
```

2. Register in `cmd/memex/main.go`

### Adding a New API Endpoint

1. Add handler in `web/handlers/handlers.go`:
```go
func (h *Handler) HandleNewEndpoint(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

2. Register route in `cmd/memexd/main.go`

## Best Practices

1. **Code Organization**
   - Keep packages focused and cohesive
   - Use meaningful package names
   - Follow standard Go project layout

2. **Error Handling**
   - Use meaningful error messages
   - Wrap errors with context
   - Handle all error cases

3. **Documentation**
   - Document all exported functions
   - Include examples in documentation
   - Keep documentation up to date

4. **Testing**
   - Write tests for new code
   - Maintain test coverage
   - Use table-driven tests

5. **Git Commits**
   - Write clear commit messages
   - Keep commits focused
   - Reference issues in commits

## Getting Help

- Check existing documentation
- Look through issues
- Ask questions in discussions
- Join developer chat
