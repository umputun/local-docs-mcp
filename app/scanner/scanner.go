package scanner

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Source represents documentation source type
type Source string

const (
	// SourceCommands represents ~/.claude/commands documentation
	SourceCommands Source = "commands"
	// SourceProjectDocs represents project docs (with configurable exclusions)
	SourceProjectDocs Source = "project-docs"
	// SourceProjectRoot represents root-level markdown files
	SourceProjectRoot Source = "project-root"
)

// Params contains parameters for creating a scanner
type Params struct {
	CommandsDir    string
	ProjectDocsDir string
	ProjectRootDir string
	MaxFileSize    int64
	ExcludeDirs    []string
}

// Scanner discovers and indexes documentation files from multiple sources
type Scanner struct {
	commandsDir    string
	projectDocsDir string
	projectRootDir string
	maxFileSize    int64
	excludeDirs    []string
}

// NewScanner creates a new scanner instance
func NewScanner(params Params) *Scanner {
	return &Scanner{
		commandsDir:    params.CommandsDir,
		projectDocsDir: params.ProjectDocsDir,
		projectRootDir: params.ProjectRootDir,
		maxFileSize:    params.MaxFileSize,
		excludeDirs:    params.ExcludeDirs,
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

	isCtxCanceled := func() error {
		select {
		case <-ctx.Done():
			return ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
		default:
			return nil
		}
	}

	if err := isCtxCanceled(); err != nil {
		return nil, err
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
	if ctxErr := isCtxCanceled(); ctxErr != nil {
		return nil, ctxErr
	}

	// scan project docs (with configurable exclusions)
	docFiles, err := s.scanSource(ctx, SourceProjectDocs, s.projectDocsDir, "**/*.md")
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		results = append(results, docFiles...)
	}

	// check context between scans
	if ctxErr := isCtxCanceled(); ctxErr != nil {
		return nil, ctxErr
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
	// check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
	default:
	}

	// check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, err // nolint:wrapcheck // returning os error as-is is acceptable
	}

	// determine if recursive scan needed
	recursive := strings.Contains(pattern, "**")
	if recursive {
		return s.scanRecursive(ctx, source, dir)
	}
	return s.scanFlat(ctx, source, dir)
}

// scanRecursive performs recursive directory scanning for markdown files
func (s *Scanner) scanRecursive(ctx context.Context, source Source, dir string) ([]FileInfo, error) {
	var results []FileInfo

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		// check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			slog.Debug("skipping path due to walk error", "path", path, "error", err)
			return nil // skip errors
		}

		// skip hidden files and directories
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// exclude configured directories from project docs
		if d.IsDir() && source == SourceProjectDocs && s.shouldExcludeDir(d.Name()) {
			return fs.SkipDir
		}

		// process only .md files
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			info, err := d.Info()
			if err != nil {
				slog.Debug("skipping file, cannot stat", "path", path, "error", err)
				return nil // skip files we can't stat
			}

			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				slog.Debug("skipping file, cannot get relative path", "path", path, "error", err)
				return nil
			}

			// extract frontmatter
			description, tags := extractFrontmatter(path)

			fileInfo := FileInfo{
				Name:        filepath.Base(path),
				Filename:    string(source) + ":" + filepath.ToSlash(relPath),
				Normalized:  strings.ToLower(filepath.Base(path)),
				Source:      source,
				Path:        path,
				Size:        info.Size(),
				Description: description,
				Tags:        tags,
			}
			results = append(results, fileInfo)
		}

		return nil
	})
	if err != nil {
		return nil, err // nolint:wrapcheck // filepath.WalkDir error is descriptive as-is
	}
	return results, nil
}

// scanFlat performs non-recursive (flat) directory scanning for markdown files
func (s *Scanner) scanFlat(ctx context.Context, source Source, dir string) ([]FileInfo, error) {
	var results []FileInfo

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
				slog.Debug("skipping file, cannot stat", "path", path, "error", err)
				continue // skip files we can't stat
			}

			// extract frontmatter
			description, tags := extractFrontmatter(path)

			fileInfo := FileInfo{
				Name:        entry.Name(),
				Filename:    string(source) + ":" + entry.Name(),
				Normalized:  strings.ToLower(entry.Name()),
				Source:      source,
				Path:        path,
				Size:        info.Size(),
				Description: description,
				Tags:        tags,
			}
			results = append(results, fileInfo)
		}
	}

	return results, nil
}

// Close is a no-op for Scanner but required to implement Interface
func (s *Scanner) Close() error {
	return nil
}

// extractFrontmatter reads frontmatter from a file (max 2KB header read)
func extractFrontmatter(path string) (description string, tags []string) {
	// #nosec G304 - path is from scanner, not user input
	f, err := os.Open(path)
	if err != nil {
		return "", nil // can't read, return empty
	}
	defer f.Close()

	// read first 2KB (enough for frontmatter)
	buf := make([]byte, 2048)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", nil // read error, return empty
	}

	// parse frontmatter (ParseFrontmatter returns empty metadata on parse errors)
	fm, _ := ParseFrontmatter(buf[:n])
	return fm.Description, fm.Tags
}

// shouldExcludeDir checks if directory should be excluded based on excludeDirs list
func (s *Scanner) shouldExcludeDir(dirName string) bool {
	for _, excludeDir := range s.excludeDirs {
		if dirName == excludeDir {
			return true
		}
	}
	return false
}
