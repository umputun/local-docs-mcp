package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/local-docs-mcp/app/scanner"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				CommandsDir:    commandsDir,
				ProjectDocsDir: docsDir,
				ProjectRootDir: tmpDir,
				MaxFileSize:    1024 * 1024,
				ServerName:     "test-server",
				Version:        "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "empty server name",
			config: Config{
				CommandsDir:    commandsDir,
				ProjectDocsDir: docsDir,
				ProjectRootDir: tmpDir,
				MaxFileSize:    1024 * 1024,
				ServerName:     "",
				Version:        "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "zero max file size",
			config: Config{
				CommandsDir:    commandsDir,
				ProjectDocsDir: docsDir,
				ProjectRootDir: tmpDir,
				MaxFileSize:    0,
				ServerName:     "test-server",
				Version:        "1.0.0",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := New(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, srv)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, srv)
			assert.NotNil(t, srv.mcp)
			assert.NotNil(t, srv.scanner)
		})
	}
}

func TestServerInitialization(t *testing.T) {
	// create test directory structure
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	// create test files
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test content"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "doc.md"), []byte("doc content"), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: tmpDir,
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, srv)

	// verify server is properly initialized
	assert.NotNil(t, srv.mcp)
	assert.NotNil(t, srv.scanner)
	assert.Equal(t, config.ServerName, srv.config.ServerName)
	assert.Equal(t, config.Version, srv.config.Version)

	// verify scanner has access to test files
	files, err := srv.scanner.Scan(context.Background())
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestServerSearchDocsHandler(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	// create test files
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "commit.md"), []byte("commit"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "push.md"), []byte("push"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte("arch"), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: tmpDir,
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	tests := []struct {
		name      string
		query     string
		wantTotal int
		wantFirst string
	}{
		{
			name:      "exact match",
			query:     "commit",
			wantTotal: 1,
			wantFirst: "commit.md",
		},
		{
			name:      "partial match",
			query:     "arch",
			wantTotal: 1,
			wantFirst: "architecture.md",
		},
		{
			name:      "no matches",
			query:     "xyz123nonexistent",
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// call the handler directly
			ctx := context.Background()
			req := &mcp.CallToolRequest{}
			input := SearchInput{Query: tt.query}

			result, output, err := srv.handleSearchDocs(ctx, req, input)
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotNil(t, output)

			// verify output structure
			searchOutput, ok := output.(*SearchOutput)
			require.True(t, ok)
			assert.Equal(t, tt.wantTotal, searchOutput.Total)

			if tt.wantTotal > 0 {
				assert.NotEmpty(t, searchOutput.Results)
				if tt.wantFirst != "" {
					assert.Contains(t, searchOutput.Results[0].Name, tt.wantFirst)
				}
			}

			// verify result content is valid JSON
			assert.NotEmpty(t, result.Content)
			var jsonData interface{}
			err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &jsonData)
			assert.NoError(t, err)
		})
	}
}

func TestServerReadDocHandler(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	testContent := "test file content"
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte(testContent), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: tmpDir,
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	tests := []struct {
		name        string
		path        string
		source      *string
		wantContent string
		wantErr     bool
	}{
		{
			name:        "read existing file",
			path:        "test.md",
			source:      nil,
			wantContent: testContent,
			wantErr:     false,
		},
		{
			name:        "read with source prefix",
			path:        "commands:test.md",
			source:      nil,
			wantContent: testContent,
			wantErr:     false,
		},
		{
			name:    "read non-existent file",
			path:    "nonexistent.md",
			source:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req := &mcp.CallToolRequest{}
			input := ReadInput{Path: tt.path, Source: tt.source}

			result, output, err := srv.handleReadDoc(ctx, req, input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotNil(t, output)

			// verify output structure
			readOutput, ok := output.(*ReadOutput)
			require.True(t, ok)
			assert.Equal(t, tt.wantContent, readOutput.Content)

			// verify result content is valid JSON
			assert.NotEmpty(t, result.Content)
			var jsonData interface{}
			err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &jsonData)
			assert.NoError(t, err)
		})
	}
}

