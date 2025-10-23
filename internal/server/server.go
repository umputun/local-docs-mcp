package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/umputun/local-docs-mcp/internal/scanner"
	"github.com/umputun/local-docs-mcp/internal/tools"
)

// Config defines server configuration
type Config struct {
	CommandsDir    string
	ProjectDocsDir string
	ProjectRootDir string
	MaxFileSize    int64
	ServerName     string
	Version        string
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
	scanner *scanner.Scanner
	mcp     *mcp.Server
}

// New creates a new MCP server instance
func New(config Config) (*Server, error) {
	// validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// create scanner
	sc := scanner.NewScanner(
		config.CommandsDir,
		config.ProjectDocsDir,
		config.ProjectRootDir,
		config.MaxFileSize,
	)

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
func (s *Server) handleSearchDocs(_ context.Context, _ *mcp.CallToolRequest, input tools.SearchInput) (*mcp.CallToolResult, any, error) {
	log.Printf("[DEBUG] search_docs called with query: %s", input.Query)

	result, err := tools.SearchDocs(s.scanner, input.Query)
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
func (s *Server) handleReadDoc(_ context.Context, _ *mcp.CallToolRequest, input tools.ReadInput) (*mcp.CallToolResult, any, error) {
	log.Printf("[DEBUG] read_doc called with path: %s, source: %v", input.Path, input.Source)

	result, err := tools.ReadDoc(s.scanner, input.Path, input.Source, s.config.MaxFileSize)
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

// handleListAllDocs handles list_all_docs tool calls
func (s *Server) handleListAllDocs(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	log.Printf("[DEBUG] list_all_docs called")

	result, err := tools.ListAllDocs(s.scanner, s.config.MaxFileSize)
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
	log.Printf("[INFO] starting MCP server: %s v%s", s.config.ServerName, s.config.Version)
	log.Printf("[INFO] scanning sources: commands=%s, docs=%s, root=%s",
		s.config.CommandsDir, s.config.ProjectDocsDir, s.config.ProjectRootDir)

	// run server with stdio transport
	return s.mcp.Run(ctx, &mcp.StdioTransport{}) // nolint:wrapcheck // MCP SDK error is descriptive
}
