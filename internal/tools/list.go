package tools

import "github.com/umputun/local-docs-mcp/internal/scanner"

// DocInfo represents information about a documentation file
type DocInfo struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Source   string `json:"source"`
	Size     int64  `json:"size,omitempty"`
	TooLarge bool   `json:"too_large,omitempty"`
}

// ListOutput contains the result of listing all documentation files
type ListOutput struct {
	Docs  []DocInfo `json:"docs"`
	Total int       `json:"total"`
}

// ListAllDocs returns a list of all available documentation files from all sources
func ListAllDocs(sc *scanner.Scanner, maxSize int64) (*ListOutput, error) {
	files, err := sc.Scan()
	if err != nil {
		return nil, err // nolint:wrapcheck // scanner error is descriptive
	}

	docs := make([]DocInfo, 0, len(files))
	for _, f := range files {
		doc := DocInfo{
			Name:     f.Name,
			Filename: f.Filename,
			Source:   string(f.Source),
			Size:     f.Size,
		}

		// mark files that exceed max size
		if f.Size > maxSize {
			doc.TooLarge = true
		}

		docs = append(docs, doc)
	}

	return &ListOutput{
		Docs:  docs,
		Total: len(docs),
	}, nil
}
