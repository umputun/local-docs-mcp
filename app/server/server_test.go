package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/local-docs-mcp/app/tools"
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
			input := tools.SearchInput{Query: tt.query}

			result, output, err := srv.handleSearchDocs(ctx, req, input)
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotNil(t, output)

			// verify output structure
			searchOutput, ok := output.(*tools.SearchOutput)
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
			input := tools.ReadInput{Path: tt.path, Source: tt.source}

			result, output, err := srv.handleReadDoc(ctx, req, input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotNil(t, output)

			// verify output structure
			readOutput, ok := output.(*tools.ReadOutput)
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
	listOutput, ok := output.(*tools.ListOutput)
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

	tests := []struct {
		name        string
		config      Config
		enableCache bool
	}{
		{
			name: "close server with regular scanner",
			config: Config{
				CommandsDir:    commandsDir,
				ProjectDocsDir: tmpDir,
				ProjectRootDir: "",
				MaxFileSize:    1024 * 1024,
				ServerName:     "test",
				Version:        "1.0",
				EnableCache:    false,
			},
			enableCache: false,
		},
		{
			name: "close server with cached scanner",
			config: Config{
				CommandsDir:    commandsDir,
				ProjectDocsDir: tmpDir,
				ProjectRootDir: "",
				MaxFileSize:    1024 * 1024,
				ServerName:     "test",
				Version:        "1.0",
				EnableCache:    true,
			},
			enableCache: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := New(tt.config)
			require.NoError(t, err)
			require.NotNil(t, srv)

			// close server
			err = srv.Close()
			assert.NoError(t, err)

			// close should be idempotent
			err = srv.Close()
			assert.NoError(t, err)
		})
	}
}

func TestServer_Close_NilScanner(t *testing.T) {
	// test close with nil scanner (shouldn't happen in practice but test defensively)
	srv := &Server{
		scanner: nil,
	}

	err := srv.Close()
	assert.NoError(t, err)
}