func TestServerListAllDocsHandler(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(filepath.Join(commandsDir, "action"), 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	// create test files
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "action", "commit.md"), []byte("commit"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "doc.md"), []byte("doc"), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: tmpDir,
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	// call the handler directly
	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := struct{}{}

	result, output, err := srv.handleListAllDocs(ctx, req, input)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, output)

	// verify output structure
	listOutput, ok := output.(*ListOutput)
	require.True(t, ok)
	assert.Len(t, listOutput.Docs, 3)
	assert.Equal(t, 3, listOutput.Total)

	// verify files have correct sources
	sources := make(map[string]bool)
	for _, f := range listOutput.Docs {
		sources[f.Source] = true
	}
	assert.True(t, sources["commands"])
	assert.True(t, sources["project-docs"])

	// verify result content is valid JSON
	assert.NotEmpty(t, result.Content)
	var jsonData interface{}
	err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &jsonData)
	assert.NoError(t, err)
}

func TestServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				CommandsDir:    "/tmp/commands",
				ProjectDocsDir: "/tmp/docs",
				ProjectRootDir: "/tmp",
				MaxFileSize:    1024,
				ServerName:     "test",
				Version:        "1.0",
			},
			wantErr: false,
		},
		{
			name: "empty server name",
			config: Config{
				CommandsDir:    "/tmp/commands",
				ProjectDocsDir: "/tmp/docs",
				ProjectRootDir: "/tmp",
				MaxFileSize:    1024,
				ServerName:     "",
				Version:        "1.0",
			},
			wantErr: true,
			errMsg:  "server name is required",
		},
		{
			name: "zero max file size",
			config: Config{
				CommandsDir:    "/tmp/commands",
				ProjectDocsDir: "/tmp/docs",
				ProjectRootDir: "/tmp",
				MaxFileSize:    0,
				ServerName:     "test",
				Version:        "1.0",
			},
			wantErr: true,
			errMsg:  "max file size must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServer_Close(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test",
		Version:        "1.0",
		CacheTTL:       time.Minute,
	}

	srv, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, srv)

	// close server
	err = srv.Close()
	assert.NoError(t, err)

	// close should be idempotent
	err = srv.Close()
	assert.NoError(t, err)
}

func TestServer_Close_NilScanner(t *testing.T) {
	// test close with nil scanner (shouldn't happen in practice but test defensively)
	srv := &Server{
		scanner: nil,
	}

	err := srv.Close()
	assert.NoError(t, err)
}

func TestServer_SearchDocs_ScoreSorting(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create files with different match qualities
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("exact"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test-command.md"), []byte("substring"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "tset.md"), []byte("fuzzy"), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	result, err := srv.searchDocs(context.Background(), "test")
	require.NoError(t, err)
	require.NotEmpty(t, result.Results)

	// verify scores are sorted descending
	for i := 1; i < len(result.Results); i++ {
		assert.GreaterOrEqual(t, result.Results[i-1].Score, result.Results[i].Score,
			"results should be sorted by score descending")
	}

	// verify exact match has highest score
	assert.Equal(t, "test.md", result.Results[0].Name)
	assert.Equal(t, 1.0, result.Results[0].Score)
}

func TestServer_SearchDocs_LimitResults(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	// create 15 files to test result limiting
	for i := 0; i < 15; i++ {
		filename := fmt.Sprintf("test-file-%02d.md", i)
		require.NoError(t, os.WriteFile(filepath.Join(commandsDir, filename), []byte("content"), 0600))
	}

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	result, err := srv.searchDocs(context.Background(), "test")
	require.NoError(t, err)

	// should return max 10 results but total should be 15
	assert.Len(t, result.Results, 10, "should limit to 10 results")
	assert.Equal(t, 15, result.Total, "total should reflect all matches")
}

func TestServer_SearchDocs_EmptyQuery(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	result, err := srv.searchDocs(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, result.Results)
	assert.Equal(t, 0, result.Total)
}

func TestServer_SearchDocs_NormalizedMatching(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create file with hyphens
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "git-commit.md"), []byte("content"), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	// search with spaces should match file with hyphens
	result, err := srv.searchDocs(context.Background(), "git commit")
	require.NoError(t, err)
	require.NotEmpty(t, result.Results)
	assert.Contains(t, result.Results[0].Name, "git-commit")
}

