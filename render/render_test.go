package render

import (
	"strings"
	"testing"
)

func TestMarkdown_BasicRendering(t *testing.T) {
	input := []byte("# Hello World\n\nThis is a paragraph.")
	result, err := Markdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, "<h1") {
		t.Error("expected HTML to contain <h1> tag")
	}
	if !strings.Contains(result.HTML, "Hello World") {
		t.Error("expected HTML to contain heading text")
	}
	if !strings.Contains(result.HTML, "<p>This is a paragraph.</p>") {
		t.Error("expected HTML to contain paragraph")
	}
}

func TestMarkdown_CodeBlock(t *testing.T) {
	input := []byte("```go\nfmt.Println(\"hello\")\n```")
	result, err := Markdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, "<pre") {
		t.Error("expected HTML to contain <pre> tag for code block")
	}
}

func TestMarkdown_GFMTable(t *testing.T) {
	input := []byte("| A | B |\n|---|---|\n| 1 | 2 |")
	result, err := Markdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, "<table>") {
		t.Error("expected HTML to contain a table")
	}
}

func TestMarkdown_Links(t *testing.T) {
	input := []byte("[Click here](https://example.com)")
	result, err := Markdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, `href="https://example.com"`) {
		t.Error("expected HTML to contain link href")
	}
	if !strings.Contains(result.HTML, "Click here") {
		t.Error("expected HTML to contain link text")
	}
}

// --- TOC extraction tests ---

func TestExtractTOC_SingleHeading(t *testing.T) {
	source := []byte("# Introduction")
	toc := extractTOC(source)
	if len(toc) != 1 {
		t.Fatalf("expected 1 TOC entry, got %d", len(toc))
	}
	if toc[0].Level != 1 {
		t.Errorf("expected level 1, got %d", toc[0].Level)
	}
	if toc[0].Title != "Introduction" {
		t.Errorf("expected title 'Introduction', got %q", toc[0].Title)
	}
	if toc[0].ID != "introduction" {
		t.Errorf("expected id 'introduction', got %q", toc[0].ID)
	}
}

func TestExtractTOC_MultipleHeadings(t *testing.T) {
	source := []byte("# Title\n## Section One\n### Subsection\n## Section Two")
	toc := extractTOC(source)
	if len(toc) != 4 {
		t.Fatalf("expected 4 TOC entries, got %d", len(toc))
	}
	expected := []struct {
		level int
		title string
	}{
		{1, "Title"},
		{2, "Section One"},
		{3, "Subsection"},
		{2, "Section Two"},
	}
	for i, e := range expected {
		if toc[i].Level != e.level {
			t.Errorf("entry %d: expected level %d, got %d", i, e.level, toc[i].Level)
		}
		if toc[i].Title != e.title {
			t.Errorf("entry %d: expected title %q, got %q", i, e.title, toc[i].Title)
		}
	}
}

func TestExtractTOC_StripsInlineMarkdown(t *testing.T) {
	source := []byte("## A **bold** heading\n## A [link](http://x.com) heading")
	toc := extractTOC(source)
	if len(toc) != 2 {
		t.Fatalf("expected 2 TOC entries, got %d", len(toc))
	}
	if toc[0].Title != "A bold heading" {
		t.Errorf("expected bold stripped, got %q", toc[0].Title)
	}
	if toc[1].Title != "A link heading" {
		t.Errorf("expected link stripped, got %q", toc[1].Title)
	}
}

func TestExtractTOC_NoHeadings(t *testing.T) {
	source := []byte("Just a paragraph with no headings.")
	toc := extractTOC(source)
	if len(toc) != 0 {
		t.Errorf("expected 0 TOC entries, got %d", len(toc))
	}
}

// --- Heading ID generation tests ---

func TestGenerateHeadingID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Getting Started", "getting-started"},
		{"API v2.0", "api-v20"},
		{"simple", "simple"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"Trailing - Dash -", "trailing---dash"},
		{"Special Ch@rs!", "special-chrs"},
	}
	for _, tc := range tests {
		got := generateHeadingID(tc.input)
		if got != tc.expected {
			t.Errorf("generateHeadingID(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// --- Obsidian preprocessing tests ---

func TestPreprocessObsidian_Comments(t *testing.T) {
	input := []byte("Hello %%this is a comment%% World")
	result := preprocessObsidian(input)
	if strings.Contains(string(result), "%%") {
		t.Error("expected comments to be removed")
	}
	if !strings.Contains(string(result), "Hello") || !strings.Contains(string(result), "World") {
		t.Error("expected surrounding text to be preserved")
	}
}

func TestPreprocessObsidian_Highlights(t *testing.T) {
	input := []byte("This is ==highlighted== text")
	result := preprocessObsidian(input)
	if !strings.Contains(string(result), "<mark>highlighted</mark>") {
		t.Errorf("expected highlight conversion, got %q", string(result))
	}
}

// --- Obsidian postprocessing tests ---

func TestPostprocessObsidian_Wikilinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			"simple wikilink",
			"Check [[My Page]] for details",
			`<a class="wikilink" href="/My-Page.md">My Page</a>`,
		},
		{
			"wikilink with display text",
			"See [[My Page|custom text]] here",
			`<a class="wikilink" href="/My-Page.md">custom text</a>`,
		},
		{
			"wikilink already has .md",
			"Link to [[notes.md]]",
			`href="/notes.md"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := postprocessObsidian(tc.input)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("expected result to contain %q, got %q", tc.contains, result)
			}
		})
	}
}

func TestPostprocessObsidian_Callouts(t *testing.T) {
	input := "<blockquote>\n<p>[!warning] Be careful"
	result := postprocessObsidian(input)
	if !strings.Contains(result, `callout-warning`) {
		t.Error("expected callout-warning class")
	}
	if !strings.Contains(result, "Be careful") {
		t.Error("expected callout title text")
	}
}

func TestPostprocessObsidian_CalloutDefaultTitle(t *testing.T) {
	input := "<blockquote>\n<p>[!info] "
	result := postprocessObsidian(input)
	if !strings.Contains(result, `callout-info`) {
		t.Error("expected callout-info class")
	}
}

// --- Integration test: full markdown with Obsidian features ---

func TestMarkdown_ObsidianFeatures(t *testing.T) {
	input := []byte(`# My Notes

This has ==highlights== and %%hidden comments%%.

Check [[Other Page]] for more info.

> [!tip] Pro Tip
> This is a helpful tip.
`)
	result, err := Markdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, "<mark>highlights</mark>") {
		t.Error("expected highlight marks in output")
	}
	if strings.Contains(result.HTML, "hidden comments") {
		t.Error("expected comments to be removed")
	}
	if !strings.Contains(result.HTML, "wikilink") {
		t.Error("expected wikilink in output")
	}
	if len(result.TOC) != 1 {
		t.Errorf("expected 1 TOC entry, got %d", len(result.TOC))
	}
}
