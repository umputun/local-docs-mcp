package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScanner(t *testing.T) {
	scanner := NewScanner("/commands", "/docs", "/root", 1024*1024)
	assert.NotNil(t, scanner)
	assert.Equal(t, "/commands", scanner.commandsDir)
	assert.Equal(t, "/docs", scanner.projectDocsDir)
	assert.Equal(t, "/root", scanner.projectRootDir)
	assert.Equal(t, int64(1024*1024), scanner.maxFileSize)
}

func TestScanner_Scan(t *testing.T) {
	// create test directory structure
	tmpDir := t.TempDir()

	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")
	rootDir := tmpDir

	// create commands structure
	require.NoError(t, os.MkdirAll(filepath.Join(commandsDir, "action"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(commandsDir, "knowledge"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "action", "commit.md"), []byte("# Commit"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "knowledge", "routegroup.md"), []byte("# Routegroup"), 0600))

	// create docs structure (excluding plans)
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "plans"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte("# Arch"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "plans", "migration.md"), []byte("# Plan"), 0600))

	// create root files
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "README.md"), []byte("# README"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("# CLAUDE"), 0600))

	// create hidden file (should be excluded)
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, ".hidden.md"), []byte("# Hidden"), 0600))

	scanner := NewScanner(commandsDir, docsDir, rootDir, 1024*1024)
	files, err := scanner.Scan()
	require.NoError(t, err)

	// verify results
	assert.NotEmpty(t, files)

	// check commands files
	hasCommit := false
	hasRoutegroup := false
	hasArchitecture := false
	hasREADME := false
	hasCLAUDE := false
	hasPlan := false
	hasHidden := false

	for _, f := range files {
		if strings.Contains(f.Filename, "action/commit.md") {
			hasCommit = true
			assert.Equal(t, SourceCommands, f.Source)
			assert.Contains(t, f.Normalized, "commit")
		}
		if strings.Contains(f.Filename, "knowledge/routegroup.md") {
			hasRoutegroup = true
			assert.Equal(t, SourceCommands, f.Source)
		}
		if strings.Contains(f.Filename, "architecture.md") {
			hasArchitecture = true
			assert.Equal(t, SourceProjectDocs, f.Source)
		}
		if strings.Contains(f.Filename, "README.md") {
			hasREADME = true
			assert.Equal(t, SourceProjectRoot, f.Source)
		}
		if strings.Contains(f.Filename, "CLAUDE.md") {
			hasCLAUDE = true
			assert.Equal(t, SourceProjectRoot, f.Source)
		}
		if strings.Contains(f.Filename, "plans") {
			hasPlan = true
		}
		if strings.Contains(f.Filename, ".hidden") {
			hasHidden = true
		}
	}

	assert.True(t, hasCommit, "should find commit.md in commands")
	assert.True(t, hasRoutegroup, "should find routegroup.md in commands")
	assert.True(t, hasArchitecture, "should find architecture.md in docs")
	assert.True(t, hasREADME, "should find README.md in root")
	assert.True(t, hasCLAUDE, "should find CLAUDE.md in root")
	assert.False(t, hasPlan, "should exclude plans directory")
	assert.False(t, hasHidden, "should exclude hidden files")
}

func TestScanner_Scan_MissingDirectories(t *testing.T) {
	// test with non-existent directories
	scanner := NewScanner("/nonexistent/commands", "/nonexistent/docs", "/nonexistent/root", 1024*1024)
	files, err := scanner.Scan()
	require.NoError(t, err, "should not error on missing directories")
	assert.Empty(t, files, "should return empty list for missing directories")
}

func TestScanner_Scan_FileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create small file
	smallFile := filepath.Join(commandsDir, "small.md")
	require.NoError(t, os.WriteFile(smallFile, []byte("small"), 0600))

	// create large file
	largeFile := filepath.Join(commandsDir, "large.md")
	require.NoError(t, os.WriteFile(largeFile, make([]byte, 2*1024*1024), 0600))

	scanner := NewScanner(commandsDir, "", "", 1024*1024)
	files, err := scanner.Scan()
	require.NoError(t, err)

	// verify both files are in results
	hasSmall := false
	hasLarge := false
	var largeFileInfo *FileInfo

	for i := range files {
		if strings.Contains(files[i].Name, "small.md") {
			hasSmall = true
		}
		if strings.Contains(files[i].Name, "large.md") {
			hasLarge = true
			largeFileInfo = &files[i]
		}
	}

	assert.True(t, hasSmall, "should include small file")
	assert.True(t, hasLarge, "should list large file")

	// verify large file is marked appropriately
	if largeFileInfo != nil {
		assert.Greater(t, largeFileInfo.Size, int64(1024*1024), "large file size should be recorded")
	}
}

func TestScanner_SourcePrefixes(t *testing.T) {
	tmpDir := t.TempDir()

	commandsDir := filepath.Join(tmpDir, "commands")
	docsDir := filepath.Join(tmpDir, "docs")

	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(docsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "doc.md"), []byte("doc"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("readme"), 0600))

	scanner := NewScanner(commandsDir, docsDir, tmpDir, 1024*1024)
	files, err := scanner.Scan()
	require.NoError(t, err)

	// verify source prefixes
	for _, f := range files {
		if strings.Contains(f.Name, "test.md") {
			assert.True(t, strings.HasPrefix(f.Filename, "commands:"), "commands file should have commands: prefix")
		}
		if strings.Contains(f.Name, "doc.md") {
			assert.True(t, strings.HasPrefix(f.Filename, "project-docs:"), "docs file should have project-docs: prefix")
		}
		if strings.Contains(f.Name, "README.md") {
			assert.True(t, strings.HasPrefix(f.Filename, "project-root:"), "root file should have project-root: prefix")
		}
	}
}

func TestScanner_NormalizedNames(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "Test-File.md"), []byte("test"), 0600))

	scanner := NewScanner(commandsDir, "", "", 1024*1024)
	files, err := scanner.Scan()
	require.NoError(t, err)
	require.NotEmpty(t, files)

	// verify normalized name is lowercase
	assert.Equal(t, "test-file.md", files[0].Normalized, "normalized name should be lowercase")
	assert.Equal(t, "Test-File.md", files[0].Name, "original name should be preserved")
}
