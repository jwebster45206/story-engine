# Story Engine Development Guide

## Build/Test Commands
```bash
# Build all packages
go build -v ./...

# Run all tests with race detection and coverage
go test -v -race -coverprofile=coverage.out ./...

# Run single test file
go test -v ./internal/handlers -run TestChatHandler

# Run single test function
go test -v ./internal/handlers -run TestChatHandler_ServeHTTP

# Run tests without integration tests
go test -short ./...

# Lint code
golangci-lint run --timeout=5m

# Download and verify dependencies
go mod download && go mod verify
```

## Code Style Guidelines
- **Imports**: Standard library first, then third-party, then local packages with blank lines between groups
- **Types**: Use struct tags for JSON serialization (`json:"field_name,omitempty"`)
- **Naming**: Use camelCase for JSON fields, PascalCase for exported Go identifiers
- **Error handling**: Always check errors, use `fmt.Errorf` for wrapping with context
- **Logging**: Use structured logging with `slog.Logger`, include context in log messages
- **Testing**: Use testify/assert, create table-driven tests, skip integration tests with `testing.Short()`
- **Context**: Always pass context.Context as first parameter for operations that may block
- **Interfaces**: Keep interfaces small and focused (e.g., `LLMService`, `Storage`)
- **Comments**: Document exported functions and types, avoid obvious comments