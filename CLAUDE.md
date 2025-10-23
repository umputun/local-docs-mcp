# Local Docs MCP Server - Development Guide

## Project Overview

Go implementation of Model Context Protocol (MCP) server providing Claude with access to local markdown documentation from multiple sources.

**Original implementation**: `~/.dot-files/claude/local-docs-mcp.py`

## Architecture

### Package Responsibilities

#### cmd/local-docs-mcp
- entry point and initialization
- logging setup
- server lifecycle management
- stdio transport configuration

#### internal/server
- MCP server creation and configuration
- tool registration with official SDK
- server startup and shutdown

#### internal/tools
- tool implementations (search_docs, read_doc, list_all_docs)
- input/output type definitions
- fuzzy matching logic
- business logic isolated from MCP protocol

#### internal/scanner
- file discovery across sources (commands, project-docs, project-root)
- safe path resolution preventing traversal
- file metadata collection (size, source)
- directory walking with filtering rules

## Design Principles

### Type Safety
- use structs with JSON tags for all tool inputs/outputs
- leverage Go's type system for compile-time safety
- no interface{} except where SDK requires it

### Error Handling
- return errors to caller, never panic
- wrap errors with context using fmt.Errorf
- consistent error response format for MCP tools

### Testing
- minimum 80% coverage
- table-driven tests for all functions
- integration tests with real file fixtures
- separate unit tests from integration tests

### Security
- safe path resolution mandatory for all user inputs
- no symlink following outside base directories
- reject absolute paths
- validate file sizes before reading
- UTF-8 validation on file content

## Dependencies

### Core
- `github.com/modelcontextprotocol/go-sdk/mcp` - official MCP SDK

### Utilities
- `github.com/sahilm/fuzzy` or `github.com/lithammer/fuzzysearch/fuzzy` - fuzzy matching
- `github.com/stretchr/testify` - testing assertions

### Optional
- `github.com/go-pkgz/lgr` - logging (or use standard log)

## Development Workflow

### Before Starting Work
1. check existing tests for patterns
2. follow migration plan in docs/plans/migration-plan.md
3. ensure go.mod is up to date

### Adding New Features
1. write tests first (TDD)
2. implement minimal code to pass tests
3. refactor for readability
4. update documentation

### Code Style
- follow standard Go conventions
- use gofmt and goimports
- keep functions under 60 lines when possible
- lowercase comments except godoc for exported items
- no emojis in code or commits

### Testing Requirements
- one test file per source file (scanner.go → scanner_test.go)
- test both success and error paths
- use testify for assertions
- table-driven tests for multiple scenarios
- integration tests in separate files (*_integration_test.go)

### Pre-commit Checklist
- [ ] tests pass: `make test`
- [ ] coverage acceptable: `make coverage`
- [ ] linter passes: `make lint`
- [ ] code formatted: `make fmt`
- [ ] documentation updated

## MCP SDK Usage Patterns

### Tool Registration
```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "tool_name",
    Description: "description",
}, handlerFunc)
```

### Handler Signature
```go
func handlerFunc(ctx context.Context, req *mcp.CallToolRequest, input InputType)
    (*mcp.CallToolResult, OutputType, error) {
    // implementation
}
```

### Input/Output Types
```go
type InputType struct {
    Field string `json:"field" jsonschema:"field description"`
}

type OutputType struct {
    Result string `json:"result"`
}
```

## File Structure Conventions

### Source Prefixes
- `commands:path/to/file.md` - from ~/.claude/commands
- `project-docs:path/to/file.md` - from $CWD/docs (excluding plans/)
- `project-root:file.md` - from $CWD/*.md

### Directory Exclusions
- hidden files (starting with `.`)
- `docs/plans/` directory
- vendor directories (if present)

### Size Limits
- maximum file size: 5MB
- files exceeding limit are listed but cannot be read
- marked with `too_large: true` in listings

## Performance Targets

### Compared to Python Version
- scan time: < 50% of Python
- search time: < 30% of Python
- memory usage: < 60% of Python
- startup time: < 10ms

### Optimization Strategies
- efficient directory walking (filepath.WalkDir)
- minimize allocations in hot paths
- consider caching file list (future enhancement)
- lazy loading where appropriate

## Testing Strategy

### Unit Tests
- test each package independently
- mock external dependencies
- table-driven for multiple scenarios
- cover edge cases and errors

### Integration Tests
- use real file fixtures in testdata/
- test full tool execution flow
- verify MCP protocol compliance
- test concurrent access

### Test Data Organization
```
testdata/
├── commands/
│   └── action/
│       └── test.md
├── docs/
│   ├── architecture.md
│   └── plans/
│       └── excluded.md  # should be excluded
└── README.md
```

## Common Patterns

### Safe Path Resolution
```go
resolved, err := SafeResolvePath(baseDir, userPath, maxSize)
if err != nil {
    return nil, fmt.Errorf("invalid path: %w", err)
}
```

### Directory Walking
```go
filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
    if err != nil {
        return err
    }
    // process file
    return nil
})
```

### Fuzzy Matching
```go
score := fuzzy.RankMatch(query, candidate)
if score > threshold {
    // include in results
}
```

## Troubleshooting

### Tests Failing
- check test data setup in testdata/
- verify file permissions
- ensure cleanup in test teardown
- check for race conditions with -race flag

### Coverage Too Low
- add tests for error paths
- test edge cases
- add integration tests
- check for untested helper functions

### Linter Errors
- run `make lint` to see issues
- fix all issues, don't disable checks
- follow golangci-lint suggestions

### Performance Issues
- run benchmarks: `make bench`
- use profiling: `go test -cpuprofile=cpu.prof`
- check for unnecessary allocations
- optimize hot paths identified by profiler

## References

- Migration plan: docs/plans/migration-plan.md
- Python implementation: ~/.dot-files/claude/local-docs-mcp.py
- MCP SDK: https://github.com/modelcontextprotocol/go-sdk
- Global guidelines: ~/.dot-files/CLAUDE.md
