package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/local-docs-mcp/internal/scanner"
)

func TestSearchDocs(t *testing.T) {
	// create test directory structure
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(filepath.Join(commandsDir, "action"), 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	// create test files with various names
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "action", "commit.md"), []byte("commit"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "action", "commit-push.md"), []byte("commit-push"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "bootstrap.md"), []byte("bootstrap"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte("arch"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "testing.md"), []byte("test"), 0600))

	sc := scanner.NewScanner(commandsDir, docsDir, tmpDir, 1024*1024)

	tests := []struct {
		name           string
		query          string
		wantCount      int
		wantFirstMatch string
		wantExactMatch bool
	}{
		{
			name:           "exact match",
			query:          "commit",
			wantCount:      2, // commit.md and commit-push.md
			wantFirstMatch: "commit.md",
			wantExactMatch: true,
		},
		{
			name:           "partial match",
			query:          "arch",
			wantCount:      1,
			wantFirstMatch: "architecture.md",
		},
		{
			name:           "fuzzy match",
			query:          "boot",
			wantCount:      1,
			wantFirstMatch: "bootstrap.md",
		},
		{
			name:      "no matches",
			query:     "xyz123nonexistent",
			wantCount: 0,
		},
		{
			name:           "case insensitive",
			query:          "COMMIT",
			wantCount:      2,
			wantFirstMatch: "commit.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SearchDocs(sc, tt.query)
			require.NoError(t, err)
			assert.NotNil(t, result)

			if tt.wantCount == 0 {
				assert.Empty(t, result.Results)
				assert.Equal(t, 0, result.Total)
				return
			}

			assert.GreaterOrEqual(t, len(result.Results), 1, "should have at least one result")
			assert.Equal(t, tt.wantCount, result.Total)

			if tt.wantFirstMatch != "" {
				assert.Contains(t, result.Results[0].Name, tt.wantFirstMatch)
			}

			// verify scores are sorted (highest first)
			for i := 1; i < len(result.Results); i++ {
				assert.GreaterOrEqual(t, result.Results[i-1].Score, result.Results[i].Score,
					"results should be sorted by score descending")
			}
		})
	}
}

func TestSearchDocs_ScoreSorting(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create files with varying match quality
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("exact"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "testing.md"), []byte("substring"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "atestb.md"), []byte("contains"), 0600))

	sc := scanner.NewScanner(commandsDir, "", "", 1024*1024)

	result, err := SearchDocs(sc, "test")
	require.NoError(t, err)
	require.NotEmpty(t, result.Results)

	// exact match should score highest
	assert.Equal(t, "test.md", result.Results[0].Name)
	assert.Equal(t, 1.0, result.Results[0].Score)
}

func TestSearchDocs_LimitResults(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create more than 10 files
	for i := 0; i < 15; i++ {
		filename := filepath.Join(commandsDir, "test-file-"+string(rune('a'+i))+".md")
		require.NoError(t, os.WriteFile(filename, []byte("test"), 0600))
	}

	sc := scanner.NewScanner(commandsDir, "", "", 1024*1024)

	result, err := SearchDocs(sc, "test")
	require.NoError(t, err)

	// should limit to 10 results
	assert.LessOrEqual(t, len(result.Results), 10, "should limit to 10 results")
	assert.Equal(t, 15, result.Total, "total should reflect all matches")
}

func TestSearchDocs_EmptyQuery(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test"), 0600))

	sc := scanner.NewScanner(commandsDir, "", "", 1024*1024)

	result, err := SearchDocs(sc, "")
	require.NoError(t, err)
	assert.Empty(t, result.Results)
	assert.Equal(t, 0, result.Total)
}

func TestSearchDocs_NormalizedMatching(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create file with hyphens and mixed case
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "Go-Test-Example.md"), []byte("test"), 0600))

	sc := scanner.NewScanner(commandsDir, "", "", 1024*1024)

	// search with spaces (should convert to hyphens)
	result, err := SearchDocs(sc, "go test example")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Results)
	assert.Contains(t, result.Results[0].Name, "Go-Test-Example.md")
}
