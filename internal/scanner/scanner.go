package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Source represents documentation source type
type Source string

const (
	// SourceCommands represents ~/.claude/commands documentation
	SourceCommands Source = "commands"
	// SourceProjectDocs represents project docs (excluding plans/)
	SourceProjectDocs Source = "project-docs"
	// SourceProjectRoot represents root-level markdown files
	SourceProjectRoot Source = "project-root"
)

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

// FileInfo contains metadata about a documentation file
type FileInfo struct {
	Name       string // original filename
	Filename   string // filename with source prefix (e.g., "commands:action/commit.md")
	Normalized string // lowercase for matching
	Source     Source // source type
	Path       string // absolute path
	Size       int64  // file size in bytes
}

// Scanner discovers and indexes documentation files from multiple sources
type Scanner struct {
	commandsDir    string
	projectDocsDir string
	projectRootDir string
	maxFileSize    int64
}

// NewScanner creates a new scanner instance
func NewScanner(commandsDir, projectDocsDir, projectRootDir string, maxFileSize int64) *Scanner {
	return &Scanner{
		commandsDir:    commandsDir,
		projectDocsDir: projectDocsDir,
		projectRootDir: projectRootDir,
		maxFileSize:    maxFileSize,
	}
}

// CommandsDir returns the commands directory path
func (s *Scanner) CommandsDir() string {
	return s.commandsDir
}

// ProjectDocsDir returns the project docs directory path
func (s *Scanner) ProjectDocsDir() string {
	return s.projectDocsDir
}

// ProjectRootDir returns the project root directory path
func (s *Scanner) ProjectRootDir() string {
	return s.projectRootDir
}

// Scan discovers all markdown files from all configured sources
func (s *Scanner) Scan(ctx context.Context) ([]FileInfo, error) {
	var results []FileInfo

	// check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
	default:
	}

	// scan commands directory recursively
	commandFiles, err := s.scanSource(ctx, SourceCommands, s.commandsDir, "**/*.md")
	if err != nil {
		// don't fail if directory doesn't exist
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		results = append(results, commandFiles...)
	}

	// check context between scans
	select {
	case <-ctx.Done():
		return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
	default:
	}

	// scan project docs (excluding plans/)
	docFiles, err := s.scanSource(ctx, SourceProjectDocs, s.projectDocsDir, "**/*.md")
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		results = append(results, docFiles...)
	}

	// check context between scans
	select {
	case <-ctx.Done():
		return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
	default:
	}

	// scan project root (only .md files in root, not subdirectories)
	rootFiles, err := s.scanSource(ctx, SourceProjectRoot, s.projectRootDir, "*.md")
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		results = append(results, rootFiles...)
	}

	return results, nil
}

// scanSource scans a single source directory for markdown files
func (s *Scanner) scanSource(ctx context.Context, source Source, dir, pattern string) ([]FileInfo, error) {
	var results []FileInfo

	// check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
	default:
	}

	// check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return results, err // nolint:wrapcheck // returning os error as-is is acceptable
	}

	// determine if recursive scan needed
	recursive := strings.Contains(pattern, "**")

	if recursive { // nolint:nestif // directory walking requires nested conditions
		// walk directory tree
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			// check context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err != nil {
				return nil // skip errors
			}

			// skip hidden files and directories
			if strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			// exclude docs/plans/ directory
			if d.IsDir() && source == SourceProjectDocs {
				if d.Name() == "plans" {
					return fs.SkipDir
				}
			}

			// process only .md files
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
				info, err := d.Info()
				if err != nil {
					return nil // skip files we can't stat
				}

				relPath, err := filepath.Rel(dir, path)
				if err != nil {
					return nil
				}

				fileInfo := FileInfo{
					Name:       filepath.Base(path),
					Filename:   string(source) + ":" + filepath.ToSlash(relPath),
					Normalized: strings.ToLower(filepath.Base(path)),
					Source:     source,
					Path:       path,
					Size:       info.Size(),
				}
				results = append(results, fileInfo)
			}

			return nil
		})
		if err != nil {
			return nil, err // nolint:wrapcheck // filepath.WalkDir error is descriptive as-is
		}
	} else {
		// non-recursive: only scan immediate directory
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err // nolint:wrapcheck // os.ReadDir error is descriptive as-is
		}

		for _, entry := range entries {
			// check context cancellation
			select {
			case <-ctx.Done():
				return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
			default:
			}

			// skip hidden files
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			// skip directories
			if entry.IsDir() {
				continue
			}

			// process only .md files
			if strings.HasSuffix(entry.Name(), ".md") {
				path := filepath.Join(dir, entry.Name())
				info, err := entry.Info()
				if err != nil {
					continue // skip files we can't stat
				}

				fileInfo := FileInfo{
					Name:       entry.Name(),
					Filename:   string(source) + ":" + entry.Name(),
					Normalized: strings.ToLower(entry.Name()),
					Source:     source,
					Path:       path,
					Size:       info.Size(),
				}
				results = append(results, fileInfo)
			}
		}
	}

	return results, nil
}
