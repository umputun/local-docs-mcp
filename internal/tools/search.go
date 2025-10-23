package tools

import (
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
func SearchDocs(sc *scanner.Scanner, query string) (*SearchOutput, error) {
	if query == "" {
		return &SearchOutput{
			Results: []SearchMatch{},
			Total:   0,
		}, nil
	}

	// get all files
	files, err := sc.Scan()
	if err != nil {
		return nil, err // nolint:wrapcheck // scanner error is descriptive
	}

	// normalize query (lowercase, replace spaces with hyphens)
	normalizedQuery := strings.ToLower(query)
	normalizedQuery = strings.ReplaceAll(normalizedQuery, " ", "-")

	var matches []SearchMatch

	// score each file
	for _, f := range files {
		score := calculateScore(normalizedQuery, f.Normalized, f.Name)
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
func calculateScore(query, normalizedName, _ string) float64 {
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