func TestServer_SearchDocs_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = srv.searchDocs(ctx, "test")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestServer_ReadDoc_AutoAddExtension(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	testContent := "test content"
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte(testContent), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	tests := []struct {
		name string
		path string
	}{
		{
			name: "with .md extension",
			path: "test.md",
		},
		{
			name: "without .md extension",
			path: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := srv.readDoc(context.Background(), tt.path, nil)
			require.NoError(t, err)
			assert.Equal(t, testContent, result.Content)
		})
	}
}

func TestServer_ReadDoc_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	maxSize := int64(100)
	largeContent := make([]byte, maxSize+1)
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "large.md"), largeContent, 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: "",
		MaxFileSize:    maxSize,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	// specify source explicitly to test size limit enforcement
	source := "commands"
	_, err = srv.readDoc(context.Background(), "large.md", &source)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve path")
}

func TestServer_ReadDoc_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = srv.readDoc(ctx, "test.md", nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestServer_ReadDoc_InvalidSource(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	source := "invalid-source"
	_, err = srv.readDoc(context.Background(), "test.md", &source)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid source")
}

func TestServer_ListAllDocs_TooLargeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	maxSize := int64(100)

	// create normal size file
	smallContent := []byte("small")
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "small.md"), smallContent, 0600))

	// create oversized file
	largeContent := make([]byte, maxSize+1)
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "large.md"), largeContent, 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: "",
		MaxFileSize:    maxSize,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	result, err := srv.listAllDocs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)

	// find large file and verify it's marked as too large
	var largeDoc *DocInfo
	for i := range result.Docs {
		if result.Docs[i].Name == "large.md" {
			largeDoc = &result.Docs[i]
			break
		}
	}
	require.NotNil(t, largeDoc, "large.md should be in results")
	assert.True(t, largeDoc.TooLarge, "large.md should be marked as too large")

	// verify small file is not marked as too large
	var smallDoc *DocInfo
	for i := range result.Docs {
		if result.Docs[i].Name == "small.md" {
			smallDoc = &result.Docs[i]
			break
		}
	}
	require.NotNil(t, smallDoc, "small.md should be in results")
	assert.False(t, smallDoc.TooLarge, "small.md should not be marked as too large")
}

