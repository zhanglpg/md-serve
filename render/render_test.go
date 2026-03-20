package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdown_BasicRendering(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	source := []byte("Just a paragraph with no headings.")
	toc := extractTOC(source)
	if len(toc) != 0 {
		t.Errorf("expected 0 TOC entries, got %d", len(toc))
	}
}

// --- Heading ID generation tests ---

func TestGenerateHeadingID(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	input := []byte("Hello %%this is a comment%% World")
	result := preprocessObsidian(input, nil)
	if strings.Contains(string(result), "%%") {
		t.Error("expected comments to be removed")
	}
	if !strings.Contains(string(result), "Hello") || !strings.Contains(string(result), "World") {
		t.Error("expected surrounding text to be preserved")
	}
}

func TestPreprocessObsidian_Highlights(t *testing.T) {
	t.Parallel()
	input := []byte("This is ==highlighted== text")
	result := preprocessObsidian(input, nil)
	if !strings.Contains(string(result), "<mark>highlighted</mark>") {
		t.Errorf("expected highlight conversion, got %q", string(result))
	}
}

// --- Obsidian postprocessing tests ---

func TestPreprocessObsidian_Wikilinks(t *testing.T) {
	t.Parallel()
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
		{
			"wikilink with angle brackets",
			"See [[Page <draft>]] here",
			`<a class="wikilink" href="/Page%20%3Cdraft%3E.md">Page &lt;draft&gt;</a>`,
		},
		{
			"wikilink with angle brackets in display text",
			"See [[Page|Show <this>]] here",
			`>Show &lt;this&gt;</a>`,
		},
		{
			"wikilink with Chinese angle brackets",
			"See [[Book《Title》]] here",
			`href="/Book%E3%80%8ATitle%E3%80%8B.md"`,
		},
		{
			"wikilink with Chinese angle brackets display",
			"See [[Book《Title》|《Title》by Author]] here",
			`>《Title》by Author</a>`,
		},
		{
			"wikilink with mixed brackets",
			"See [[Draft <v1> 《Final》]] here",
			`href="/Draft%20%3Cv1%3E%20%E3%80%8AFinal%E3%80%8B.md"`,
		},
		{
			"wikilink ampersand display escaped",
			"See [[Q&A]] here",
			`>Q&amp;A</a>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := string(preprocessObsidian([]byte(tc.input), nil))
			if !strings.Contains(result, tc.contains) {
				t.Errorf("expected result to contain %q, got %q", tc.contains, result)
			}
		})
	}
}

func TestPreprocessObsidian_AttachmentLinks(t *testing.T) {
	t.Parallel()
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
			result := string(preprocessObsidian([]byte(tc.input), nil))
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

func TestPreprocessObsidian_ImageEmbeds(t *testing.T) {
	t.Parallel()
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
			"excalidraw embed becomes img",
			"Here is ![[drawing.excalidraw]]",
			`<img src="/drawing.excalidraw" alt="drawing.excalidraw" />`,
		},
		{
			"avif embed",
			"Here is ![[photo.avif]]",
			`<img src="/photo.avif" alt="photo.avif" />`,
		},
		{
			"webp embed",
			"Here is ![[photo.webp]]",
			`<img src="/photo.webp" alt="photo.webp" />`,
		},
		{
			"tiff embed",
			"Here is ![[photo.tiff]]",
			`<img src="/photo.tiff" alt="photo.tiff" />`,
		},
		{
			"gif embed",
			"Here is ![[animation.gif]]",
			`<img src="/animation.gif" alt="animation.gif" />`,
		},
		{
			"bmp embed",
			"Here is ![[image.bmp]]",
			`<img src="/image.bmp" alt="image.bmp" />`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := string(preprocessObsidian([]byte(tc.input), nil))
			if !strings.Contains(result, tc.contains) {
				t.Errorf("expected result to contain %q, got %q", tc.contains, result)
			}
		})
	}
}

func TestPostprocessObsidian_Callouts(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	input := "<blockquote>\n<p>[!info] "
	result := postprocessObsidian(input, nil)
	if !strings.Contains(result, `callout-info`) {
		t.Error("expected callout-info class")
	}
}

func TestPreprocessObsidian_EmbedWithAngleBrackets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			"image embed with angle brackets in alt",
			`![[photo.png|Image <preview>]]`,
			`<img src="/photo.png" alt="Image &lt;preview&gt;" />`,
		},
		{
			"non-image embed with angle brackets",
			`![[doc <v2>.pdf]]`,
			`<a class="wikilink embed" href="/doc%20%3Cv2%3E.pdf">doc &lt;v2&gt;.pdf</a>`,
		},
		{
			"embed with Chinese angle brackets",
			`![[photo《special》.png]]`,
			`<img src="/photo%E3%80%8Aspecial%E3%80%8B.png" alt="photo《special》.png" />`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := string(preprocessObsidian([]byte(tc.input), nil))
			if !strings.Contains(result, tc.contains) {
				t.Errorf("expected result to contain %q, got %q", tc.contains, result)
			}
		})
	}
}

func TestMarkdown_AngleBracketsInWikilinks(t *testing.T) {
	t.Parallel()
	// End-to-end test: angle brackets inside wiki links should not be
	// interpreted as HTML tags by goldmark.
	input := []byte("See [[Page <draft>]] for details")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, `href="/Page%20%3Cdraft%3E.md"`) {
		t.Errorf("expected angle brackets URL-encoded in href, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, `>Page &lt;draft&gt;</a>`) {
		t.Errorf("expected angle brackets HTML-escaped in display text, got %q", result.HTML)
	}
}

func TestMarkdown_ChineseBracketsInWikilinks(t *testing.T) {
	t.Parallel()
	input := []byte("See [[Book《Title》]] here")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, `href="/Book%E3%80%8ATitle%E3%80%8B.md"`) {
		t.Errorf("expected Chinese brackets URL-encoded in href, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, `>Book《Title》</a>`) {
		t.Errorf("expected Chinese brackets preserved in display text, got %q", result.HTML)
	}
}

// --- Integration test: full markdown with Obsidian features ---

func TestMarkdown_ObsidianFeatures(t *testing.T) {
	t.Parallel()
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

func TestPreprocessObsidian_WithVaultResolution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "Target.md"), []byte("# Target"), 0644)
	os.WriteFile(filepath.Join(dir, "Root.md"), []byte("# Root"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: "/vault1"}

	// Wiki link to file in subdirectory
	result := string(preprocessObsidian([]byte("[[Target]]"), opts))
	if !strings.Contains(result, `href="/vault1/sub/Target.md"`) {
		t.Errorf("expected resolved path with vault prefix, got %q", result)
	}

	// Wiki link to file at root
	result = string(preprocessObsidian([]byte("[[Root]]"), opts))
	if !strings.Contains(result, `href="/vault1/Root.md"`) {
		t.Errorf("expected resolved path at root with vault prefix, got %q", result)
	}

	// Wiki link with no prefix (single-vault mode)
	optsNoPrefix := &RenderOptions{VaultDir: dir, URLPrefix: ""}
	result = string(preprocessObsidian([]byte("[[Target]]"), optsNoPrefix))
	if !strings.Contains(result, `href="/sub/Target.md"`) {
		t.Errorf("expected resolved path without prefix, got %q", result)
	}

	// Wiki link to non-existent file falls back to original behavior
	result = string(preprocessObsidian([]byte("[[NonExistent]]"), opts))
	if !strings.Contains(result, `href="/vault1/NonExistent.md"`) {
		t.Errorf("expected fallback path with prefix, got %q", result)
	}
}

func TestPreprocessObsidian_EmbedWithVaultResolution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	os.MkdirAll(assetsDir, 0755)
	os.WriteFile(filepath.Join(assetsDir, "photo.png"), []byte("png"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: "/vault1"}

	// Embed should resolve the image path
	result := string(preprocessObsidian([]byte("![[photo.png]]"), opts))
	if !strings.Contains(result, `<img src="/vault1/assets/photo.png"`) {
		t.Errorf("expected resolved img src with vault prefix, got %q", result)
	}

	// Without prefix (single-vault mode)
	optsNoPrefix := &RenderOptions{VaultDir: dir, URLPrefix: ""}
	result = string(preprocessObsidian([]byte("![[photo.png]]"), optsNoPrefix))
	if !strings.Contains(result, `<img src="/assets/photo.png"`) {
		t.Errorf("expected resolved img src without prefix, got %q", result)
	}
}

func TestPreprocessObsidian_ExcalidrawEmbed_WithShadowSVG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	drawDir := filepath.Join(dir, "drawings")
	os.MkdirAll(drawDir, 0755)
	os.WriteFile(filepath.Join(drawDir, "diagram.excalidraw"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(drawDir, "diagram.excalidraw.svg"), []byte("<svg></svg>"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: ""}

	result := string(preprocessObsidian([]byte("![[diagram.excalidraw]]"), opts))
	if !strings.Contains(result, `<img src="/drawings/diagram.excalidraw.svg"`) {
		t.Errorf("expected img tag with shadow SVG, got %q", result)
	}
	if strings.Contains(result, `class="excalidraw-embed"`) {
		t.Error("should not produce excalidraw-embed when shadow SVG exists")
	}
}

func TestPreprocessObsidian_ExcalidrawEmbed_WithShadowPNG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "diagram.excalidraw"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "diagram.excalidraw.png"), []byte("png"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: ""}

	result := string(preprocessObsidian([]byte("![[diagram.excalidraw]]"), opts))
	if !strings.Contains(result, `<img src="/diagram.excalidraw.png"`) {
		t.Errorf("expected img tag with shadow PNG, got %q", result)
	}
}

func TestPreprocessObsidian_ExcalidrawEmbed_SVGPreferredOverPNG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "diagram.excalidraw"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "diagram.excalidraw.svg"), []byte("<svg></svg>"), 0644)
	os.WriteFile(filepath.Join(dir, "diagram.excalidraw.png"), []byte("png"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: ""}

	result := string(preprocessObsidian([]byte("![[diagram.excalidraw]]"), opts))
	if !strings.Contains(result, `.excalidraw.svg"`) {
		t.Errorf("expected SVG preferred over PNG, got %q", result)
	}
}

