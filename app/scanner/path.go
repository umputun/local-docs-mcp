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

	userPath = filepath.Clean(userPath)

	// catch above-root traversal lexically. Clean preserves leading "..", so a
	// rooted match here is precise -- unlike strings.Contains(.., ".."), which
	// false-positives on legitimate filenames like "a..b.md" or "....md".
	if userPath == ".." || strings.HasPrefix(userPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal not allowed: %s", userPath)
	}

	absPath := filepath.Join(baseDir, userPath)

	info, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", userPath)
		}
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	// resolve symlinks on both ends and compare real paths so a symlink
	// inside baseDir cannot smuggle access to a target outside it.
	realBase, err := filepath.EvalSymlinks(filepath.Clean(baseDir))
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	relPath, err := filepath.Rel(realBase, realPath)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal not allowed: resolved path outside base directory")
	}

	// size check uses Stat (follows symlinks) so we measure the real target,
	// not the symlink entry from Lstat above.
	if info.Mode()&os.ModeSymlink != 0 {
		info, err = os.Stat(realPath)
		if err != nil {
			return "", fmt.Errorf("failed to stat file: %w", err)
		}
	}
	if info.Size() > maxSize {
		return "", fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxSize)
	}

	return absPath, nil
}