func TestServer_ListAllDocs_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = srv.listAllDocs(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestServer_CalculateScore(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	tests := []struct {
		name      string
		query     string
		file      scanner.FileInfo
		wantScore float64
		checkFunc func(t *testing.T, score float64)
	}{
		{
			name:      "exact match",
			query:     "test",
			file:      scanner.FileInfo{Normalized: "test"},
			wantScore: 1.0,
		},
		{
			name:      "exact match with .md extension",
			query:     "test",
			file:      scanner.FileInfo{Normalized: "test.md"},
			wantScore: 1.0,
		},
		{
			name:      "exact match already has .md",
			query:     "test.md",
			file:      scanner.FileInfo{Normalized: "test.md"},
			wantScore: 1.0,
		},
		{
			name:  "substring match - half the name",
			query: "test",
			file:  scanner.FileInfo{Normalized: "test-command"},
			checkFunc: func(t *testing.T, score float64) {
				// score should be 0.8 * (4/12) = 0.267
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 1.0)
				assert.Greater(t, score, 0.2)
				assert.Less(t, score, 0.3)
			},
		},
		{
			name:  "substring match - most of the name",
			query: "architecture",
			file:  scanner.FileInfo{Normalized: "architecture-guide"},
			checkFunc: func(t *testing.T, score float64) {
				// score should be 0.8 * (12/18) = 0.533
				assert.Greater(t, score, 0.5)
				assert.Less(t, score, 0.6)
			},
		},
		{
			name:  "substring match - small query in long name",
			query: "ab",
			file:  scanner.FileInfo{Normalized: "abcdefghijklmnop"},
			checkFunc: func(t *testing.T, score float64) {
				// score should be 0.8 * (2/16) = 0.1
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 0.15)
			},
		},
		{
			name:  "fuzzy match above threshold",
			query: "test",
			file:  scanner.FileInfo{Normalized: "t-e-s-t"},
			checkFunc: func(t *testing.T, score float64) {
				// sahilm/fuzzy can match chars in sequence with gaps
				// score should be > 0 and scaled down (< exact/substring)
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 0.7)
				assert.Greater(t, score, 0.4) // should be ~0.469 (67/100 * 0.7)
			},
		},
		{
			name:      "fuzzy match below threshold - weak subsequence",
			query:     "tst",
			file:      scanner.FileInfo{Normalized: "test"},
			wantScore: 0.0, // score too low, below 0.3 threshold
		},
		{
			name:      "no fuzzy match - chars out of order",
			query:     "tset",
			file:      scanner.FileInfo{Normalized: "test"},
			wantScore: 0.0, // sahilm/fuzzy can't match transpositions
		},
		{
			name:      "no match - completely different",
			query:     "xyz",
			file:      scanner.FileInfo{Normalized: "abc"},
			wantScore: 0.0,
		},
		{
			name:      "no match - very different strings",
			query:     "architecture",
			file:      scanner.FileInfo{Normalized: "zzzzzz"},
			wantScore: 0.0,
		},
		{
			name:      "case sensitivity already normalized",
			query:     "test",
			file:      scanner.FileInfo{Normalized: "test"},
			wantScore: 1.0,
		},
		{
			name:      "empty query",
			query:     "",
			file:      scanner.FileInfo{Normalized: "test"},
			wantScore: 0.0,
		},
		{
			name:      "empty normalized name",
			query:     "test",
			file:      scanner.FileInfo{Normalized: ""},
			wantScore: 0.0,
		},
		{
			name:      "both empty",
			query:     "",
			file:      scanner.FileInfo{Normalized: ""},
			wantScore: 1.0, // empty == empty
		},
		{
			name:  "substring at end",
			query: "file",
			file:  scanner.FileInfo{Normalized: "test-file"},
			checkFunc: func(t *testing.T, score float64) {
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 1.0)
			},
		},
		{
			name:  "substring at start",
			query: "test",
			file:  scanner.FileInfo{Normalized: "test-file"},
			checkFunc: func(t *testing.T, score float64) {
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 1.0)
			},
		},
		{
			name:  "substring in middle",
			query: "mid",
			file:  scanner.FileInfo{Normalized: "start-mid-end"},
			checkFunc: func(t *testing.T, score float64) {
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 1.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// for these tests, query has no spaces, so both params are the same
			score := srv.calculateScore(tt.query, tt.query, tt.file)

			if tt.checkFunc != nil {
				tt.checkFunc(t, score)
			} else {
				assert.Equal(t, tt.wantScore, score)
			}
		})
	}
}

func TestServer_CalculateScore_FuzzyThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	// test cases that should be below fuzzy threshold and return 0
	tests := []struct {
		name  string
		query string
		file  scanner.FileInfo
	}{
		{
			name:  "very poor fuzzy match",
			query: "abcdefgh",
			file:  scanner.FileInfo{Normalized: "zyxwvuts"},
		},
		{
			name:  "single char query no match",
			query: "x",
			file:  scanner.FileInfo{Normalized: "test"},
		},
		{
			name:  "completely unrelated",
			query: "documentation",
			file:  scanner.FileInfo{Normalized: "zzzzz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// for these tests, query has no spaces, so both params are the same
			score := srv.calculateScore(tt.query, tt.query, tt.file)
			assert.Equal(t, 0.0, score, "poor fuzzy matches should be below threshold")
		})
	}
}