func TestPreprocessObsidian_ExcalidrawEmbed_NoShadowFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	drawDir := filepath.Join(dir, "drawings")
	os.MkdirAll(drawDir, 0755)
	excalidrawData := `{"type":"excalidraw","version":2,"elements":[{"type":"rectangle"}]}`
	os.WriteFile(filepath.Join(drawDir, "diagram.excalidraw"), []byte(excalidrawData), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: ""}

	// Without shadow file, should render as <img> pointing to the .excalidraw URL
	// so the server can resolve to the shadow at request time
	result := string(preprocessObsidian([]byte("![[diagram.excalidraw]]"), opts))
	if !strings.Contains(result, `<img src="/drawings/diagram.excalidraw"`) {
		t.Errorf("expected img tag with excalidraw path, got %q", result)
	}
	if strings.Contains(result, `class="excalidraw-embed"`) {
		t.Error("should not produce excalidraw-embed div")
	}
}

func TestPreprocessObsidian_ExcalidrawEmbed_WithPrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.excalidraw"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "test.excalidraw.svg"), []byte("<svg></svg>"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: "/vault1"}

	result := string(preprocessObsidian([]byte("![[test.excalidraw]]"), opts))
	if !strings.Contains(result, `<img src="/vault1/test.excalidraw.svg"`) {
		t.Errorf("expected img with vault prefix, got %q", result)
	}
}

