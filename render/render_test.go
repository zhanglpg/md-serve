package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdown_BasicRendering(t *testing.T) {
	input := []byte("# Hello World\n\nThis is a paragraph.")
	result, err := Markdown(input, nil)
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
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, "<pre") {
		t.Error("expected HTML to contain <pre> tag for code block")
	}
}

func TestMarkdown_GFMTable(t *testing.T) {
	input := []byte("| A | B |\n|---|---|\n| 1 | 2 |")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, "<table>") {
		t.Error("expected HTML to contain a table")
	}
}

func TestMarkdown_Links(t *testing.T) {
	input := []byte("[Click here](https://example.com)")
	result, err := Markdown(input, nil)
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
			`<a class="wikilink" href="/My%20Page.md">My Page</a>`,
		},
		{
			"wikilink with display text",
			"See [[My Page|custom text]] here",
			`<a class="wikilink" href="/My%20Page.md">custom text</a>`,
		},
		{
			"wikilink already has .md",
			"Link to [[notes.md]]",
			`href="/notes.md"`,
		},
		{
			"wikilink with special characters",
			"See [[Page (draft)]] here",
			`href="/Page%20%28draft%29.md"`,
		},
		{
			"wikilink with ampersand",
			"See [[Q&A]] here",
			`href="/Q&A.md"`,
		},
		{
			"wikilink with path",
			"See [[folder/My Page]] here",
			`href="/folder/My%20Page.md"`,
		},
		{
			"wikilink with hash",
			"See [[Notes #1]] here",
			`href="/Notes%20%231.md"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := postprocessObsidian(tc.input, nil)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("expected result to contain %q, got %q", tc.contains, result)
			}
		})
	}
}

func TestPostprocessObsidian_AttachmentLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{
			"png image link",
			"See [[photo.png]]",
			`<a class="wikilink" href="/photo.png">photo.png</a>`,
			".md",
		},
		{
			"jpg image link",
			"See [[photo.jpg]]",
			`href="/photo.jpg"`,
			".md",
		},
		{
			"excalidraw link",
			"See [[diagram.excalidraw]]",
			`href="/diagram.excalidraw"`,
			".md",
		},
		{
			"pdf link",
			"See [[document.pdf]]",
			`href="/document.pdf"`,
			".md",
		},
		{
			"attachment in subfolder",
			"See [[assets/photo.png]]",
			`href="/assets/photo.png"`,
			".md",
		},
		{
			"attachment with display text",
			"See [[photo.png|My Photo]]",
			`<a class="wikilink" href="/photo.png">My Photo</a>`,
			".md",
		},
		{
			"attachment with spaces in name",
			"See [[my photo.png]]",
			`href="/my%20photo.png"`,
			".md",
		},
		{
			"regular note still gets .md",
			"See [[My Page]]",
			`href="/My%20Page.md"`,
			"",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := postprocessObsidian(tc.input, nil)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("expected result to contain %q, got %q", tc.contains, result)
			}
			if tc.excludes != "" {
				// Check that the href does NOT contain the excluded suffix
				// We need to check the href specifically, not the whole output
				hrefStart := strings.Index(result, `href="`)
				if hrefStart >= 0 {
					hrefEnd := strings.Index(result[hrefStart+6:], `"`)
					href := result[hrefStart+6 : hrefStart+6+hrefEnd]
					if strings.HasSuffix(href, tc.excludes) {
						t.Errorf("href %q should not end with %q", href, tc.excludes)
					}
				}
			}
		})
	}
}

