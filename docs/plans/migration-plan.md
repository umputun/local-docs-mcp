# Migration Plan: Python to Go MCP Server

## Overview

Migrate the local-docs MCP server from Python to Go, maintaining full compatibility with the Model Context Protocol while achieving better performance, type safety, and comprehensive test coverage.

## Source Reference

**Original Implementation**: `~/.dot-files/claude/local-docs-mcp.py`
- Lines of code: ~400
- Dependencies: `mcp[cli]`, Python 3.10+
- Execution: UV-based shebang script

## Goals

1. **Functional parity**: All features from Python version
2. **Type safety**: Leverage Go's type system
3. **Performance**: Faster file scanning and search
4. **Test coverage**: Minimum 80% coverage
5. **Production ready**: Proper error handling, logging, documentation

## Architecture

### Package Structure

```
local-docs-mcp/
├── cmd/
│   └── local-docs-mcp/
│       └── main.go              # entry point, server initialization
├── internal/
│   ├── server/
│   │   ├── server.go            # MCP server setup and configuration
│   │   └── server_test.go
│   ├── tools/
│   │   ├── search.go            # search_docs tool implementation
│   │   ├── search_test.go
│   │   ├── read.go              # read_doc tool implementation
│   │   ├── read_test.go
│   │   ├── list.go              # list_all_docs tool implementation
│   │   └── list_test.go
│   └── scanner/
│       ├── scanner.go           # file discovery and indexing
│       ├── scanner_test.go
│       ├── path.go              # safe path resolution
│       └── path_test.go
├── docs/
│   └── plans/
│       └── migration-plan.md    # this file
├── go.mod
├── go.sum
├── README.md
├── CLAUDE.md                    # project-specific guidance
├── Makefile
└── .gitignore
```

### Component Responsibilities

#### cmd/local-docs-mcp/main.go
- argument parsing (if any)
- server initialization
- logging setup
- stdio transport setup
- graceful shutdown handling

#### internal/server/
- MCP server creation using official SDK
- tool registration
- configuration management
- server lifecycle

#### internal/tools/
- tool implementations (search_docs, read_doc, list_all_docs)
- input/output struct definitions
- business logic for each tool
- fuzzy matching for search

#### internal/scanner/
- file discovery across multiple sources
- path safety validation
- file metadata collection
- directory walking with filtering

## Dependencies

### Required Libraries

```go
// MCP SDK
github.com/modelcontextprotocol/go-sdk/mcp

// Fuzzy matching
github.com/sahilm/fuzzy
// or
github.com/lithammer/fuzzysearch/fuzzy

// Testing
github.com/stretchr/testify

// Logging (optional, can use standard log)
github.com/go-pkgz/lgr
```

### Go Module Initialization

```bash
cd ~/dev.umputun/local-docs-mcp
go mod init github.com/umputun/local-docs-mcp
```

## Implementation Plan

### Phase 1: Project Setup (0.5 day)

#### Tasks
- [x] create directory structure
- [ ] initialize go module
- [ ] create README.md with project description
- [ ] create CLAUDE.md with Go-specific guidance
- [ ] setup .gitignore
- [ ] create Makefile with common targets
- [ ] initialize git repository

#### Deliverables
- working Go module
- basic documentation
- development tooling configured

### Phase 2: Core Scanner Implementation (1 day)

#### internal/scanner/scanner.go

**Types**:
```go
type Source string

const (
    SourceCommands     Source = "commands"
    SourceProjectDocs  Source = "project-docs"
    SourceProjectRoot  Source = "project-root"
)

type FileInfo struct {
    Name       string
    Filename   string  // with source prefix
    Normalized string  // lowercase for matching
    Source     Source
    Path       string  // absolute path
    Size       int64
}

type Scanner struct {
    commandsDir     string
    projectDocsDir  string
    projectRootDir  string
    maxFileSize     int64
}
```

**Methods**:
- `NewScanner(commandsDir, projectDocsDir, projectRootDir string) *Scanner`
- `Scan() ([]FileInfo, error)` - scan all sources
- `scanSource(source Source, dir string, glob string) ([]FileInfo, error)`

**Python equivalent**: `get_file_list()` function (lines 165-230)

#### internal/scanner/path.go

**Function**:
```go
func SafeResolvePath(baseDir, userPath string, maxSize int64) (string, error)
```

**Validation checks**:
- reject absolute paths
- add .md extension if missing
- prevent path traversal with `..`
- verify path stays within baseDir
- check file exists and size < maxSize

**Python equivalent**: `safe_resolve_path()` function (lines 122-163)

#### Tests