func TestPreprocessObsidian_ExcalidrawEmbed_FileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := &RenderOptions{VaultDir: dir, URLPrefix: ""}

	// When file doesn't exist, should still render as <img> so the server
	// can attempt shadow resolution at request time
	result := string(preprocessObsidian([]byte("![[missing.excalidraw]]"), opts))
	if !strings.Contains(result, `<img src="/missing.excalidraw"`) {
		t.Errorf("expected img tag for missing excalidraw file, got %q", result)
	}
	if strings.Contains(result, `class="excalidraw-embed"`) {
		t.Error("should not produce excalidraw-embed for missing file")
	}
	if strings.Contains(result, `class="wikilink embed"`) {
		t.Error("should not produce wikilink embed link for excalidraw file")
	}
}

func TestPreprocessObsidian_ExcalidrawWikilink_WithShadow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "drawing.excalidraw"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "drawing.excalidraw.svg"), []byte("<svg></svg>"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: ""}

	result := string(preprocessObsidian([]byte("See [[drawing.excalidraw]]"), opts))
	if !strings.Contains(result, `href="/drawing.excalidraw.svg"`) {
		t.Errorf("expected wikilink href to point to shadow SVG, got %q", result)
	}
}

func TestPreprocessObsidian_ExcalidrawEmbed_VaultWideResolution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Shadow SVG is in a subdirectory, but the embed uses just the basename
	subDir := filepath.Join(dir, "drawings", "nested")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "diagram.excalidraw"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(subDir, "diagram.excalidraw.svg"), []byte("<svg></svg>"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: ""}

	// Wiki link without path should resolve to the nested shadow SVG
	result := string(preprocessObsidian([]byte("![[diagram.excalidraw]]"), opts))
	if !strings.Contains(result, `diagram.excalidraw.svg"`) {
		t.Errorf("expected vault-wide resolution to find nested shadow SVG, got %q", result)
	}
}

func TestPreprocessObsidian_ExcalidrawWikilink_VaultWideResolution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subDir := filepath.Join(dir, "drawings", "nested")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "drawing.excalidraw"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(subDir, "drawing.excalidraw.svg"), []byte("<svg></svg>"), 0644)

	opts := &RenderOptions{VaultDir: dir, URLPrefix: ""}

	result := string(preprocessObsidian([]byte("See [[drawing.excalidraw]]"), opts))
	if !strings.Contains(result, `drawing.excalidraw.svg"`) {
		t.Errorf("expected vault-wide resolution to find nested shadow SVG, got %q", result)
	}
}

