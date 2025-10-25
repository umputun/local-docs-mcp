package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sahilm/fuzzy"

	"github.com/umputun/local-docs-mcp/app/scanner"
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

const (
	// fuzzyThreshold is minimum score for fuzzy matching
	fuzzyThreshold = 0.3
	// maxSearchResults is maximum number of results to return
	maxSearchResults = 10
)

// SearchInput represents input for searching documentation
type SearchInput struct {
	Query string `json:"query"`
}

// SearchMatch represents a single search result
type SearchMatch struct {
	Path   string  `json:"path"`
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Source string  `json:"source"`
}

// SearchOutput contains search results
type SearchOutput struct {
	Results []SearchMatch `json:"results"`
	Total   int           `json:"total"`
}

// ReadInput represents input for reading a documentation file
type ReadInput struct {
	Path   string  `json:"path"`
	Source *string `json:"source,omitempty"`
}

// ReadOutput contains the result of reading a documentation file
type ReadOutput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	Source  string `json:"source"`
}

// DocInfo represents information about a documentation file
type DocInfo struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Source   string `json:"source"`
	Size     int64  `json:"size,omitempty"`
	TooLarge bool   `json:"too_large,omitempty"`
}

// ListOutput contains the result of listing all documentation files
type ListOutput struct {
	Docs  []DocInfo `json:"docs"`
	Total int       `json:"total"`
}

// searchDocs searches for documentation files matching the query
func (s *Server) searchDocs(ctx context.Context, query string) (*SearchOutput, error) {
	if query == "" {
		return &SearchOutput{
			Results: []SearchMatch{},
			Total:   0,
		}, nil
	}

	// get all files
	files, err := s.scanner.Scan(ctx)
	if err != nil {
		return nil, err // nolint:wrapcheck // scanner error is descriptive
	}

	// normalize query (lowercase, replace spaces with hyphens)
	normalizedQuery := strings.ToLower(query)
	normalizedQuery = strings.ReplaceAll(normalizedQuery, " ", "-")

	var matches []SearchMatch

	// score each file
	for _, f := range files {
		// check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
		default:
		}

		score := s.calculateScore(normalizedQuery, f.Normalized)
		if score > 0 {
			matches = append(matches, SearchMatch{
				Path:   f.Filename,
				Name:   f.Name,
				Score:  score,
				Source: string(f.Source),
			})
		}
	}

	// sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	// limit results
	total := len(matches)
	if len(matches) > maxSearchResults {
		matches = matches[:maxSearchResults]
	}

	return &SearchOutput{
		Results: matches,
		Total:   total,
	}, nil
}

// calculateScore computes match score for a file
func (s *Server) calculateScore(query, normalizedName string) float64 {
	// exact match (case insensitive)
	if normalizedName == query || normalizedName == query+".md" {
		return 1.0
	}

	// substring match
	if strings.Contains(normalizedName, query) {
		// score based on how much of the name is the query
		return 0.8 * (float64(len(query)) / float64(len(normalizedName)))
	}

	// fuzzy match
	matches := fuzzy.Find(query, []string{normalizedName})
	if len(matches) > 0 && matches[0].Score > 0 {
		// normalize fuzzy score to 0-1 range
		// sahilm/fuzzy returns higher scores for better matches (up to ~100 for perfect match)
		fuzzyScore := float64(matches[0].Score) / 100.0
		if fuzzyScore > 1.0 {
			fuzzyScore = 1.0
		}

		// only accept if above threshold
		if fuzzyScore >= fuzzyThreshold {
			return fuzzyScore * 0.7 // scale down fuzzy matches
		}
	}

	return 0
}

// readDoc reads a specific documentation file
func (s *Server) readDoc(ctx context.Context, path string, source *string) (*ReadOutput, error) {
	// check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
	default:
	}

	// parse source prefix from path if present
	var sourceStr string
	cleanPath := path

	if strings.Contains(path, ":") {
		parts := strings.SplitN(path, ":", 2)
		sourceStr = parts[0]
		cleanPath = parts[1]
	} else if source != nil {
		sourceStr = *source
	}

	// map source string to directory
	var baseDir string
	var actualSource scanner.Source

	if sourceStr != "" {
		switch sourceStr {
		case "commands":
			baseDir = s.scanner.CommandsDir()
			actualSource = scanner.SourceCommands
		case "project-docs":
			baseDir = s.scanner.ProjectDocsDir()
			actualSource = scanner.SourceProjectDocs
		case "project-root":
			baseDir = s.scanner.ProjectRootDir()
			actualSource = scanner.SourceProjectRoot
		default:
			return nil, fmt.Errorf("invalid source: %s", sourceStr)
		}

		// try to resolve and read from specified source
		resolvedPath, err := scanner.SafeResolvePath(baseDir, cleanPath, s.config.MaxFileSize)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path in %s: %w", sourceStr, err)
		}

		// check context before reading
		select {
		case <-ctx.Done():
			return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
		default:
		}

		// #nosec G304 - path is validated by SafeResolvePath
		content, err := os.ReadFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		return &ReadOutput{
			Path:    cleanPath,
			Content: string(content),
			Size:    len(content),
			Source:  string(actualSource),
		}, nil
	}

	// no source specified, try all sources in order
	sources := []struct {
		name string
		dir  string
		src  scanner.Source
	}{
		{"commands", s.scanner.CommandsDir(), scanner.SourceCommands},
		{"project-docs", s.scanner.ProjectDocsDir(), scanner.SourceProjectDocs},
		{"project-root", s.scanner.ProjectRootDir(), scanner.SourceProjectRoot},
	}

	for _, src := range sources {
		// check context
		select {
		case <-ctx.Done():
			return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
		default:
		}

		resolvedPath, err := scanner.SafeResolvePath(src.dir, cleanPath, s.config.MaxFileSize)
		if err != nil {
			continue // try next source
		}

		// #nosec G304 - path is validated by SafeResolvePath
		content, err := os.ReadFile(resolvedPath)
		if err != nil {
			continue // try next source
		}

		return &ReadOutput{
			Path:    cleanPath,
			Content: string(content),
			Size:    len(content),
			Source:  string(src.src),
		}, nil
	}

	return nil, fmt.Errorf("file not found in any source: %s", cleanPath)
}

// listAllDocs returns a list of all available documentation files from all sources
func (s *Server) listAllDocs(ctx context.Context) (*ListOutput, error) {
	files, err := s.scanner.Scan(ctx)
	if err != nil {
		return nil, err // nolint:wrapcheck // scanner error is descriptive
	}

	docs := make([]DocInfo, 0, len(files))
	for _, f := range files {
		// check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
		default:
		}

		doc := DocInfo{
			Name:     f.Name,
			Filename: f.Filename,
			Source:   string(f.Source),
			Size:     f.Size,
		}

		// mark files that exceed max size
		if f.Size > s.config.MaxFileSize {
			doc.TooLarge = true
		}

		docs = append(docs, doc)
	}

	return &ListOutput{
		Docs:  docs,
		Total: len(docs),
	}, nil
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
func (s *Server) handleSearchDocs(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, any, error) {
	slog.Debug("search_docs called", "query", input.Query)

	result, err := s.searchDocs(ctx, input.Query)
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
func (s *Server) handleReadDoc(ctx context.Context, _ *mcp.CallToolRequest, input ReadInput) (*mcp.CallToolResult, any, error) {
	slog.Debug("read_doc called", "path", input.Path, "source", input.Source)

	result, err := s.readDoc(ctx, input.Path, input.Source)
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

	result, err := s.listAllDocs(ctx)
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
