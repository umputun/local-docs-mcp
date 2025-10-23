package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/local-docs-mcp/internal/scanner"
)

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
	sc := scanner.NewScanner(commandsDir, docsDir, tmpDir, 1024*1024)

	// test list
	result, err := ListAllDocs(sc, 1024*1024)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, result.Total)
	assert.Len(t, result.Docs, 3)

	// verify doc info
	for _, doc := range result.Docs {
		assert.NotEmpty(t, doc.Name)
		assert.NotEmpty(t, doc.Filename)
		assert.NotEmpty(t, doc.Source)
		assert.Greater(t, doc.Size, int64(0))
		assert.False(t, doc.TooLarge)
	}
}

func TestListAllDocs_TooLargeFlag(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create small file
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "small.md"), []byte("small"), 0600))

	// create large file
	largeContent := make([]byte, 2*1024*1024)
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "large.md"), largeContent, 0600))

	sc := scanner.NewScanner(commandsDir, "", "", 1024*1024)
	result, err := ListAllDocs(sc, 1024*1024)
	require.NoError(t, err)

	// find large file
	var largeDoc *DocInfo
	for i := range result.Docs {
		if result.Docs[i].Name == "large.md" {
			largeDoc = &result.Docs[i]
			break
		}
	}

	require.NotNil(t, largeDoc, "large file should be in results")
	assert.True(t, largeDoc.TooLarge, "large file should be marked as too large")
	assert.Greater(t, largeDoc.Size, int64(1024*1024))
}

func TestListAllDocs_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	sc := scanner.NewScanner(commandsDir, "", "", 1024*1024)
	result, err := ListAllDocs(sc, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.Docs)
}

func TestListAllDocs_NonExistentDirectories(t *testing.T) {
	sc := scanner.NewScanner("/nonexistent/commands", "/nonexistent/docs", "/nonexistent/root", 1024*1024)
	result, err := ListAllDocs(sc, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.Docs)
}
