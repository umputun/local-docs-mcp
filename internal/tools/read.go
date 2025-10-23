package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/umputun/local-docs-mcp/internal/scanner"
)

// ReadInput represents input for reading a documentation file
type ReadInput struct {
	Path   string  `json:"path"`
	Source *string `json:"source,omitempty"`
}

// ReadOutput contains the result of reading a documentation file
type ReadOutput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	Source  string `json:"source"`
}

// ReadDoc reads a specific documentation file
func ReadDoc(sc *scanner.Scanner, path string, source *string, maxSize int64) (*ReadOutput, error) {
	// parse source prefix from path if present
	var sourceStr string
	cleanPath := path

	if strings.Contains(path, ":") {
		parts := strings.SplitN(path, ":", 2)
		sourceStr = parts[0]
		cleanPath = parts[1]
	} else if source != nil {
		sourceStr = *source
	}

	// map source string to directory
	var baseDir string
	var actualSource scanner.Source

	if sourceStr != "" {
		switch sourceStr {
		case "commands":
			baseDir = getCommandsDir(sc)
			actualSource = scanner.SourceCommands
		case "project-docs":
			baseDir = getProjectDocsDir(sc)
			actualSource = scanner.SourceProjectDocs
		case "project-root":
			baseDir = getProjectRootDir(sc)
			actualSource = scanner.SourceProjectRoot
		default:
			return nil, fmt.Errorf("invalid source: %s", sourceStr)
		}

		// try to resolve and read from specified source
		resolvedPath, err := scanner.SafeResolvePath(baseDir, cleanPath, maxSize)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path in %s: %w", sourceStr, err)
		}

		// #nosec G304 - path is validated by SafeResolvePath
		content, err := os.ReadFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		return &ReadOutput{
			Path:    cleanPath,
			Content: string(content),
			Size:    len(content),
			Source:  string(actualSource),
		}, nil
	}

	// no source specified, try all sources in order
	sources := []struct {
		name string
		dir  string
		src  scanner.Source
	}{
		{"commands", getCommandsDir(sc), scanner.SourceCommands},
		{"project-docs", getProjectDocsDir(sc), scanner.SourceProjectDocs},
		{"project-root", getProjectRootDir(sc), scanner.SourceProjectRoot},
	}

	for _, s := range sources {
		resolvedPath, err := scanner.SafeResolvePath(s.dir, cleanPath, maxSize)
		if err != nil {
			continue // try next source
		}

		// #nosec G304 - path is validated by SafeResolvePath
		content, err := os.ReadFile(resolvedPath)
		if err != nil {
			continue // try next source
		}

		return &ReadOutput{
			Path:    cleanPath,
			Content: string(content),
			Size:    len(content),
			Source:  string(s.src),
		}, nil
	}

	return nil, fmt.Errorf("file not found in any source: %s", cleanPath)
}

// helper functions to access scanner's directories

func getCommandsDir(sc *scanner.Scanner) string {
	return sc.CommandsDir()
}

func getProjectDocsDir(sc *scanner.Scanner) string {
	return sc.ProjectDocsDir()
}

func getProjectRootDir(sc *scanner.Scanner) string {
	return sc.ProjectRootDir()
}
