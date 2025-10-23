package internal

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/umputun/local-docs-mcp/internal/scanner"
)

const (
	// FuzzyThreshold is minimum score for fuzzy matching
	FuzzyThreshold = 0.3
	// MaxSearchResults is maximum number of results to return
	MaxSearchResults = 10
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

// SearchDocs searches for documentation files matching the query
func SearchDocs(ctx context.Context, sc scanner.Interface, query string) (*SearchOutput, error) {
	if query == "" {
		return &SearchOutput{
			Results: []SearchMatch{},
			Total:   0,
		}, nil
	}

	// get all files
	files, err := sc.Scan(ctx)
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

		score := calculateScore(normalizedQuery, f.Normalized)
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
	if len(matches) > MaxSearchResults {
		matches = matches[:MaxSearchResults]
	}

	return &SearchOutput{
		Results: matches,
		Total:   total,
	}, nil
}

// calculateScore computes match score for a file
func calculateScore(query, normalizedName string) float64 {
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
		// fuzzy.Find returns lower scores for better matches
		// we want higher scores for better matches
		fuzzyScore := 1.0 / (1.0 + float64(matches[0].Score))

		// only accept if above threshold
		if fuzzyScore >= FuzzyThreshold {
			return fuzzyScore * 0.7 // scale down fuzzy matches
		}
	}

	return 0
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

// ReadDoc reads a specific documentation file
func ReadDoc(ctx context.Context, sc scanner.Interface, path string, source *string, maxSize int64) (*ReadOutput, error) {
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
			baseDir = sc.CommandsDir()
			actualSource = scanner.SourceCommands
		case "project-docs":
			baseDir = sc.ProjectDocsDir()
			actualSource = scanner.SourceProjectDocs
		case "project-root":
			baseDir = sc.ProjectRootDir()
			actualSource = scanner.SourceProjectRoot
		default:
			return nil, fmt.Errorf("invalid source: %s", sourceStr)
		}

		// try to resolve and read from specified source
		resolvedPath, err := scanner.SafeResolvePath(baseDir, cleanPath, maxSize)
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
		{"commands", sc.CommandsDir(), scanner.SourceCommands},
		{"project-docs", sc.ProjectDocsDir(), scanner.SourceProjectDocs},
		{"project-root", sc.ProjectRootDir(), scanner.SourceProjectRoot},
	}

	for _, s := range sources {
		// check context
		select {
		case <-ctx.Done():
			return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
		default:
		}

		resolvedPath, err := scanner.SafeResolvePath(s.dir, cleanPath, maxSize)
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
			Source:  string(s.src),
		}, nil
	}

	return nil, fmt.Errorf("file not found in any source: %s", cleanPath)
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

// ListAllDocs returns a list of all available documentation files from all sources
func ListAllDocs(ctx context.Context, sc scanner.Interface, maxSize int64) (*ListOutput, error) {
	files, err := sc.Scan(ctx)
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
		if f.Size > maxSize {
			doc.TooLarge = true
		}

		docs = append(docs, doc)
	}

	return &ListOutput{
		Docs:  docs,
		Total: len(docs),
	}, nil
}