**scanner_test.go**:
- test scanning each source independently
- test filtering hidden files
- test excluding docs/plans/
- test handling missing directories
- test performance with large directory trees

**path_test.go**:
- test absolute path rejection
- test path traversal prevention
- test .md extension addition
- test file size validation
- test symlink handling
- test non-existent file handling

### Phase 3: Tool Implementations (1.5 days)

#### internal/tools/search.go

**Types**:
```go
type SearchInput struct {
    Query string `json:"query" jsonschema:"search query for documentation files"`
}

type SearchMatch struct {
    Path   string  `json:"path"`
    Name   string  `json:"name"`
    Score  float64 `json:"score"`
    Source string  `json:"source"`
}

type SearchOutput struct {
    Results []SearchMatch `json:"results"`
    Total   int          `json:"total"`
}
```

**Implementation**:
- normalize query (lowercase, replace spaces with hyphens)
- exact/substring matching (score 1.0)
- fuzzy matching for non-exact (threshold 0.3)
- sort by score descending
- limit to top 10 results

**Python equivalent**: `search_docs()` function (lines 232-263)

#### internal/tools/read.go

**Types**:
```go
type ReadInput struct {
    Path   string  `json:"path" jsonschema:"file path to read, can include source prefix"`
    Source *string `json:"source,omitempty" jsonschema:"optional source: commands, project-docs, or project-root"`
}

type ReadOutput struct {
    Path    string `json:"path"`
    Content string `json:"content"`
    Size    int    `json:"size"`
    Source  string `json:"source"`
}
```

**Implementation**:
- parse source prefix from path (e.g., "commands:file.md")
- try specified source first, then fallback to all sources
- use SafeResolvePath for validation
- read file content with UTF-8 validation
- return structured output

**Python equivalent**: `read_doc()` function (lines 265-352)

#### internal/tools/list.go

**Types**:
```go
type ListOutput struct {
    Docs  []DocInfo `json:"docs"`
    Total int       `json:"total"`
}

type DocInfo struct {
    Name     string `json:"name"`
    Filename string `json:"filename"`
    Source   string `json:"source"`
    Size     int64  `json:"size,omitempty"`
    TooLarge bool   `json:"too_large,omitempty"`
}
```

**Implementation**:
- call scanner.Scan() to get all files
- add size information
- mark files exceeding size limit
- return complete list

**Python equivalent**: `list_all_docs()` function (lines 354-395)

#### Tests

**search_test.go**:
- test exact match scoring
- test substring match scoring
- test fuzzy matching
- test score sorting
- test result limiting (top 10)
- test empty query handling
- test no matches scenario

**read_test.go**:
- test reading with source prefix
- test reading with source parameter
- test fallback to all sources
- test file not found
- test file too large
- test UTF-8 decode error
- test permission denied

**list_test.go**:
- test listing all sources
- test size information inclusion
- test too_large flag
- test empty directory handling

### Phase 4: MCP Server Integration (0.5 day)

#### internal/server/server.go

**Types**:
```go
type Config struct {
    CommandsDir     string
    ProjectDocsDir  string
    ProjectRootDir  string
    MaxFileSize     int64
}

type Server struct {
    config  *Config
    scanner *scanner.Scanner
    mcp     *mcp.Server
}
```

**Methods**:
- `New(config *Config) (*Server, error)` - create and configure server
- `registerTools()` - register all three tools
- `Run(ctx context.Context) error` - start server with stdio transport

**Tool registration pattern**:
```go
mcp.AddTool(s.mcp, &mcp.Tool{
    Name:        "search_docs",
    Description: "Search for documentation files matching the query",
}, s.searchDocs)
```

**Python equivalent**: MCP server initialization and tool decorators (lines 86, 232, 265, 354)

#### cmd/local-docs-mcp/main.go

**Responsibilities**:
- setup logging
- determine directories (HOME/.claude/commands, CWD/docs, CWD)
- create server config
- initialize server
- handle context cancellation
- run server

**Python equivalent**: `if __name__ == "__main__"` block (lines 397-402)

#### Tests

**server_test.go**:
- test server initialization
- test tool registration
- test invalid config handling
- integration test with mock transport

### Phase 5: Integration Tests (0.5 day)

#### Test Strategy

**Setup test fixtures**:
```
testdata/
├── commands/
│   ├── action/
│   │   └── test-action.md
│   └── knowledge/
│       └── test-knowledge.md
├── docs/
│   ├── architecture.md
│   └── plans/
│       └── excluded.md  # should be excluded
└── root/
    └── README.md
```

