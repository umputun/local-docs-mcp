package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScanner(t *testing.T) {
	scanner := NewScanner(Params{
		CommandsDir:    "/commands",
		ProjectDocsDir: "/docs",
		ProjectRootDir: "/root",
		MaxFileSize:    1024 * 1024,
		ExcludeDirs:    []string{"plans"},
	})
	assert.NotNil(t, scanner)
	assert.Equal(t, "/commands", scanner.commandsDir)
	assert.Equal(t, "/docs", scanner.projectDocsDir)
	assert.Equal(t, "/root", scanner.projectRootDir)
	assert.Equal(t, int64(1024*1024), scanner.maxFileSize)
	assert.Equal(t, []string{"plans"}, scanner.excludeDirs)
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

	scanner := NewScanner(Params{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: rootDir,
		MaxFileSize:    1024 * 1024,
		ExcludeDirs:    []string{"plans"},
	})
	files, err := scanner.Scan(context.Background())
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
	scanner := NewScanner(Params{
		CommandsDir:    "/nonexistent/commands",
		ProjectDocsDir: "/nonexistent/docs",
		ProjectRootDir: "/nonexistent/root",
		MaxFileSize:    1024 * 1024,
		ExcludeDirs:    []string{"plans"},
	})
	files, err := scanner.Scan(context.Background())
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

	scanner := NewScanner(Params{
		CommandsDir: commandsDir,
		MaxFileSize: 1024 * 1024,
		ExcludeDirs: []string{"plans"},
	})
	files, err := scanner.Scan(context.Background())
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

	scanner := NewScanner(Params{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: tmpDir,
		MaxFileSize:    1024 * 1024,
		ExcludeDirs:    []string{"plans"},
	})
	files, err := scanner.Scan(context.Background())
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

	scanner := NewScanner(Params{
		CommandsDir: commandsDir,
		MaxFileSize: 1024 * 1024,
		ExcludeDirs: []string{"plans"},
	})
	files, err := scanner.Scan(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, files)

	// verify normalized name is lowercase
	assert.Equal(t, "test-file.md", files[0].Normalized, "normalized name should be lowercase")
	assert.Equal(t, "Test-File.md", files[0].Name, "original name should be preserved")
}

func TestScanner_scanRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")

	// create recursive directory structure for project docs
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "architecture"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "guides", "deep"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, ".hidden"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "plans"), 0755))

	// create files at various levels
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "root.md"), []byte("root"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "architecture", "system.md"), []byte("system"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "guides", "test.md"), []byte("test"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "guides", "deep", "nested.md"), []byte("nested"), 0600))

	// create hidden files (should be excluded)
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, ".hidden.md"), []byte("hidden"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, ".hidden", "file.md"), []byte("hidden"), 0600))

	// create excluded directory files (should be excluded for project-docs)
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "plans", "plan.md"), []byte("plan"), 0600))

	scanner := NewScanner(Params{
		ProjectDocsDir: docsDir,
		MaxFileSize:    1024 * 1024,
		ExcludeDirs:    []string{"plans"},
	})

	ctx := context.Background()

	tests := []struct {
		name          string
		source        Source
		dir           string
		wantFileCount int
		wantFiles     []string
		excludePlans  bool
	}{
		{
			name:          "recursive scan excludes plans for project-docs",
			source:        SourceProjectDocs,
			dir:           docsDir,
			wantFileCount: 4,
			wantFiles:     []string{"root.md", "system.md", "test.md", "nested.md"},
			excludePlans:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := scanner.scanRecursive(ctx, tt.source, tt.dir)
			require.NoError(t, err)
			assert.Len(t, files, tt.wantFileCount)

			// verify all expected files are present
			fileNames := make(map[string]bool)
			for _, f := range files {
				fileNames[f.Name] = true
			}
			for _, want := range tt.wantFiles {
				assert.True(t, fileNames[want], "expected file %s not found", want)
			}

			// verify hidden files are excluded
			for _, f := range files {
				assert.False(t, strings.HasPrefix(f.Name, "."), "hidden file %s should be excluded", f.Name)
			}

			// verify excluded directory files are not present (only for project-docs)
			if tt.excludePlans {
				for _, f := range files {
					assert.False(t, strings.Contains(f.Path, "/plans/"), "file from excluded directory should not be present: %s", f.Path)
				}
			}

			// verify all files have correct source prefix
			for _, f := range files {
				assert.True(t, strings.HasPrefix(f.Filename, string(tt.source)+":"), "file should have source prefix")
			}
		})
	}
}

