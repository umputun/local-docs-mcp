package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	// create test files with various names
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "commit.md"), []byte("commit"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "commit-push.md"), []byte("commit-push"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "bootstrap.md"), []byte("bootstrap"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte("arch"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "testing.md"), []byte("test"), 0600))

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, ProjectDocsDir: docsDir, ProjectRootDir: tmpDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

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
			result, err := SearchDocs(ctx, sc, tt.query)
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

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	result, err := SearchDocs(ctx, sc, "test")
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

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	result, err := SearchDocs(ctx, sc, "test")
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

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	result, err := SearchDocs(ctx, sc, "")
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

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	// search with spaces (should convert to hyphens)
	result, err := SearchDocs(ctx, sc, "go test example")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Results)
	assert.Contains(t, result.Results[0].Name, "Go-Test-Example.md")
}

func TestSearchDocs_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test"), 0600))

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})

	// create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := SearchDocs(ctx, sc, "test")
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestReadDoc(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(filepath.Join(commandsDir, "action"), 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	// create test files
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("commands content"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "action", "commit.md"), []byte("action content"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "doc.md"), []byte("docs content"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("readme content"), 0600))

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, ProjectDocsDir: docsDir, ProjectRootDir: tmpDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	tests := []struct {
		name        string
		path        string
		source      *string
		wantContent string
		wantSource  string
		wantErr     bool
	}{
		{
			name:        "read from commands (no source)",
			path:        "test.md",
			source:      nil,
			wantContent: "commands content",
			wantSource:  "commands",
		},
		{
			name:        "read with commands prefix",
			path:        "commands:test.md",
			source:      nil,
			wantContent: "commands content",
			wantSource:  "commands",
		},
		{
			name:        "read nested file",
			path:        "action/commit.md",
			source:      nil,
			wantContent: "action content",
			wantSource:  "commands",
		},
		{
			name:        "read from project-docs",
			path:        "doc.md",
			source:      strPtr("project-docs"),
			wantContent: "docs content",
			wantSource:  "project-docs",
		},
		{
			name:        "read from project-root",
			path:        "README.md",
			source:      strPtr("project-root"),
			wantContent: "readme content",
			wantSource:  "project-root",
		},
		{
			name:    "file not found",
			path:    "nonexistent.md",
			source:  nil,
			wantErr: true,
		},
		{
			name:    "invalid source",
			path:    "test.md",
			source:  strPtr("invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ReadDoc(ctx, sc, tt.path, tt.source, 1024*1024)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantContent, result.Content)
			assert.Equal(t, tt.wantSource, result.Source)
			assert.Equal(t, len(tt.wantContent), result.Size)
		})
	}
}

func TestReadDoc_AutoAddExtension(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("content"), 0600))

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	// should work with or without .md extension
	result1, err := ReadDoc(ctx, sc, "test", nil, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "content", result1.Content)

	result2, err := ReadDoc(ctx, sc, "test.md", nil, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "content", result2.Content)
}

func TestReadDoc_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create large file
	largeContent := make([]byte, 2000)
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "large.md"), largeContent, 0600))

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1000, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	_, err := ReadDoc(ctx, sc, "large.md", nil, 1000)
	assert.Error(t, err)
	// the error could be "too large" or "file not found" depending on which source tries first
	assert.True(t,
		strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "file not found"),
		"error should mention file size or not found: %v", err)
}

func TestReadDoc_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("content"), 0600))

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ReadDoc(ctx, sc, "test.md", nil, 1024*1024)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestListAllDocs(t *testing.T) {
	// create test directory structure
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	// create test files
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test1.md"), []byte("test"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "test2.md"), []byte("doc"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("readme"), 0600))

	// create scanner
	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, ProjectDocsDir: docsDir, ProjectRootDir: tmpDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	result, err := ListAllDocs(ctx, sc, 1024*1024)
	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.Len(t, result.Docs, 3)
	assert.Equal(t, 3, result.Total)

	// verify all sources are represented
	sources := make(map[string]bool)
	for _, doc := range result.Docs {
		sources[doc.Source] = true
		assert.False(t, doc.TooLarge)
	}

	assert.True(t, sources["commands"])
	assert.True(t, sources["project-docs"])
	assert.True(t, sources["project-root"])
}

func TestListAllDocs_TooLargeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create small file
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "small.md"), []byte("small"), 0600))

	// create large file
	largeContent := make([]byte, 2000)
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "large.md"), largeContent, 0600))

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1000, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	result, err := ListAllDocs(ctx, sc, 1000)
	require.NoError(t, err)

	assert.Len(t, result.Docs, 2)

	// find large file
	var largeDoc *DocInfo
	for i := range result.Docs {
		if result.Docs[i].Name == "large.md" {
			largeDoc = &result.Docs[i]
			break
		}
	}

	require.NotNil(t, largeDoc, "large.md should be in results")
	assert.True(t, largeDoc.TooLarge, "large.md should be marked as too large")
}

func TestListAllDocs_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test"), 0600))

	sc := scanner.NewScanner(scanner.Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ListAllDocs(ctx, sc, 1024*1024)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

// helper function
func strPtr(s string) *string {
	return &s
}