**Integration test scenarios**:
1. full workflow: search → read → list
2. cross-source search (finds files from all sources)
3. source-specific reads
4. path traversal attempts (security)
5. large file handling
6. concurrent tool calls
7. UTF-8 handling with various encodings

**Test execution**:
```go
func TestIntegration(t *testing.T) {
    // setup test directories
    // create server
    // execute tool calls
    // verify results
}
```

### Phase 6: Documentation & Polish (0.5 day)

#### README.md

**Contents**:
- project description
- installation instructions
- configuration in .claude.json
- usage examples
- development setup
- testing instructions
- comparison with Python version

#### CLAUDE.md

**Contents**:
- project architecture
- package responsibilities
- testing conventions
- MCP SDK usage patterns
- security considerations
- performance notes

#### Makefile

**Targets**:
```makefile
.PHONY: test
test:
	go test -race -cover ./...

.PHONY: test-verbose
test-verbose:
	go test -v -race -cover ./...

.PHONY: coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

.PHONY: lint
lint:
	golangci-lint run

.PHONY: build
build:
	go build -o bin/local-docs-mcp ./cmd/local-docs-mcp

.PHONY: install
install:
	go install ./cmd/local-docs-mcp

.PHONY: run
run:
	go run ./cmd/local-docs-mcp
```

### Phase 7: Testing & Validation (0.5 day)

#### Test Coverage Goals

- **Minimum**: 80% coverage across all packages
- **Target areas**:
  - scanner package: 85%+
  - tools package: 90%+
  - server package: 80%+

#### Validation Checklist

- [ ] all Python features implemented
- [ ] all tests passing
- [ ] coverage target met
- [ ] linter passing (golangci-lint)
- [ ] integration with Claude Code tested
- [ ] performance benchmarks run
- [ ] security review completed
- [ ] documentation reviewed

#### Manual Testing

1. **Installation test**:
   ```bash
   go install ./cmd/local-docs-mcp
   # update ~/.claude.json
   # restart Claude Code
   ```

2. **Functional test**:
   - search for known documentation
   - read specific files
   - list all docs
   - verify source prefixes work

3. **Error handling test**:
   - invalid paths
   - missing files
   - permission issues

## Implementation Details

### Fuzzy Matching Implementation

**Option 1: github.com/sahilm/fuzzy**
```go
import "github.com/sahilm/fuzzy"

matches := fuzzy.Find(query, candidates)
for _, match := range matches {
    score := 1.0 - (float64(match.Distance) / float64(len(query)))
    // use score
}
```

**Option 2: Custom ratio-based (similar to Python's SequenceMatcher)**
```go
func sequenceMatcherRatio(s1, s2 string) float64 {
    // implement Levenshtein distance or similar
    // return ratio 0.0-1.0
}
```

### Error Handling Pattern

**Consistent error responses**:
```go
type ErrorResponse struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

// tool result with error
return &mcp.CallToolResult{
    Content: []mcp.Content{
        &mcp.TextContent{
            Text: fmt.Sprintf(`{"error": {"code": "%s", "message": "%s"}}`,
                code, message),
        },
    },
}, nil
```

### Logging Pattern

```go
import "log"

// or with lgr
import "github.com/go-pkgz/lgr"

logger := lgr.New(lgr.Msec, lgr.Debug)
logger.Logf("INFO scanning %s", dir)
```

### Constants

```go
const (
    MaxFileSize      = 5 * 1024 * 1024  // 5MB
    FuzzyThreshold   = 0.3              // minimum fuzzy match score
    MaxSearchResults = 10               // top N results
)
```

## Testing Strategy

### Unit Tests

**Coverage targets**:
- each function tested independently
- table-driven tests for multiple scenarios
- error cases covered
- edge cases handled

**Example**:
```go
func TestSafeResolvePath(t *testing.T) {
    tests := []struct {
        name     string
        baseDir  string
        userPath string
        maxSize  int64
        wantErr  bool
        errMsg   string
    }{
        {
            name:     "valid relative path",
            baseDir:  "/tmp/test",
            userPath: "doc.md",
            maxSize:  1024,
            wantErr:  false,
        },
        {
            name:     "path traversal attempt",
            baseDir:  "/tmp/test",
            userPath: "../etc/passwd",
            maxSize:  1024,
            wantErr:  true,
            errMsg:   "path traversal",
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### Integration Tests

**Test file creation**:
```go
func setupTestDirs(t *testing.T) (cleanup func()) {
    tmpDir := t.TempDir()

    // create test directory structure
    commandsDir := filepath.Join(tmpDir, "commands")
    docsDir := filepath.Join(tmpDir, "docs")

    // create test files
    createTestFile(t, commandsDir, "test.md", "# Test")

    return func() {
        // cleanup if needed
    }
}
```

**MCP interaction tests**:
```go
func TestMCPToolExecution(t *testing.T) {
    // create server
    // simulate tool calls via MCP protocol
    // verify responses
}
```

### Performance Tests

**Benchmarks**:
```go
func BenchmarkScanner(b *testing.B) {
    scanner := NewScanner(commandsDir, docsDir, rootDir)
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        scanner.Scan()
    }
}

