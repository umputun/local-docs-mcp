# Quick Start Guide

## Project Setup (5 minutes)

```bash
cd ~/dev.umputun/local-docs-mcp

# initialize go module
go mod init github.com/umputun/local-docs-mcp

# add dependencies
go get github.com/modelcontextprotocol/go-sdk/mcp
go get github.com/sahilm/fuzzy
go get github.com/stretchr/testify

# verify
go mod tidy
```

## Implementation Order

### 1. Scanner Package (Day 1)
**Files**: `internal/scanner/path.go`, `internal/scanner/scanner.go`

```bash
# create test fixtures first
mkdir -p testdata/{commands/action,docs/plans,root}
echo "# Test" > testdata/commands/action/test.md
echo "# Docs" > testdata/docs/architecture.md
echo "# README" > testdata/README.md

# implement + test
touch internal/scanner/path.go internal/scanner/path_test.go
touch internal/scanner/scanner.go internal/scanner/scanner_test.go
```

**Key functions**:
- `SafeResolvePath(baseDir, userPath, maxSize) (string, error)`
- `NewScanner(commandsDir, projectDocsDir, projectRootDir) *Scanner`
- `Scan() ([]FileInfo, error)`

### 2. Tools Package (Day 2-3)
**Files**: `internal/tools/search.go`, `internal/tools/read.go`, `internal/tools/list.go`

```bash
touch internal/tools/{search,read,list}.go
touch internal/tools/{search,read,list}_test.go
```

**Implementation order**:
1. list.go (simplest - just calls scanner)
2. read.go (path resolution + file reading)
3. search.go (fuzzy matching logic)

### 3. Server Package (Day 3-4)
**Files**: `internal/server/server.go`, `cmd/local-docs-mcp/main.go`

```bash
touch internal/server/server.go internal/server/server_test.go
touch cmd/local-docs-mcp/main.go
```

### 4. Integration Tests (Day 4)
**Files**: `internal/server/integration_test.go`

### 5. Documentation & Polish (Day 5)
- complete README
- verify all tests pass
- run linter
- performance benchmarks

## Testing at Each Step

```bash
# after each file
make test

# check coverage
make coverage

# run linter
make lint
```

## Daily Checklist

**Day 1** (Scanner):
- [ ] SafeResolvePath with tests
- [ ] Scanner.Scan() with tests
- [ ] Coverage > 85%

**Day 2** (Tools 1/2):
- [ ] list_all_docs implementation + tests
- [ ] read_doc implementation + tests
- [ ] Coverage > 90%

**Day 3** (Tools 2/2 + Server):
- [ ] search_docs implementation + tests
- [ ] Server setup with tool registration
- [ ] Basic integration test

**Day 4** (Integration):
- [ ] Full integration tests
- [ ] Security tests (path traversal)
- [ ] Concurrent access tests

**Day 5** (Polish):
- [ ] All coverage targets met
- [ ] Linter passing
- [ ] Manual testing with Claude Code
- [ ] Performance benchmarks

## Key Types Reference

```go
// scanner package
type FileInfo struct {
    Name       string
    Filename   string  // with source prefix
    Normalized string
    Source     Source
    Path       string
    Size       int64
}

// tools package
type SearchInput struct {
    Query string `json:"query"`
}

type ReadInput struct {
    Path   string  `json:"path"`
    Source *string `json:"source,omitempty"`
}

type ListOutput struct {
    Docs  []DocInfo `json:"docs"`
    Total int       `json:"total"`
}
```

## Constants

```go
const (
    MaxFileSize      = 5 * 1024 * 1024  // 5MB
    FuzzyThreshold   = 0.3
    MaxSearchResults = 10
)

type Source string
const (
    SourceCommands     Source = "commands"
    SourceProjectDocs  Source = "project-docs"
    SourceProjectRoot  Source = "project-root"
)
```

## Testing Template

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:    "description",
            input:   InputType{},
            want:    OutputType{},
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

## Quick Commands

```bash
# build and test
make build && make test

# test specific package
go test ./internal/scanner/...

# test specific function
go test -run TestSafeResolvePath ./internal/scanner/

# benchmark
go test -bench=BenchmarkScan ./internal/scanner/

# coverage for package
go test -cover ./internal/scanner/

# install locally
make install

# run
local-docs-mcp
```

## Debug MCP Server

```bash
# run with verbose logging
MCP_DEBUG=1 local-docs-mcp

# test with simple request
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | local-docs-mcp
```

## Performance Targets

| Metric | Python | Go Target |
|--------|--------|-----------|
| Scan 1000 files | ~100ms | < 50ms |
| Search query | ~20ms | < 10ms |
| Startup | ~200ms | < 10ms |

## Common Gotchas

1. **Path separators**: Use `filepath.Join()` not string concatenation
2. **Test cleanup**: Use `t.TempDir()` for auto-cleanup
3. **Source prefix**: Remember to strip "commands:" before file operations
4. **Hidden files**: Start with `.` - must exclude
5. **Plans directory**: Must exclude `docs/plans/`

## Reference Files

- Full plan: `docs/plans/migration-plan.md`
- Architecture: `CLAUDE.md`
- Python original: `~/.dot-files/claude/local-docs-mcp.py`