func TestFindExcalidrawShadow_VaultWideResolution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subDir := filepath.Join(dir, "deep", "folder")
	os.MkdirAll(subDir, 0755)
	// Shadow exists in a subdirectory but we search with just the basename
	os.WriteFile(filepath.Join(subDir, "chart.excalidraw.svg"), []byte("<svg></svg>"), 0644)

	// Direct path doesn't exist, should fall back to vault-wide search
	result := findExcalidrawShadow(dir, "chart.excalidraw")
	expected := filepath.Join("deep", "folder", "chart.excalidraw.svg")
	if result != expected {
		t.Errorf("expected vault-wide resolution to find %q, got %q", expected, result)
	}
}

func TestFindExcalidrawShadow_VaultWideResolution_PNG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subDir := filepath.Join(dir, "assets")
	os.MkdirAll(subDir, 0755)
	// Only PNG shadow exists in a subdirectory
	os.WriteFile(filepath.Join(subDir, "chart.excalidraw.png"), []byte("png"), 0644)

	result := findExcalidrawShadow(dir, "chart.excalidraw")
	expected := filepath.Join("assets", "chart.excalidraw.png")
	if result != expected {
		t.Errorf("expected vault-wide resolution to find %q, got %q", expected, result)
	}
}

func TestResolveWikiTarget_CaseInsensitive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subDir := filepath.Join(dir, "notes")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "my-notes.md"), []byte("# Notes"), 0644)

	// Search with different casing
	got := ResolveWikiTarget(dir, "My-Notes")
	expected := filepath.Join("notes", "my-notes.md")
	if got != expected {
		t.Errorf("ResolveWikiTarget case insensitive = %q, want %q", got, expected)
	}
}

func TestResolveWikiTarget_SpaceHyphenInterop(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "My Page.md"), []byte("# Page"), 0644)

	// Search with hyphens should find file with spaces
	got := ResolveWikiTarget(dir, "My-Page")
	if got != "My Page.md" {
		t.Errorf("ResolveWikiTarget space/hyphen interop = %q, want %q", got, "My Page.md")
	}
}

func TestResolveWikiTarget_WithHashAnchor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "page.md"), []byte("# Page"), 0644)

	// Target with .md extension already
	got := ResolveWikiTarget(dir, "page.md")
	if got != "page.md" {
		t.Errorf("ResolveWikiTarget with .md suffix = %q, want %q", got, "page.md")
	}
}

func TestResolveWikiTarget_DirectPathPreferredOverWalk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create file at root AND in subdirectory
	os.WriteFile(filepath.Join(dir, "Page.md"), []byte("# Root Page"), 0644)
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "Page.md"), []byte("# Sub Page"), 0644)

	// Direct path should win
	got := ResolveWikiTarget(dir, "Page")
	if got != "Page.md" {
		t.Errorf("ResolveWikiTarget direct path preferred = %q, want %q", got, "Page.md")
	}
}

func TestMarkdown_MultipleHighlightsOnSameLine(t *testing.T) {
	t.Parallel()
	input := []byte("This has ==first== and ==second== highlights")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, "<mark>first</mark>") {
		t.Error("expected first highlight to be converted")
	}
	if !strings.Contains(result.HTML, "<mark>second</mark>") {
		t.Error("expected second highlight to be converted")
	}
}

func TestMarkdown_MultilineComments(t *testing.T) {
	t.Parallel()
	input := []byte("Before %%\nthis is hidden\n%% After")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.HTML, "this is hidden") {
		t.Error("expected multiline comment to be removed")
	}
}

func TestMarkdown_EmptyInput(t *testing.T) {
	t.Parallel()
	result, err := Markdown([]byte(""), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HTML != "" && result.HTML != "\n" {
		// Empty input should produce minimal output
		t.Logf("empty input produced: %q", result.HTML)
	}
	if len(result.TOC) != 0 {
		t.Errorf("expected 0 TOC entries for empty input, got %d", len(result.TOC))
	}
}

func TestPostprocessObsidian_MultipleCallouts(t *testing.T) {
	t.Parallel()
	input := "<blockquote>\n<p>[!warning] Warning title</p>\n</blockquote>\n<blockquote>\n<p>[!tip] Tip title</p>\n</blockquote>"
	result := postprocessObsidian(input, nil)
	if !strings.Contains(result, "callout-warning") {
		t.Error("expected callout-warning class")
	}
	if !strings.Contains(result, "callout-tip") {
		t.Error("expected callout-tip class")
	}
}

func TestResolveWikiTarget(t *testing.T) {
	t.Parallel()
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
			got := ResolveWikiTarget(vaultDir, tc.target)
			if got != tc.expected {
				t.Errorf("ResolveWikiTarget(%q) = %q, want %q", tc.target, got, tc.expected)
			}
		})
	}
}