func BenchmarkSearch(b *testing.B) {
    // benchmark search performance
}
```

## Migration Verification

### Feature Parity Checklist

Python feature → Go implementation:

- [ ] list databases from all three sources (commands, project-docs, project-root)
- [ ] exclude hidden files (starting with `.`)
- [ ] exclude `docs/plans/` directory
- [ ] safe path resolution preventing traversal
- [ ] 5MB file size limit
- [ ] fuzzy search with scoring
- [ ] exact/substring match priority
- [ ] source prefix support (e.g., "commands:file.md")
- [ ] source parameter fallback
- [ ] UTF-8 validation
- [ ] proper error responses
- [ ] structured JSON output
- [ ] logging to stderr
- [ ] stdio transport

### Performance Comparison

**Python baseline**:
- scan time: measure on reference system
- search time: measure on reference system
- memory usage: measure on reference system

**Go targets**:
- scan time: < 50% of Python
- search time: < 30% of Python
- memory usage: < 60% of Python

## Deployment

### Binary Distribution

```bash
# build for current platform
go build -o local-docs-mcp ./cmd/local-docs-mcp

# install to GOPATH/bin
go install ./cmd/local-docs-mcp

# cross-compile for different platforms
GOOS=linux GOARCH=amd64 go build -o local-docs-mcp-linux ./cmd/local-docs-mcp
GOOS=darwin GOARCH=arm64 go build -o local-docs-mcp-darwin-arm64 ./cmd/local-docs-mcp
```

### Claude Configuration

**Update ~/.claude.json**:
```json
{
  "mcpServers": {
    "local-docs": {
      "command": "/path/to/local-docs-mcp"
    }
  }
}
```

**Or use installed binary**:
```json
{
  "mcpServers": {
    "local-docs": {
      "command": "local-docs-mcp"
    }
  }
}
```

## Timeline Summary

| Phase | Duration | Tasks |
|-------|----------|-------|
| 1. Project Setup | 0.5 day | directories, go.mod, docs, tooling |
| 2. Scanner | 1 day | file discovery, path safety, tests |
| 3. Tools | 1.5 days | search, read, list implementations + tests |
| 4. MCP Integration | 0.5 day | server setup, tool registration |
| 5. Integration Tests | 0.5 day | end-to-end testing |
| 6. Documentation | 0.5 day | README, CLAUDE.md, Makefile |
| 7. Validation | 0.5 day | coverage, performance, manual testing |
| **Total** | **5 days** | full migration with comprehensive tests |

## Success Criteria

1. **Functionality**: All Python features working in Go
2. **Tests**: 80%+ coverage, all tests passing
3. **Performance**: Faster than Python version
4. **Quality**: golangci-lint passing
5. **Integration**: Working with Claude Code
6. **Documentation**: Complete and accurate

## Risks & Mitigations

### Risk 1: MCP SDK API Changes
- **Mitigation**: Pin SDK version, monitor updates
- **Fallback**: Use official SDK examples as reference

### Risk 2: Fuzzy Matching Differences
- **Mitigation**: Test against Python results, adjust threshold
- **Fallback**: Implement custom matcher matching Python's behavior

### Risk 3: File System Edge Cases
- **Mitigation**: Extensive path safety tests
- **Fallback**: Conservative validation, reject suspicious paths

### Risk 4: Performance Regression
- **Mitigation**: Benchmarks, profiling
- **Fallback**: Optimize hot paths, consider caching

## Future Enhancements

Post-migration improvements:

1. **Caching**: Cache file list for faster repeated queries
2. **Watch mode**: Auto-update index on file changes
3. **Advanced search**: Support regex, multiple terms
4. **Metrics**: Expose usage statistics
5. **Configuration**: Support config file for customization
6. **Indexing**: Pre-build search index for faster queries

## References

- Original Python implementation: `~/.dot-files/claude/local-docs-mcp.py`
- Go MCP SDK: https://github.com/modelcontextprotocol/go-sdk
- MCP Protocol: https://modelcontextprotocol.io
- Project conventions: `~/.dot-files/CLAUDE.md`
