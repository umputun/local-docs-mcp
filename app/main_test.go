package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testBinaryPath string
	buildOnce      sync.Once
	buildErr       error
)

// buildTestBinary builds the binary once for all integration tests
func buildTestBinary() (string, error) {
	buildOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "local-docs-mcp-test-*")
		if err != nil {
			buildErr = err
			return
		}

		testBinaryPath = filepath.Join(tmpDir, "local-docs-mcp-test")
		buildCmd := exec.Command("go", "build", "-o", testBinaryPath, ".")
		buildErr = buildCmd.Run()
	})

	return testBinaryPath, buildErr
}

func TestIntegration_Server(t *testing.T) {
	// skip if in short mode
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// create test directory structure
	tmpDir := t.TempDir()
	sharedDocsDir := filepath.Join(tmpDir, "shared")
	projectDocsDir := filepath.Join(tmpDir, "docs")

	// create shared docs structure
	require.NoError(t, os.MkdirAll(filepath.Join(sharedDocsDir, "action"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(sharedDocsDir, "action", "commit.md"),
		[]byte("# Commit Action\nHow to commit code."),
		0600,
	))

	// create project docs structure
	require.NoError(t, os.MkdirAll(filepath.Join(projectDocsDir, "plans"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDocsDir, "architecture.md"),
		[]byte("# Architecture\nSystem architecture docs."),
		0600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDocsDir, "plans", "excluded.md"),
		[]byte("# Plan\nShould be excluded."),
		0600,
	))

	// create project root file
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "README.md"),
		[]byte("# Test Project\nTest README."),
		0600,
	))

	// build test binary
	binaryPath, err := buildTestBinary()
	require.NoError(t, err, "failed to build test binary")

	// create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}, nil)

	// create command to run server
	cmd := exec.Command(
		binaryPath,
		"--shared-docs-dir="+sharedDocsDir,
		"--docs-dir=docs",
		"--enable-root-docs",
	)
	cmd.Dir = tmpDir
	transport := &mcp.CommandTransport{Command: cmd}

	// connect to server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err, "failed to connect to server")
	defer session.Close()

	// test list_all_docs tool
	t.Run("list_all_docs", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "list_all_docs",
			Arguments: map[string]any{},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.Content)

		// verify response contains expected files
		content := result.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, content, "commit.md", "should list shared docs")
		assert.Contains(t, content, "architecture.md", "should list project docs")
		assert.Contains(t, content, "README.md", "should list root files")
		assert.NotContains(t, content, "excluded.md", "should not list plans directory")
	})

	// test search_docs tool
	t.Run("search_docs", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "search_docs",
			Arguments: map[string]any{"query": "arch"},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.Content)

		content := result.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, content, "architecture.md", "should find architecture doc")
	})

	// test read_doc tool
	t.Run("read_doc", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "read_doc",
			Arguments: map[string]any{"path": "project-docs:architecture.md"},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.Content)

		content := result.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, content, "System architecture docs", "should read file content")
	})

	// test read_doc with source auto-detection
	t.Run("read_doc_auto_source", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "read_doc",
			Arguments: map[string]any{"path": "README.md"},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.Content)

		content := result.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, content, "Test Project", "should find and read README")
	})
}

func TestIntegration_ServerWithCache(t *testing.T) {
	// skip if in short mode
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// create test directory structure
	tmpDir := t.TempDir()
	sharedDocsDir := filepath.Join(tmpDir, "shared")

	// create minimal docs structure
	require.NoError(t, os.MkdirAll(sharedDocsDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(sharedDocsDir, "test.md"),
		[]byte("# Test\nTest content."),
		0600,
	))

	// build test binary
	binaryPath, err := buildTestBinary()
	require.NoError(t, err, "failed to build test binary")

	// create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}, nil)

	// create command to run server (caching is always enabled)
	cmd := exec.Command(
		binaryPath,
		"--shared-docs-dir="+sharedDocsDir,
		"--cache-ttl=1m",
	)
	transport := &mcp.CommandTransport{Command: cmd}

	// connect to server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err, "failed to connect to server")
	defer session.Close()

	// first call to populate cache
	result1, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_all_docs",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result1.Content)

	// second call should use cache
	result2, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_all_docs",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result2.Content)

	// results should be identical
	assert.Equal(t, result1.Content[0].(*mcp.TextContent).Text, result2.Content[0].(*mcp.TextContent).Text)
}

func TestExpandTilde(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "tilde prefix",
			input:    "~/.claude/commands",
			expected: filepath.Join(homeDir, ".claude/commands"),
			wantErr:  false,
		},
		{
			name:     "tilde only",
			input:    "~/",
			expected: homeDir,
			wantErr:  false,
		},
		{
			name:     "no tilde",
			input:    "/absolute/path",
			expected: "/absolute/path",
			wantErr:  false,
		},
		{
			name:     "relative path",
			input:    "relative/path",
			expected: "relative/path",
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "tilde not at start",
			input:    "/path/~/file",
			expected: "/path/~/file",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandTilde(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRun(t *testing.T) {
	// subtest 1: invalid config (doesn't start server, always works)
	t.Run("invalid config", func(t *testing.T) {
		tmpDir := t.TempDir()

		// change to tmpDir for test
		oldDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(oldDir)

		// create options with invalid config (zero max file size)
		opts := Options{
			SharedDocsDir:  tmpDir,
			ProjectDocsDir: "docs",
			MaxFileSize:    0, // invalid - must be > 0
			CacheTTL:       1 * time.Hour,
			ExcludeDirs:    []string{"plans"},
		}

		ctx := context.Background()

		// run should return error due to invalid config
		err = run(ctx, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create server")
	})

	t.Run("success with cancelled context", func(t *testing.T) {
		tmpDir := t.TempDir()
		sharedDocsDir := filepath.Join(tmpDir, "shared")

		// create directory
		require.NoError(t, os.MkdirAll(sharedDocsDir, 0755))

		// change to tmpDir for test
		oldDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(oldDir)

		// redirect stdout to avoid "write /dev/stdout: file already closed" error during coverage
		oldStdout := os.Stdout
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		require.NoError(t, err)
		os.Stdout = devNull
		defer func() {
			devNull.Close()
			os.Stdout = oldStdout
		}()

		// create options
		opts := Options{
			SharedDocsDir:  sharedDocsDir,
			ProjectDocsDir: "docs",
			MaxFileSize:    1024 * 1024,
			CacheTTL:       1 * time.Hour,
			ExcludeDirs:    []string{"plans"},
		}

		// use pre-cancelled context so server exits immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// run with cancelled context
		err = run(ctx, opts)
		// should handle cancellation gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "context canceled")
		}
	})
}
