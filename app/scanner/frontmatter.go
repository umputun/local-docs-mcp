package scanner

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter contains metadata extracted from YAML frontmatter
type Frontmatter struct {
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

// rawFrontmatter is used for initial YAML parsing to handle flexible tag formats
type rawFrontmatter struct {
	Description string      `yaml:"description"`
	Tags        interface{} `yaml:"tags"`
}

// ParseFrontmatter extracts YAML frontmatter from markdown content.
// Returns frontmatter metadata and content without the frontmatter block.
// If no frontmatter exists or parsing fails, returns empty metadata and original content.
func ParseFrontmatter(content []byte) (frontmatter Frontmatter, strippedContent []byte) {
	var fm Frontmatter

	// check if content starts with frontmatter delimiter
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return fm, content
	}

	// determine line ending type
	lineEnding := "\n"
	if bytes.HasPrefix(content, []byte("---\r\n")) {
		lineEnding = "\r\n"
	}

	// find the closing delimiter
	// skip the opening "---\n" or "---\r\n"
	startIdx := bytes.Index(content, []byte(lineEnding))
	if startIdx == -1 {
		return fm, content
	}
	startIdx += len(lineEnding)

	// look for closing "---" on its own line
	remaining := content[startIdx:]

	// look for closing delimiter: either at start of remaining (empty frontmatter)
	// or preceded by a line ending
	var endIdx int
	closingPattern := "---" + lineEnding
	if bytes.HasPrefix(remaining, []byte(closingPattern)) {
		// empty frontmatter case: ---\n---\n
		endIdx = 0
	} else {
		// normal case: content\n---\n
		closingDelim := lineEnding + closingPattern
		endIdx = bytes.Index(remaining, []byte(closingDelim))
		if endIdx == -1 {
			// no closing delimiter found
			return fm, content
		}
		endIdx += len(lineEnding) // skip the newline before ---
	}

	// extract YAML block (between the two --- delimiters)
	yamlBlock := remaining[:endIdx]

	// parse YAML into raw format to handle flexible tag types (only if not empty)
	var raw rawFrontmatter
	if len(yamlBlock) > 0 {
		if err := yaml.Unmarshal(yamlBlock, &raw); err != nil {
			// parsing failed, return empty metadata with original content
			return Frontmatter{}, content
		}
		// copy description
		fm.Description = raw.Description
		// parse tags: handle string (comma-separated), array, or interface slice
		fm.Tags = parseTags(raw.Tags)
	}

	// find where content starts (after closing ---)
	contentStart := startIdx + endIdx + len(closingPattern)
	contentWithoutFrontmatter := content[contentStart:]

	return fm, contentWithoutFrontmatter
}

// parseTags converts various tag formats to []string
func parseTags(tags interface{}) []string {
	if tags == nil {
		return nil
	}

	switch v := tags.(type) {
	case string:
		// handle comma-separated string
		if v == "" {
			return nil
		}
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, tag := range parts {
			if trimmed := strings.TrimSpace(tag); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result

	case []interface{}:
		// handle YAML array
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				result = append(result, strings.TrimSpace(str))
			}
		}
		return result

	case []string:
		// already string array
		result := make([]string, 0, len(v))
		for _, tag := range v {
			if trimmed := strings.TrimSpace(tag); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result

	default:
		return nil
	}
}