func TestServer_CalculateScore_FuzzyMatching(t *testing.T) {
	// tests fuzzy matching behavior with sahilm/fuzzy library
	// note: this library does subsequence matching (chars in order with gaps)
	// not true fuzzy matching (transpositions, insertions, deletions)

	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	// cases caught by substring before reaching fuzzy
	substringCaught := []struct {
		query string
		file  scanner.FileInfo
	}{
		{"doc", scanner.FileInfo{Normalized: "docs"}},   // doc is substring of docs
		{"test", scanner.FileInfo{Normalized: "ttest"}}, // test is substring of ttest
		{"git", scanner.FileInfo{Normalized: "gitt"}},   // git is substring of gitt
	}

	for _, tt := range substringCaught {
		// for these tests, query has no spaces, so both params are the same
		score := srv.calculateScore(tt.query, tt.query, tt.file)
		// these all match via substring, not fuzzy
		assert.Greater(t, score, 0.0, "should match via substring")
		assert.LessOrEqual(t, score, 0.8, "substring matches score <= 0.8")
	}

	// successful fuzzy matches (not exact, not substring)
	fuzzyMatches := []struct {
		query string
		file  scanner.FileInfo
	}{
		{"test", scanner.FileInfo{Normalized: "t-e-s-t"}},          // chars with separators
		{"cmd", scanner.FileInfo{Normalized: "c-o-m-m-a-n-d"}},     // subsequence with gaps
		{"git", scanner.FileInfo{Normalized: "g-i-t-c-o-m-m-i-t"}}, // prefix subsequence
		{"comit", scanner.FileInfo{Normalized: "commit"}},          // missing char (still valid subsequence)
	}

	for _, tt := range fuzzyMatches {
		// for these tests, query has no spaces, so both params are the same
		score := srv.calculateScore(tt.query, tt.query, tt.file)
		assert.Greater(t, score, 0.0, "fuzzy match should score > 0")
		assert.Less(t, score, 0.8, "fuzzy matches score < substring matches")
	}

	// sahilm/fuzzy limitations - cannot match these
	noFuzzyMatch := []struct {
		query  string
		file   scanner.FileInfo
		reason string
	}{
		{"tset", scanner.FileInfo{Normalized: "test"}, "chars out of order (transposition)"},
		{"tst", scanner.FileInfo{Normalized: "test"}, "weak subsequence below threshold"},
		{"xyz", scanner.FileInfo{Normalized: "abc"}, "no common chars"},
	}

	for _, tt := range noFuzzyMatch {
		// for these tests, query has no spaces, so both params are the same
		score := srv.calculateScore(tt.query, tt.query, tt.file)
		assert.Equal(t, 0.0, score, "should not match: %s", tt.reason)
	}
}

func TestServer_ApplyFrontmatterBoost(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	tests := []struct {
		name      string
		baseScore float64
		query     string
		file      scanner.FileInfo
		wantScore float64
	}{
		{
			name:      "description match boosts score",
			baseScore: 0.5,
			query:     "testing",
			file:      scanner.FileInfo{Description: "This is for testing purposes", Tags: nil},
			wantScore: 1.0, // 0.5 + 0.5
		},
		{
			name:      "exact tag match boosts score",
			baseScore: 0.5,
			query:     "golang",
			file:      scanner.FileInfo{Description: "", Tags: []string{"golang", "tutorial"}},
			wantScore: 0.8, // 0.5 + 0.3
		},
		{
			name:      "partial tag match boosts score",
			baseScore: 0.5,
			query:     "go",
			file:      scanner.FileInfo{Description: "", Tags: []string{"golang"}},
			wantScore: 0.65, // 0.5 + 0.15
		},
		{
			name:      "multiple tag matches - exact and partial",
			baseScore: 0.5,
			query:     "test",
			file:      scanner.FileInfo{Description: "", Tags: []string{"test", "testing"}},
			wantScore: 0.95, // 0.5 + 0.3 (exact) + 0.15 (partial), note: might have float precision
		},
		{
			name:      "description and tag match combined",
			baseScore: 0.3,
			query:     "api",
			file:      scanner.FileInfo{Description: "API documentation guide", Tags: []string{"api", "rest"}},
			wantScore: 1.1, // 0.3 + 0.5 (desc) + 0.3 (tag)
		},
		{
			name:      "case insensitive description match",
			baseScore: 0.4,
			query:     "testing",
			file:      scanner.FileInfo{Description: "TESTING AND DEVELOPMENT", Tags: nil},
			wantScore: 0.9, // 0.4 + 0.5
		},
		{
			name:      "case insensitive tag match",
			baseScore: 0.4,
			query:     "golang",
			file:      scanner.FileInfo{Description: "", Tags: []string{"GoLang"}},
			wantScore: 0.7, // 0.4 + 0.3
		},
		{
			name:      "no boost - no matches",
			baseScore: 0.6,
			query:     "test",
			file:      scanner.FileInfo{Description: "something else", Tags: []string{"other"}},
			wantScore: 0.6, // no boost
		},
		{
			name:      "no boost - empty frontmatter",
			baseScore: 0.7,
			query:     "test",
			file:      scanner.FileInfo{Description: "", Tags: nil},
			wantScore: 0.7, // no boost
		},
		{
			name:      "boost capped at 1.0 - excessive metadata",
			baseScore: 0.5,
			query:     "test",
			file:      scanner.FileInfo{Description: "test description", Tags: []string{"test", "testing", "tester", "tested"}},
			wantScore: 1.5, // 0.5 + 1.0 (capped: would be 0.5 + 0.3 + 0.15*3 = 1.25 without cap)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := srv.applyFrontmatterBoost(tt.baseScore, tt.query, tt.file)
			assert.InDelta(t, tt.wantScore, score, 0.0001, "score should match within precision")
		})
	}
}

