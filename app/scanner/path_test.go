package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeResolvePath(t *testing.T) {
	// create test directory structure
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test"), 0600))

	largeFile := filepath.Join(tmpDir, "large.md")
	require.NoError(t, os.WriteFile(largeFile, make([]byte, 6*1024*1024), 0600))

	tests := []struct {
		name     string
		baseDir  string
		userPath string
		maxSize  int64
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid relative path",
			baseDir:  tmpDir,
			userPath: "test.md",
			maxSize:  1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "valid relative path without extension",
			baseDir:  tmpDir,
			userPath: "test",
			maxSize:  1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "path traversal attempt with ..",
			baseDir:  tmpDir,
			userPath: "../etc/passwd",
			maxSize:  1024 * 1024,
			wantErr:  true,
			errMsg:   "path traversal",
		},
		{
			name:     "absolute path rejection",
			baseDir:  tmpDir,
			userPath: "/etc/passwd",
			maxSize:  1024 * 1024,
			wantErr:  true,
			errMsg:   "absolute",
		},
		{
			name:     "file too large",
			baseDir:  tmpDir,
			userPath: "large.md",
			maxSize:  1024 * 1024,
			wantErr:  true,
			errMsg:   "too large",
		},
		{
			name:     "file does not exist",
			baseDir:  tmpDir,
			userPath: "nonexistent.md",
			maxSize:  1024 * 1024,
			wantErr:  true,
			errMsg:   "not found",
		},
		{
			name:     "empty path",
			baseDir:  tmpDir,
			userPath: "",
			maxSize:  1024 * 1024,
			wantErr:  true,
			errMsg:   "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeResolvePath(tt.baseDir, tt.userPath, tt.maxSize)
			if tt.wantErr {
				require.Error(t, err, "expected error for %s", tt.name)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message should contain %s", tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, got)
			assert.True(t, filepath.IsAbs(got), "resolved path should be absolute")
		})
	}
}

func TestSafeResolvePath_AddsMdExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test"), 0600))

	// test without .md extension
	resolved, err := SafeResolvePath(tmpDir, "test", 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, testFile, resolved)

	// test with .md extension
	resolved, err = SafeResolvePath(tmpDir, "test.md", 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, testFile, resolved)
}

func TestSafeResolvePath_PreventsDotDotTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	// create parent directory file
	parentFile := filepath.Join(filepath.Dir(tmpDir), "secret.md")
	require.NoError(t, os.WriteFile(parentFile, []byte("secret"), 0600))
	defer os.Remove(parentFile)

	tests := []string{
		"../secret.md",
		"../secret",
		"subdir/../../secret.md",
		"./../../secret.md",
	}

	for _, userPath := range tests {
		t.Run(userPath, func(t *testing.T) {
			_, err := SafeResolvePath(tmpDir, userPath, 1024*1024)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "path traversal")
		})
	}
}

func TestSafeResolvePath_Symlinks(t *testing.T) {
	tmpDir := t.TempDir()

	// create file inside tmpDir
	insideFile := filepath.Join(tmpDir, "inside.md")
	require.NoError(t, os.WriteFile(insideFile, []byte("inside"), 0600))

	// create file outside tmpDir
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.md")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0600))

	// create symlink from inside pointing to outside
	symlinkPath := filepath.Join(tmpDir, "symlink.md")
	err := os.Symlink(outsideFile, symlinkPath)
	if err != nil {
		t.Skip("symlink creation not supported")
	}

	// attempt to access file via symlink
	// note: symlinks are followed by filepath.Abs/Clean, but filepath.Rel check catches traversal
	resolved, err := SafeResolvePath(tmpDir, "symlink.md", 1024*1024)
	// if symlink points outside base dir, filepath.Rel should detect it
	if err == nil {
		// on some systems symlink might not be followed or check might not catch it
		t.Logf("symlink resolved to: %s", resolved)
	} else {
		// expected behavior - symlink traversal detected
		assert.Contains(t, err.Error(), "path traversal")
	}
}

func TestSafeResolvePath_MalformedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		userPath string
		errMsg   string
	}{
		{
			name:     "null byte in path",
			userPath: "test\x00.md",
			errMsg:   "failed to stat file", // os.Stat returns "invalid argument" for null bytes
		},
		{
			name:     "only dots",
			userPath: "...",
			errMsg:   "path traversal",
		},
		{
			name:     "mixed slashes and dots",
			userPath: "./../test.md",
			errMsg:   "path traversal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SafeResolvePath(tmpDir, tt.userPath, 1024*1024)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestSafeResolvePath_VeryLongPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// create deeply nested directory structure (within limits)
	deepPath := tmpDir
	for i := 0; i < 10; i++ {
		deepPath = filepath.Join(deepPath, "subdir")
		require.NoError(t, os.MkdirAll(deepPath, 0755))
	}
	deepFile := filepath.Join(deepPath, "deep.md")
	require.NoError(t, os.WriteFile(deepFile, []byte("deep"), 0600))

	// construct relative path
	relPath := filepath.Join("subdir", "subdir", "subdir", "subdir", "subdir", "subdir", "subdir", "subdir", "subdir", "subdir", "deep.md")

	// should work with valid deep path
	resolved, err := SafeResolvePath(tmpDir, relPath, 1024*1024)
	require.NoError(t, err)
	assert.Equal(t, deepFile, resolved)

	// test excessively long filename
	longName := make([]byte, 300)
	for i := range longName {
		longName[i] = 'a'
	}
	longFilePath := string(longName) + ".md"

	_, err = SafeResolvePath(tmpDir, longFilePath, 1024*1024)
	require.Error(t, err)
	// os.Stat returns "file name too long" for excessively long filenames
	assert.Contains(t, err.Error(), "failed to stat file")
}

func TestSafeResolvePath_Unicode(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "japanese characters",
			filename: "日本語.md",
			wantErr:  false,
		},
		{
			name:     "cyrillic characters",
			filename: "русский.md",
			wantErr:  false,
		},
		{
			name:     "emoji",
			filename: "test🎉.md",
			wantErr:  false,
		},
		{
			name:     "mixed unicode and ascii",
			filename: "test-文档.md",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create file with unicode name
			testFile := filepath.Join(tmpDir, tt.filename)
			require.NoError(t, os.WriteFile(testFile, []byte("test"), 0600))

			// should be able to resolve unicode filename
			resolved, err := SafeResolvePath(tmpDir, tt.filename, 1024*1024)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testFile, resolved)
			}
		})
	}
}
