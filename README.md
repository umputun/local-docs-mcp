# Local Docs MCP Server [![build](https://github.com/umputun/local-docs-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/umputun/local-docs-mcp/actions/workflows/ci.yml) &nbsp;[![Coverage Status](https://coveralls.io/repos/github/umputun/local-docs-mcp/badge.svg?branch=master)](https://coveralls.io/github/umputun/local-docs-mcp?branch=master)

Implementation of the Model Context Protocol (MCP) server for local documentation access. Provides Claude with seamless access to markdown documentation from multiple sources.

## Features

- **Multi-source documentation**: Access docs from commands, project docs, and project root
- **Smart search**: Fuzzy matching with exact/substring match priority
- **Optional caching**: File list caching with automatic invalidation on changes (~3000x faster)
- **File watching**: Automatic cache invalidation when documentation files change
- **Safe path handling**: Prevents directory traversal and validates paths
- **Source prefixes**: Explicitly specify documentation source (e.g., `commands:file.md`)
- **Size limits**: Prevents reading files larger than 5MB

## Documentation Sources

1. **Shared Docs** (`~/.claude/commands/**/*.md`): User commands and knowledge bases (configurable)
2. **Project Docs** (`$CWD/docs/**/*.md`): Project-specific documentation with configurable exclusions
3. **Project Root** (`$CWD/*.md`): Root-level docs like README.md, CONTRIBUTING.md (opt-in)

## Installation

### Download Binary

Download the latest release from the [releases page](https://github.com/umputun/local-docs-mcp/releases).

### Using Go

```bash
go install github.com/umputun/local-docs-mcp/app@latest
```

### Using Homebrew (macOS)

```bash
brew tap umputun/apps
brew install umputun/apps/local-docs-mcp
```

### Build from Source

```bash
git clone https://github.com/umputun/local-docs-mcp.git
cd local-docs-mcp
make build
make install
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

## Configuration

### Command Line Options

```bash
# customize documentation directories
local-docs-mcp --shared-docs-dir=~/.my-docs --docs-dir=documentation

# enable root-level markdown scanning
local-docs-mcp --enable-root-docs

# exclude directories from project docs scan
local-docs-mcp --exclude-dir=plans --exclude-dir=drafts

# multiple exclusions via environment variable
EXCLUDE_DIRS=plans,drafts,archive local-docs-mcp
```

Available options:
- `--shared-docs-dir` - shared documentation directory (default: `~/.claude/commands`)
- `--docs-dir` - project docs directory (default: `docs`)
- `--enable-root-docs` - scan root-level `*.md` files (default: disabled)
- `--exclude-dir` - directories to exclude from project docs scan (default: `plans`)
- `--enable-cache` - enable file list caching
- `--cache-ttl` - cache time-to-live (default: `1h`)

### Caching (Optional)

Enable file list caching for significantly faster repeated queries:

```bash
# enable with 1h TTL (default)
local-docs-mcp --enable-cache

# custom TTL
local-docs-mcp --enable-cache --cache-ttl=30m

# via environment variables
ENABLE_CACHE=true CACHE_TTL=2h local-docs-mcp
```

**Performance**: Cache hits are ~3,000x faster than filesystem scans (66 nanoseconds vs 201 microseconds). The cache automatically invalidates when documentation files change, ensuring fresh data.

**How it works**:
- First query scans filesystem and populates cache
- Subsequent queries return instantly from memory
- File watcher detects changes and invalidates cache within 500ms
- TTL provides safety fallback (default: 1 hour)

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

## Security

- Path traversal prevention
- File size limits (5MB)
- UTF-8 validation
- No symlink following outside base directories
- Absolute path rejection

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.
