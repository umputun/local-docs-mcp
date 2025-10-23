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
