package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo contains metadata about a documentation file
type FileInfo struct {
	Name        string   // original filename
	Filename    string   // filename with source prefix (e.g., "commands:action/commit.md")
	Normalized  string   // lowercase for matching
	Source      Source   // source type
	Path        string   // absolute path
	Size        int64    // file size in bytes
	Description string   // description from frontmatter (if present)
	Tags        []string // tags from frontmatter (if present)
}

// SafeResolvePath resolves a user-provided path relative to baseDir with security checks.
// It prevents path traversal, validates file existence and size, and adds .md extension if missing.
func SafeResolvePath(baseDir, userPath string, maxSize int64) (string, error) {
	// reject empty path
	if userPath == "" {
		return "", fmt.Errorf("empty path provided")
	}

	// reject absolute paths
	if filepath.IsAbs(userPath) {
		return "", fmt.Errorf("absolute paths not allowed: %s", userPath)
	}

	// add .md extension if missing
	if !strings.HasSuffix(userPath, ".md") {
		userPath += ".md"
	}

	// clean the path to normalize it
	userPath = filepath.Clean(userPath)

	// check for path traversal attempts
	if strings.Contains(userPath, "..") {
		return "", fmt.Errorf("path traversal not allowed: %s", userPath)
	}

	// resolve to absolute path
	absPath := filepath.Join(baseDir, userPath)

	// verify the resolved path is still within baseDir
	cleanBase := filepath.Clean(baseDir)
	cleanPath := filepath.Clean(absPath)

	relPath, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path traversal not allowed: resolved path outside base directory")
	}

	// check file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", userPath)
		}
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	// check file size
	if info.Size() > maxSize {
		return "", fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxSize)
	}

	return absPath, nil
}
