package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/umputun/local-docs-mcp/app/scanner"
	"github.com/umputun/local-docs-mcp/app/tools"
)

// Config defines server configuration
type Config struct {
	CommandsDir    string
	ProjectDocsDir string
	ProjectRootDir string
	ExcludeDirs    []string
	MaxFileSize    int64
	ServerName     string
	Version        string
	EnableCache    bool
	CacheTTL       time.Duration
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.ServerName == "" {
		return fmt.Errorf("server name is required")
	}
	if c.MaxFileSize <= 0 {
		return fmt.Errorf("max file size must be greater than zero")
	}
	return nil
}

// Server represents the MCP server instance
type Server struct {
	config  Config
	scanner scanner.Interface
	mcp     *mcp.Server
}

// New creates a new MCP server instance
func New(config Config) (*Server, error) {
	// validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// create base scanner
	baseScanner := scanner.NewScanner(scanner.Params{
		CommandsDir:    config.CommandsDir,
		ProjectDocsDir: config.ProjectDocsDir,
		ProjectRootDir: config.ProjectRootDir,
		MaxFileSize:    config.MaxFileSize,
		ExcludeDirs:    config.ExcludeDirs,
	})

	// wrap with caching if enabled
	var sc scanner.Interface = baseScanner
	if config.EnableCache {
		cached, err := scanner.NewCachedScanner(baseScanner, config.CacheTTL)
		if err != nil {
			slog.Warn("failed to create cached scanner, using regular scanner", "error", err)
		} else {
			sc = cached
			slog.Info("file list caching enabled", "ttl", config.CacheTTL)
		}
	}

	// create MCP server
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    config.ServerName,
		Version: config.Version,
	}, nil)

	server := &Server{
		config:  config,
		scanner: sc,
		mcp:     mcpServer,
	}

	// register tools
	server.registerTools()

	return server, nil
}

// registerTools registers all MCP tools
func (s *Server) registerTools() {
	// register search_docs tool
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "search_docs",
		Description: "Search for documentation files matching the query with fuzzy matching. Returns top 10 results sorted by relevance.",
	}, s.handleSearchDocs)

	// register read_doc tool
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "read_doc",
		Description: "Read a specific documentation file. Supports source prefixes (e.g., 'commands:action/commit.md') or tries all sources if not specified.",
	}, s.handleReadDoc)

	// register list_all_docs tool
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_all_docs",
		Description: "List all available documentation files from all sources (commands, project-docs, project-root).",
	}, s.handleListAllDocs)
}

// handleSearchDocs handles search_docs tool calls
func (s *Server) handleSearchDocs(ctx context.Context, _ *mcp.CallToolRequest, input tools.SearchInput) (*mcp.CallToolResult, any, error) {
	slog.Debug("search_docs called", "query", input.Query)

	result, err := tools.SearchDocs(ctx, s.scanner, input.Query)
	if err != nil {
		return nil, nil, fmt.Errorf("search failed: %w", err)
	}

	// convert to JSON for response
	content, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(content),
			},
		},
	}, result, nil
}

// handleReadDoc handles read_doc tool calls
func (s *Server) handleReadDoc(ctx context.Context, _ *mcp.CallToolRequest, input tools.ReadInput) (*mcp.CallToolResult, any, error) {
	slog.Debug("read_doc called", "path", input.Path, "source", input.Source)

	result, err := tools.ReadDoc(ctx, s.scanner, input.Path, input.Source, s.config.MaxFileSize)
	if err != nil {
		return nil, nil, fmt.Errorf("read failed: %w", err)
	}

	// convert to JSON for response
	content, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(content),
			},
		},
	}, result, nil
}

// handleListAllDocs handles list_all_docs tool calls.
// input is required by MCP SDK signature but list_all_docs takes no parameters.
func (s *Server) handleListAllDocs(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	slog.Debug("list_all_docs called")

	result, err := tools.ListAllDocs(ctx, s.scanner, s.config.MaxFileSize)
	if err != nil {
		return nil, nil, fmt.Errorf("list failed: %w", err)
	}

	// convert to JSON for response
	content, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(content),
			},
		},
	}, result, nil
}

// Run starts the MCP server with stdio transport
func (s *Server) Run(ctx context.Context) error {
	slog.Info("starting MCP server", "name", s.config.ServerName, "version", s.config.Version)
	slog.Info("scanning sources", "commands", s.config.CommandsDir, "docs", s.config.ProjectDocsDir, "root", s.config.ProjectRootDir)

	// ensure cleanup on exit
	defer s.Close()

	// run server with stdio transport
	return s.mcp.Run(ctx, &mcp.StdioTransport{}) // nolint:wrapcheck // MCP SDK error is descriptive
}

// Close cleans up server resources
func (s *Server) Close() error {
	if s.scanner != nil {
		return s.scanner.Close() // nolint:wrapcheck // scanner error is descriptive
	}
	return nil
}
