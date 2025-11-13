package scanner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantDescription string
		wantTags        []string
		wantContent     string
	}{
		{
			name: "valid frontmatter with description and tags array",
			input: `---
description: Enforce Test-Driven Development approach for Go code
tags: [testing, development]
---
# Content here
Some markdown content`,
			wantDescription: "Enforce Test-Driven Development approach for Go code",
			wantTags:        []string{"testing", "development"},
			wantContent:     "# Content here\nSome markdown content",
		},
		{
			name: "valid frontmatter with comma-separated tags",
			input: `---
description: Enforce Test-Driven Development approach for Go code
tags: testing, development
---
# Content here`,
			wantDescription: "Enforce Test-Driven Development approach for Go code",
			wantTags:        []string{"testing", "development"},
			wantContent:     "# Content here",
		},
		{
			name: "frontmatter with only description",
			input: `---
description: Just a description
---
Content without tags`,
			wantDescription: "Just a description",
			wantTags:        nil,
			wantContent:     "Content without tags",
		},
		{
			name: "frontmatter with only tags",
			input: `---
tags: [tag1, tag2, tag3]
---
Content without description`,
			wantDescription: "",
			wantTags:        []string{"tag1", "tag2", "tag3"},
			wantContent:     "Content without description",
		},
		{
			name: "no frontmatter",
			input: `# Regular markdown
No frontmatter here`,
			wantDescription: "",
			wantTags:        nil,
			wantContent:     "# Regular markdown\nNo frontmatter here",
		},
		{
			name: "empty frontmatter",
			input: `---
---
Content after empty frontmatter`,
			wantDescription: "",
			wantTags:        nil,
			wantContent:     "Content after empty frontmatter",
		},
		{
			name: "malformed frontmatter - no closing delimiter",
			input: `---
description: Missing closing delimiter
Content starts here`,
			wantDescription: "",
			wantTags:        nil,
			wantContent:     "---\ndescription: Missing closing delimiter\nContent starts here",
		},
		{
			name: "malformed frontmatter - invalid YAML",
			input: `---
description: [invalid yaml: {
---
Content here`,
			wantDescription: "",
			wantTags:        nil,
			wantContent:     "---\ndescription: [invalid yaml: {\n---\nContent here",
		},
		{
			name: "frontmatter with extra whitespace in tags",
			input: `---
description: Test
tags: tag1 ,  tag2  , tag3
---
Content`,
			wantDescription: "Test",
			wantTags:        []string{"tag1", "tag2", "tag3"},
			wantContent:     "Content",
		},
		{
			name:            "windows line endings",
			input:           "---\r\ndescription: Windows test\r\ntags: [win, test]\r\n---\r\nContent",
			wantDescription: "Windows test",
			wantTags:        []string{"win", "test"},
			wantContent:     "Content",
		},
		{
			name:            "empty input",
			input:           "",
			wantDescription: "",
			wantTags:        nil,
			wantContent:     "",
		},
		{
			name: "only frontmatter delimiter",
			input: `---
`,
			wantDescription: "",
			wantTags:        nil,
			wantContent:     "---\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, content := ParseFrontmatter([]byte(tt.input))
			assert.Equal(t, tt.wantDescription, fm.Description, "description mismatch")
			assert.Equal(t, tt.wantTags, fm.Tags, "tags mismatch")
			assert.Equal(t, tt.wantContent, string(content), "content mismatch")
		})
	}
}

func TestParseFrontmatter_MultilineDescription(t *testing.T) {
	input := `---
description: |
  This is a multiline
  description that spans
  multiple lines
tags: [test]
---
Content here`

	fm, content := ParseFrontmatter([]byte(input))
	assert.Contains(t, fm.Description, "multiline")
	assert.Equal(t, []string{"test"}, fm.Tags)
	assert.Equal(t, "Content here", string(content))
}