func TestPostprocessObsidian_ImageEmbeds(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			"png embed",
			"Here is ![[photo.png]]",
			`<img src="/photo.png" alt="photo.png" />`,
		},
		{
			"png embed with alt text",
			"Here is ![[photo.png|A nice photo]]",
			`<img src="/photo.png" alt="A nice photo" />`,
		},
		{
			"jpg embed",
			"Here is ![[image.jpg]]",
			`<img src="/image.jpg" alt="image.jpg" />`,
		},
		{
			"svg embed",
			"Here is ![[diagram.svg]]",
			`<img src="/diagram.svg" alt="diagram.svg" />`,
		},
		{
			"embed in subfolder",
			"Here is ![[assets/photo.png]]",
			`<img src="/assets/photo.png" alt="assets/photo.png" />`,
		},
		{
			"embed with spaces",
			"Here is ![[my photo.png]]",
			`<img src="/my%20photo.png" alt="my photo.png" />`,
		},
		{
			"non-image embed becomes link",
			"Here is ![[document.pdf]]",
			`<a class="wikilink embed" href="/document.pdf">document.pdf</a>`,
		},
		{
			"excalidraw embed becomes link",
			"Here is ![[drawing.excalidraw]]",
			`<a class="wikilink embed" href="/drawing.excalidraw">drawing.excalidraw</a>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := postprocessObsidian(tc.input, nil)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("expected result to contain %q, got %q", tc.contains, result)
			}
		})
	}
}

func TestPostprocessObsidian_Callouts(t *testing.T) {
	input := "<blockquote>\n<p>[!warning] Be careful"
	result := postprocessObsidian(input, nil)
	if !strings.Contains(result, `callout-warning`) {
		t.Error("expected callout-warning class")
	}
	if !strings.Contains(result, "Be careful") {
		t.Error("expected callout title text")
	}
}

func TestPostprocessObsidian_CalloutDefaultTitle(t *testing.T) {
	input := "<blockquote>\n<p>[!info] "
	result := postprocessObsidian(input, nil)
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
	result, err := Markdown(input, nil)
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

func TestPostprocessObsidian_WithVaultResolution(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "Target.md"), []byte("# Target"), 0644)
	os.WriteFile(filepath.Join(dir, "Root.md"), []byte("# Root"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: "/vault1"}

	// Wiki link to file in subdirectory
	result := postprocessObsidian("[[Target]]", opts)
	if !strings.Contains(result, `href="/vault1/sub/Target.md"`) {
		t.Errorf("expected resolved path with vault prefix, got %q", result)
	}

	// Wiki link to file at root
	result = postprocessObsidian("[[Root]]", opts)
	if !strings.Contains(result, `href="/vault1/Root.md"`) {
		t.Errorf("expected resolved path at root with vault prefix, got %q", result)
	}

	// Wiki link with no prefix (single-vault mode)
	optsNoPrefix := &RenderOptions{VaultDir: dir, URLPrefix: ""}
	result = postprocessObsidian("[[Target]]", optsNoPrefix)
	if !strings.Contains(result, `href="/sub/Target.md"`) {
		t.Errorf("expected resolved path without prefix, got %q", result)
	}

	// Wiki link to non-existent file falls back to original behavior
	result = postprocessObsidian("[[NonExistent]]", opts)
	if !strings.Contains(result, `href="/vault1/NonExistent.md"`) {
		t.Errorf("expected fallback path with prefix, got %q", result)
	}
}

func TestResolveWikiTarget(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "deep", "nested")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "Page.md"), []byte("# Page"), 0644)
	os.WriteFile(filepath.Join(dir, "Root.md"), []byte("# Root"), 0644)
	os.WriteFile(filepath.Join(dir, "photo.png"), []byte("png"), 0644)

	tests := []struct {
		name     string
		target   string
		expected string
	}{
		{"root file", "Root", "Root.md"},
		{"nested file", "Page", filepath.Join("deep", "nested", "Page.md")},
		{"direct path", "deep/nested/Page", filepath.Join("deep", "nested", "Page.md")},
		{"attachment", "photo.png", "photo.png"},
		{"not found", "Missing", ""},
		{"empty vault dir", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vaultDir := dir
			if tc.target == "" || tc.name == "empty vault dir" {
				vaultDir = ""
			}
			got := resolveWikiTarget(vaultDir, tc.target)
			if got != tc.expected {
				t.Errorf("resolveWikiTarget(%q) = %q, want %q", tc.target, got, tc.expected)
			}
		})
	}
}