func TestScanner_scanRecursive_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test"), 0600))

	scanner := NewScanner(Params{
		CommandsDir: commandsDir,
		MaxFileSize: 1024 * 1024,
		ExcludeDirs: []string{"plans"},
	})

	// create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scanner.scanRecursive(ctx, SourceCommands, commandsDir)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestScanner_scanFlat(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := tmpDir

	// create files in root
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "README.md"), []byte("readme"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "LICENSE.md"), []byte("license"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CHANGELOG.md"), []byte("changelog"), 0600))

	// create subdirectory with files (should NOT be scanned)
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "docs"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "docs", "nested.md"), []byte("nested"), 0600))

	// create hidden files (should be excluded)
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, ".hidden.md"), []byte("hidden"), 0600))

	// create non-md files (should be excluded)
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "test.txt"), []byte("text"), 0600))

	scanner := NewScanner(Params{
		ProjectRootDir: rootDir,
		MaxFileSize:    1024 * 1024,
		ExcludeDirs:    []string{"plans"},
	})

	ctx := context.Background()

	tests := []struct {
		name          string
		source        Source
		dir           string
		wantFileCount int
		wantFiles     []string
	}{
		{
			name:          "flat scan finds only root level md files",
			source:        SourceProjectRoot,
			dir:           rootDir,
			wantFileCount: 3,
			wantFiles:     []string{"README.md", "LICENSE.md", "CHANGELOG.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := scanner.scanFlat(ctx, tt.source, tt.dir)
			require.NoError(t, err)
			assert.Len(t, files, tt.wantFileCount)

			// verify all expected files are present
			fileNames := make(map[string]bool)
			for _, f := range files {
				fileNames[f.Name] = true
			}
			for _, want := range tt.wantFiles {
				assert.True(t, fileNames[want], "expected file %s not found", want)
			}

			// verify hidden files are excluded
			for _, f := range files {
				assert.False(t, strings.HasPrefix(f.Name, "."), "hidden file %s should be excluded", f.Name)
			}

			// verify nested files are not present
			for _, f := range files {
				assert.False(t, strings.Contains(f.Name, "nested"), "nested file should not be present in flat scan")
			}

			// verify all files have correct source prefix
			for _, f := range files {
				assert.True(t, strings.HasPrefix(f.Filename, string(tt.source)+":"), "file should have source prefix")
			}
		})
	}
}

func TestScanner_scanFlat_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("test"), 0600))

	scanner := NewScanner(Params{
		ProjectRootDir: tmpDir,
		MaxFileSize:    1024 * 1024,
		ExcludeDirs:    []string{"plans"},
	})

	// create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scanner.scanFlat(ctx, SourceProjectRoot, tmpDir)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestScanner_shouldExcludeDir(t *testing.T) {
	scanner := NewScanner(Params{
		ExcludeDirs: []string{"plans", "vendor", "node_modules"},
	})

	tests := []struct {
		name     string
		dirName  string
		expected bool
	}{
		{name: "plans directory", dirName: "plans", expected: true},
		{name: "vendor directory", dirName: "vendor", expected: true},
		{name: "node_modules directory", dirName: "node_modules", expected: true},
		{name: "docs directory", dirName: "docs", expected: false},
		{name: "src directory", dirName: "src", expected: false},
		{name: "empty name", dirName: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.shouldExcludeDir(tt.dirName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScanner_Close(t *testing.T) {
	// test that Close is a no-op and always returns nil
	tmpDir := t.TempDir()
	scanner := NewScanner(Params{
		CommandsDir:    tmpDir,
		ProjectDocsDir: tmpDir,
		ProjectRootDir: tmpDir,
		MaxFileSize:    1024,
	})

	// close should succeed
	err := scanner.Close()
	assert.NoError(t, err)

	// close should be idempotent
	err = scanner.Close()
	assert.NoError(t, err)
}

func TestScanner_DirectoryGetters(t *testing.T) {
	commandsDir := "/commands"
	docsDir := "/docs"
	rootDir := "/root"

	scanner := NewScanner(Params{
		CommandsDir:    commandsDir,
		ProjectDocsDir: docsDir,
		ProjectRootDir: rootDir,
		MaxFileSize:    1024,
	})

	assert.Equal(t, commandsDir, scanner.CommandsDir())
	assert.Equal(t, docsDir, scanner.ProjectDocsDir())
	assert.Equal(t, rootDir, scanner.ProjectRootDir())
}