func TestServer_CalculateScore_MultiWordQuery(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	tests := []struct {
		name             string
		filenameQuery    string
		frontmatterQuery string
		file             scanner.FileInfo
		expectDescBoost  bool
		expectTagBoost   bool
		minScore         float64
	}{
		{
			name:             "multi-word query matches description with spaces",
			filenameQuery:    "test-plan",
			frontmatterQuery: "test plan",
			file: scanner.FileInfo{
				Normalized:  "something-else.md",
				Description: "comprehensive test plan for the application",
				Tags:        nil,
			},
			expectDescBoost: true,
			minScore:        0.5, // should get +0.5 boost
		},
		{
			name:             "multi-word query matches tag with spaces",
			filenameQuery:    "api-documentation",
			frontmatterQuery: "api documentation",
			file: scanner.FileInfo{
				Normalized:  "guide.md",
				Description: "",
				Tags:        []string{"api documentation", "rest"},
			},
			expectTagBoost: true,
			minScore:       0.3, // should get +0.3 boost for exact tag match
		},
		{
			name:             "multi-word query partial match in description",
			filenameQuery:    "test-case",
			frontmatterQuery: "test case",
			file: scanner.FileInfo{
				Normalized:  "doc.md",
				Description: "writing effective test case scenarios",
				Tags:        nil,
			},
			expectDescBoost: true,
			minScore:        0.5,
		},
		{
			name:             "hyphenated query matches hyphenated filename but not spaced description",
			filenameQuery:    "test-plan",
			frontmatterQuery: "test-plan",
			file: scanner.FileInfo{
				Normalized:  "test-plan.md",
				Description: "test plan documentation", // has spaces, won't match hyphenated query
				Tags:        nil,
			},
			expectDescBoost: false,
			minScore:        1.0, // exact filename match
		},
		{
			name:             "spaced query matches both filename (hyphenated) and description (spaced)",
			filenameQuery:    "test-plan",
			frontmatterQuery: "test plan",
			file: scanner.FileInfo{
				Normalized:  "test-plan.md",
				Description: "test plan documentation",
				Tags:        []string{"test plan"},
			},
			expectDescBoost: true,
			expectTagBoost:  true,
			minScore:        1.8, // 1.0 (exact) + 0.5 (desc) + 0.3 (tag)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := srv.calculateScore(tt.filenameQuery, tt.frontmatterQuery, tt.file)
			assert.GreaterOrEqual(t, score, tt.minScore,
				"score should be at least %f (got %f)", tt.minScore, score)

			if tt.expectDescBoost {
				assert.Contains(t, strings.ToLower(tt.file.Description), tt.frontmatterQuery,
					"frontmatterQuery should match description")
			}
			if tt.expectTagBoost {
				matched := false
				for _, tag := range tt.file.Tags {
					if strings.Contains(strings.ToLower(tag), tt.frontmatterQuery) {
						matched = true
						break
					}
				}
				assert.True(t, matched, "frontmatterQuery should match at least one tag")
			}
		})
	}
}

