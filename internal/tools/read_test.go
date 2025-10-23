package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/local-docs-mcp/internal/scanner"
)

func TestReadDoc(t *testing.T) {
	// create test directory structure
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(filepath.Join(commandsDir, "action"), 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	testContent := "# Test Document\n\nThis is a test."
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "action", "test.md"), []byte(testContent), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "doc.md"), []byte("# Doc"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# README"), 0600))

	sc := scanner.NewScanner(commandsDir, docsDir, tmpDir, 1024*1024)

	tests := []struct {
		name        string
		path        string
		source      *string
		wantContent string
		wantSource  string
		wantErr     bool
	}{
		{
			name:        "read with source prefix",
			path:        "commands:action/test.md",
			source:      nil,
			wantContent: testContent,
			wantSource:  "commands",
			wantErr:     false,
		},
		{
			name:        "read without source prefix",
			path:        "action/test.md",
			source:      stringPtr("commands"),
			wantContent: testContent,
			wantSource:  "commands",
			wantErr:     false,
		},
		{
			name:        "read from project-docs",
			path:        "project-docs:doc.md",
			source:      nil,
			wantContent: "# Doc",
			wantSource:  "project-docs",
			wantErr:     false,
		},
		{
			name:        "read from project-root",
			path:        "project-root:README.md",
			source:      nil,
			wantContent: "# README",
			wantSource:  "project-root",
			wantErr:     false,
		},
		{
			name:    "file not found",
			path:    "nonexistent.md",
			source:  stringPtr("commands"),
			wantErr: true,
		},
		{
			name:    "invalid source",
			path:    "test.md",
			source:  stringPtr("invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ReadDoc(sc, tt.path, tt.source, 1024*1024)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantContent, result.Content)
			assert.Equal(t, tt.wantSource, result.Source)
			assert.Greater(t, result.Size, 0)
			assert.NotEmpty(t, result.Path)
		})
	}
}

func TestReadDoc_FallbackToAllSources(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("commands"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "doc.md"), []byte("docs"), 0600))

	sc := scanner.NewScanner(commandsDir, docsDir, tmpDir, 1024*1024)

	// without source, should try all sources
	result, err := ReadDoc(sc, "test.md", nil, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, "commands", result.Content)
	assert.Equal(t, "commands", result.Source)
}

func TestReadDoc_FileTooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create large file
	largeContent := make([]byte, 2*1024*1024)
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "large.md"), largeContent, 0600))

	sc := scanner.NewScanner(commandsDir, "", "", 1024*1024)

	_, err := ReadDoc(sc, "large.md", stringPtr("commands"), 1024*1024)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestReadDoc_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	sc := scanner.NewScanner(commandsDir, "", "", 1024*1024)

	tests := []string{
		"../etc/passwd",
		"../../secret.md",
		"subdir/../../etc/passwd",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			_, err := ReadDoc(sc, path, stringPtr("commands"), 1024*1024)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "traversal")
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
