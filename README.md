# Local Docs MCP Server (Go)

Go implementation of the Model Context Protocol (MCP) server for local documentation access. Provides Claude with seamless access to markdown documentation from multiple sources.

## Status

🚧 **In Development** - Migration from Python version in progress

See [Migration Plan](docs/plans/migration-plan.md) for detailed implementation roadmap.

## Features

- **Multi-source documentation**: Access docs from commands, project docs, and project root
- **Smart search**: Fuzzy matching with exact/substring match priority
- **Safe path handling**: Prevents directory traversal and validates paths
- **Source prefixes**: Explicitly specify documentation source (e.g., `commands:file.md`)
- **Size limits**: Prevents reading files larger than 5MB

## Documentation Sources

1. **Commands** (`~/.claude/commands/**/*.md`): User commands and knowledge bases
2. **Project Docs** (`$CWD/docs/**/*.md`): Project-specific documentation (excludes `plans/`)
3. **Project Root** (`$CWD/*.md`): Root-level docs like README.md, CONTRIBUTING.md

## Installation

### Prerequisites

- Go 1.21 or later
- Claude Code

### Build from Source

```bash
git clone <repository-url>
cd local-docs-mcp
make build
```

### Install

```bash
make install
# Or manually:
go install ./cmd/local-docs-mcp
```

### Configure Claude

Add to `~/.claude.json`:

```json
{
  "mcpServers": {
    "local-docs": {
      "command": "local-docs-mcp"
    }
  }
}
```

Or use absolute path:

```json
{
  "mcpServers": {
    "local-docs": {
      "command": "/path/to/local-docs-mcp"
    }
  }
}
```

Restart Claude Code to load the server.

## Usage

Once configured, Claude can query documentation naturally:

- "Show me docs for routegroup"
- "Find documentation about testing"
- "List all available commands"
- "What's in the go-architect command?"

## Available Tools

### search_docs

Search for documentation files by name with fuzzy matching.

**Input**: `{"query": "search-term"}`

**Output**: Top 10 matching files with scores

### read_doc

Read a specific documentation file.

**Input**: `{"path": "file.md"}` or `{"path": "commands:action/commit.md"}`

**Output**: File content with metadata

### list_all_docs

List all available documentation files from all sources.

**Output**: Complete file listing with sizes and source information

## Development

### Setup

```bash
# clone repository
git clone <repository-url>
cd local-docs-mcp

# install dependencies
go mod download

# run tests
make test
```

### Project Structure

```
local-docs-mcp/
├── cmd/local-docs-mcp/  # entry point
├── internal/
│   ├── server/          # MCP server setup
│   ├── tools/           # tool implementations
│   └── scanner/         # file discovery and indexing
├── docs/
│   └── plans/           # implementation plans
└── testdata/            # test fixtures
```

### Testing

```bash
# run all tests
make test

# verbose output
make test-verbose

# coverage report
make coverage

# run linter
make lint
```

### Test Coverage Goals

- Minimum: 80% across all packages
- scanner package: 85%+
- tools package: 90%+
- server package: 80%+

## Comparison with Python Version

| Aspect | Python | Go |
|--------|--------|-----|
| Performance | Baseline | 2-3x faster (target) |
| Type Safety | Runtime | Compile-time |
| Dependencies | mcp[cli], UV | Official Go SDK |
| Binary Size | N/A | ~8-10MB |
| Startup Time | ~200ms | ~10ms (target) |
| Memory Usage | Baseline | ~40% reduction (target) |

## Security

- Path traversal prevention
- File size limits (5MB)
- UTF-8 validation
- No symlink following outside base directories
- Absolute path rejection

## Original Implementation

This is a Go port of the Python implementation at `~/.dot-files/claude/local-docs-mcp.py`.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.
