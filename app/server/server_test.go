package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		name           string
		query          string
		normalizedName string
		wantScore      float64
		checkFunc      func(t *testing.T, score float64)
	}{
		{
			name:           "exact match",
			query:          "test",
			normalizedName: "test",
			wantScore:      1.0,
		},
		{
			name:           "exact match with .md extension",
			query:          "test",
			normalizedName: "test.md",
			wantScore:      1.0,
		},
		{
			name:           "exact match already has .md",
			query:          "test.md",
			normalizedName: "test.md",
			wantScore:      1.0,
		},
		{
			name:           "substring match - half the name",
			query:          "test",
			normalizedName: "test-command",
			checkFunc: func(t *testing.T, score float64) {
				// score should be 0.8 * (4/12) = 0.267
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 1.0)
				assert.Greater(t, score, 0.2)
				assert.Less(t, score, 0.3)
			},
		},
		{
			name:           "substring match - most of the name",
			query:          "architecture",
			normalizedName: "architecture-guide",
			checkFunc: func(t *testing.T, score float64) {
				// score should be 0.8 * (12/18) = 0.533
				assert.Greater(t, score, 0.5)
				assert.Less(t, score, 0.6)
			},
		},
		{
			name:           "substring match - small query in long name",
			query:          "ab",
			normalizedName: "abcdefghijklmnop",
			checkFunc: func(t *testing.T, score float64) {
				// score should be 0.8 * (2/16) = 0.1
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 0.15)
			},
		},
		{
			name:           "fuzzy match above threshold",
			query:          "test",
			normalizedName: "t-e-s-t",
			checkFunc: func(t *testing.T, score float64) {
				// sahilm/fuzzy can match chars in sequence with gaps
				// score should be > 0 and scaled down (< exact/substring)
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 0.7)
				assert.Greater(t, score, 0.4) // should be ~0.469 (67/100 * 0.7)
			},
		},
		{
			name:           "fuzzy match below threshold - weak subsequence",
			query:          "tst",
			normalizedName: "test",
			wantScore:      0.0, // score too low, below 0.3 threshold
		},
		{
			name:           "no fuzzy match - chars out of order",
			query:          "tset",
			normalizedName: "test",
			wantScore:      0.0, // sahilm/fuzzy can't match transpositions
		},
		{
			name:           "no match - completely different",
			query:          "xyz",
			normalizedName: "abc",
			wantScore:      0.0,
		},
		{
			name:           "no match - very different strings",
			query:          "architecture",
			normalizedName: "zzzzzz",
			wantScore:      0.0,
		},
		{
			name:           "case sensitivity already normalized",
			query:          "test",
			normalizedName: "test",
			wantScore:      1.0,
		},
		{
			name:           "empty query",
			query:          "",
			normalizedName: "test",
			wantScore:      0.0,
		},
		{
			name:           "empty normalized name",
			query:          "test",
			normalizedName: "",
			wantScore:      0.0,
		},
		{
			name:           "both empty",
			query:          "",
			normalizedName: "",
			wantScore:      1.0, // empty == empty
		},
		{
			name:           "substring at end",
			query:          "file",
			normalizedName: "test-file",
			checkFunc: func(t *testing.T, score float64) {
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 1.0)
			},
		},
		{
			name:           "substring at start",
			query:          "test",
			normalizedName: "test-file",
			checkFunc: func(t *testing.T, score float64) {
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 1.0)
			},
		},
		{
			name:           "substring in middle",
			query:          "mid",
			normalizedName: "start-mid-end",
			checkFunc: func(t *testing.T, score float64) {
				assert.Greater(t, score, 0.0)
				assert.Less(t, score, 1.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := srv.calculateScore(tt.query, tt.normalizedName)

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
		name           string
		query          string
		normalizedName string
	}{
		{
			name:           "very poor fuzzy match",
			query:          "abcdefgh",
			normalizedName: "zyxwvuts",
		},
		{
			name:           "single char query no match",
			query:          "x",
			normalizedName: "test",
		},
		{
			name:           "completely unrelated",
			query:          "documentation",
			normalizedName: "zzzzz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := srv.calculateScore(tt.query, tt.normalizedName)
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
		name  string
	}{
		{"doc", "docs"},   // doc is substring of docs
		{"test", "ttest"}, // test is substring of ttest
		{"git", "gitt"},   // git is substring of gitt
	}

	for _, tt := range substringCaught {
		score := srv.calculateScore(tt.query, tt.name)
		// these all match via substring, not fuzzy
		assert.Greater(t, score, 0.0, "should match via substring")
		assert.LessOrEqual(t, score, 0.8, "substring matches score <= 0.8")
	}

	// successful fuzzy matches (not exact, not substring)
	fuzzyMatches := []struct {
		query string
		name  string
	}{
		{"test", "t-e-s-t"},          // chars with separators
		{"cmd", "c-o-m-m-a-n-d"},     // subsequence with gaps
		{"git", "g-i-t-c-o-m-m-i-t"}, // prefix subsequence
		{"comit", "commit"},          // missing char (still valid subsequence)
	}

	for _, tt := range fuzzyMatches {
		score := srv.calculateScore(tt.query, tt.name)
		assert.Greater(t, score, 0.0, "fuzzy match should score > 0")
		assert.Less(t, score, 0.8, "fuzzy matches score < substring matches")
	}

	// sahilm/fuzzy limitations - cannot match these
	noFuzzyMatch := []struct {
		query  string
		name   string
		reason string
	}{
		{"tset", "test", "chars out of order (transposition)"},
		{"tst", "test", "weak subsequence below threshold"},
		{"xyz", "abc", "no common chars"},
	}

	for _, tt := range noFuzzyMatch {
		score := srv.calculateScore(tt.query, tt.name)
		assert.Equal(t, 0.0, score, "should not match: %s", tt.reason)
	}
}