func TestServer_ReadDoc_FrontmatterStripping(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create file with frontmatter
	contentWithFrontmatter := `---
description: Test file with frontmatter
tags: [test, example]
---

# Actual Content

This is the real content that should be returned.`

	expectedContent := `
# Actual Content

This is the real content that should be returned.`

	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "with-fm.md"),
		[]byte(contentWithFrontmatter), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	stringPtr := func(s string) *string { return &s }

	tests := []struct {
		name   string
		path   string
		source *string
	}{
		{
			name:   "with source specified",
			path:   "with-fm.md",
			source: stringPtr("commands"),
		},
		{
			name:   "without source (tries all)",
			path:   "with-fm.md",
			source: nil,
		},
		{
			name:   "with source prefix in path",
			path:   "commands:with-fm.md",
			source: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := srv.readDoc(context.Background(), tt.path, tt.source)
			require.NoError(t, err)
			assert.Equal(t, expectedContent, result.Content,
				"frontmatter should be stripped from content")
			assert.NotContains(t, result.Content, "---",
				"content should not contain frontmatter delimiters")
			assert.NotContains(t, result.Content, "description:",
				"content should not contain frontmatter fields")
		})
	}
}

func TestServer_ListAllDocs_IncludesFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create file with frontmatter
	contentWithFM := `---
description: Command for testing purposes
tags: [testing, development, cli]
---

# Test Command`

	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test-cmd.md"),
		[]byte(contentWithFM), 0600))

	// create file without frontmatter
	contentWithoutFM := "# Plain Command"
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "plain.md"),
		[]byte(contentWithoutFM), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	result, err := srv.listAllDocs(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result.Docs), 2, "should have at least 2 files")

	// find file with frontmatter (only check commands source)
	var docWithFM, docWithoutFM *DocInfo
	for i := range result.Docs {
		if result.Docs[i].Name == "test-cmd.md" && result.Docs[i].Source == "commands" {
			docWithFM = &result.Docs[i]
		}
		if result.Docs[i].Name == "plain.md" && result.Docs[i].Source == "commands" {
			docWithoutFM = &result.Docs[i]
		}
	}

	require.NotNil(t, docWithFM, "should find file with frontmatter")
	assert.Equal(t, "Command for testing purposes", docWithFM.Description)
	assert.Equal(t, []string{"testing", "development", "cli"}, docWithFM.Tags)

	require.NotNil(t, docWithoutFM, "should find file without frontmatter")
	assert.Empty(t, docWithoutFM.Description)
	assert.Empty(t, docWithoutFM.Tags)
}

func TestServer_SearchDocs_FrontmatterBoostingEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create file with frontmatter containing search term
	contentWithFM := `---
description: This is about golang programming
tags: [golang, programming]
---
# Some content
`
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "golang-guide.md"),
		[]byte(contentWithFM), 0600))

	// create file with search term only in filename
	contentNoFM := "# Content here"
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "other-file.md"),
		[]byte(contentNoFM), 0600))

	config := Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: "",
		MaxFileSize:    1024 * 1024,
		ServerName:     "test-server",
		Version:        "1.0.0",
	}

	srv, err := New(config)
	require.NoError(t, err)

	// search for "golang" - should rank golang-guide.md higher due to frontmatter boost
	result, err := srv.searchDocs(context.Background(), "golang")
	require.NoError(t, err)
	require.NotEmpty(t, result.Results, "should have search results")

	// golang-guide.md should be first due to frontmatter boost
	assert.Equal(t, "golang-guide.md", result.Results[0].Name, "file with frontmatter match should rank highest")
	assert.Greater(t, result.Results[0].Score, 1.0, "score should include frontmatter boost")
}
