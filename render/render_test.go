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

func TestAltBasename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"my file.md", "my-file.md"},
		{"my-file.md", "my file.md"},
		{"myfile.md", ""},
		{"a b-c.md", "a-b-c.md"}, // space found first
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := altBasename(tc.input); got != tc.want {
				t.Errorf("altBasename(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFindExcalidrawShadow_NoShadow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "drawing.excalidraw"), []byte("{}"), 0644)

	got := findExcalidrawShadow(dir, "drawing.excalidraw")
	if got != "" {
		t.Errorf("findExcalidrawShadow() = %q, want empty string", got)
	}
}

func TestFindExcalidrawShadow_EmptyVaultDir(t *testing.T) {
	t.Parallel()
	got := findExcalidrawShadow("", "drawing.excalidraw")
	if got != "" {
		t.Errorf("findExcalidrawShadow() = %q, want empty string", got)
	}
}

// --- Frontmatter tests ---

func TestParseFrontmatter_Valid(t *testing.T) {
	t.Parallel()
	input := []byte("---\ntitle: My Note\ntags:\n  - go\n  - test\n---\n# Hello")
	props, body := parseFrontmatter(input)
	if props == nil {
		t.Fatal("expected frontmatter to be parsed")
	}
	if props["title"] != "My Note" {
		t.Errorf("title = %v, want 'My Note'", props["title"])
	}
	tags, ok := props["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Fatalf("tags = %v, want [go test]", props["tags"])
	}
	if string(body) != "# Hello" {
		t.Errorf("body = %q, want '# Hello'", body)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("# Just a heading\nSome text")
	props, body := parseFrontmatter(input)
	if props != nil {
		t.Error("expected nil properties for content without frontmatter")
	}
	if string(body) != string(input) {
		t.Error("body should equal original input when no frontmatter")
	}
}

func TestParseFrontmatter_InvalidYAML(t *testing.T) {
	t.Parallel()
	input := []byte("---\n: invalid: yaml: [[\n---\n# Hello")
	props, body := parseFrontmatter(input)
	if props != nil {
		t.Error("expected nil properties for invalid YAML")
	}
	// Frontmatter should always be stripped from body even on parse failure
	if string(body) != "# Hello" {
		t.Errorf("body = %q, want %q (frontmatter stripped)", body, "# Hello")
	}
}

func TestParseFrontmatter_EmptyFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("---\n---\n# Hello")
	props, body := parseFrontmatter(input)
	// Empty YAML produces nil map
	if props != nil {
		t.Logf("props = %v (empty frontmatter may produce nil or empty map)", props)
	}
	if !strings.Contains(string(body), "# Hello") {
		t.Errorf("body = %q, want to contain '# Hello'", body)
	}
}

func TestParseFrontmatter_NotAtStart(t *testing.T) {
	t.Parallel()
	input := []byte("Some text\n---\ntitle: Not frontmatter\n---\n")
	props, body := parseFrontmatter(input)
	if props != nil {
		t.Error("frontmatter not at start of file should not be parsed")
	}
	if string(body) != string(input) {
		t.Error("body should equal original input")
	}
}

func TestRenderFrontmatterHTML_TextProperty(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"title": "My Note"}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `class="frontmatter-properties"`) {
		t.Error("expected frontmatter-properties wrapper")
	}
	if !strings.Contains(html, `class="frontmatter-key"`) {
		t.Error("expected frontmatter-key element")
	}
	if !strings.Contains(html, "title") {
		t.Error("expected key name 'title'")
	}
	if !strings.Contains(html, "My Note") {
		t.Error("expected value 'My Note'")
	}
}

func TestRenderFrontmatterHTML_BooleanProperty(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"draft": true, "publish": false}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `checked disabled`) {
		t.Error("expected checked checkbox for true value")
	}
	if !strings.Contains(html, `<input type="checkbox" disabled>`) {
		t.Error("expected unchecked checkbox for false value")
	}
}

func TestRenderFrontmatterHTML_TagsList(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{
		"tags": []interface{}{"golang", "testing"},
	}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `class="frontmatter-tag"`) {
		t.Error("expected frontmatter-tag class for tags")
	}
	if !strings.Contains(html, "#golang") {
		t.Error("expected '#golang' tag with hash prefix")
	}
	if !strings.Contains(html, "#testing") {
		t.Error("expected '#testing' tag with hash prefix")
	}
}

func TestRenderFrontmatterHTML_TagsSingleString(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{
		"tags": "solo-tag",
	}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `class="frontmatter-tag"`) {
		t.Error("expected frontmatter-tag class")
	}
	if !strings.Contains(html, "#solo-tag") {
		t.Error("expected '#solo-tag' with hash prefix")
	}
}

func TestRenderFrontmatterHTML_AliasesList(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{
		"aliases": []interface{}{"Alt Name", "Other Name"},
	}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `class="frontmatter-alias"`) {
		t.Error("expected frontmatter-alias class for aliases")
	}
	if !strings.Contains(html, "Alt Name") {
		t.Error("expected alias value")
	}
}

func TestRenderFrontmatterHTML_CSSClasses(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{
		"cssclasses": []interface{}{"wide-page", "no-toc"},
	}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `class="frontmatter-cssclass"`) {
		t.Error("expected frontmatter-cssclass class")
	}
	if !strings.Contains(html, "wide-page") {
		t.Error("expected cssclass value")
	}
}

func TestRenderFrontmatterHTML_DateProperty(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"date": "2024-01-15"}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `<time class="frontmatter-date">`) {
		t.Error("expected <time> element for date value")
	}
	if !strings.Contains(html, "2024-01-15") {
		t.Error("expected date value")
	}
}

func TestRenderFrontmatterHTML_DatetimeProperty(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"created": "2024-01-15T10:30"}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `<time class="frontmatter-date">`) {
		t.Error("expected <time> element for datetime value")
	}
	if !strings.Contains(html, "2024-01-15T10:30") {
		t.Error("expected datetime value")
	}
}

func TestRenderFrontmatterHTML_DatetimeWithSpace(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"created": "2024-01-15 10:30"}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `<time class="frontmatter-date">`) {
		t.Error("expected <time> element for datetime with space separator")
	}
}

func TestRenderFrontmatterHTML_NumberProperty(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"rating": 4.5}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, "4.5") {
		t.Error("expected number value '4.5'")
	}
}

func TestRenderFrontmatterHTML_IntegerProperty(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"count": 42}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, "42") {
		t.Error("expected integer value '42'")
	}
}

func TestRenderFrontmatterHTML_NilValue(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"empty": nil}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `class="frontmatter-empty"`) {
		t.Error("expected frontmatter-empty class for nil value")
	}
	if !strings.Contains(html, "—") {
		t.Error("expected em dash for nil value")
	}
}

func TestRenderFrontmatterHTML_EmptyList(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"items": []interface{}{}}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `class="frontmatter-empty"`) {
		t.Error("expected frontmatter-empty for empty list")
	}
}

func TestRenderFrontmatterHTML_HTMLEscaping(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{"title": `<script>alert("xss")</script>`}
	html := renderFrontmatterHTML(props, nil)
	if strings.Contains(html, "<script>") {
		t.Error("HTML in property values must be escaped")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("expected escaped HTML entities")
	}
}

func TestRenderFrontmatterHTML_SortedKeys(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{
		"zebra": "z",
		"alpha": "a",
		"middle": "m",
	}
	html := renderFrontmatterHTML(props, nil)
	alphaIdx := strings.Index(html, "alpha")
	middleIdx := strings.Index(html, "middle")
	zebraIdx := strings.Index(html, "zebra")
	if alphaIdx > middleIdx || middleIdx > zebraIdx {
		t.Error("expected keys to appear in alphabetical order")
	}
}

func TestMarkdown_FrontmatterIntegration(t *testing.T) {
	t.Parallel()
	input := []byte("---\ntitle: Test Page\ntags:\n  - go\n  - markdown\ndraft: true\ndate: 2024-03-15\n---\n# Hello World\n\nContent here.")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Frontmatter rendered
	if !strings.Contains(result.HTML, `class="frontmatter-properties"`) {
		t.Error("expected frontmatter properties section in output")
	}
	if !strings.Contains(result.HTML, "Test Page") {
		t.Error("expected title property value")
	}
	if !strings.Contains(result.HTML, "#go") {
		t.Error("expected tag with hash prefix")
	}
	if !strings.Contains(result.HTML, `checked disabled`) {
		t.Error("expected checked checkbox for draft: true")
	}
	if !strings.Contains(result.HTML, `<time class="frontmatter-date">`) {
		t.Error("expected date rendered as <time>")
	}
	// Body rendered
	if !strings.Contains(result.HTML, "<h1") {
		t.Error("expected body heading to be rendered")
	}
	if !strings.Contains(result.HTML, "Content here.") {
		t.Error("expected body content")
	}
	// Frontmatter should NOT appear as raw text in body
	if strings.Contains(result.HTML, "---\ntitle:") {
		t.Error("raw frontmatter should be stripped from output")
	}
}

func TestMarkdown_FrontmatterStrippedFromTOC(t *testing.T) {
	t.Parallel()
	input := []byte("---\ntitle: My Page\n---\n# First Heading\n## Second Heading")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.TOC) != 2 {
		t.Fatalf("expected 2 TOC entries, got %d", len(result.TOC))
	}
	if result.TOC[0].Title != "First Heading" {
		t.Errorf("first TOC entry = %q, want 'First Heading'", result.TOC[0].Title)
	}
}

func TestMarkdown_NoFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("# No Frontmatter\n\nJust content.")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.HTML, "frontmatter-properties") {
		t.Error("should not render frontmatter section when none present")
	}
	if !strings.Contains(result.HTML, "No Frontmatter") {
		t.Error("expected body content to be rendered")
	}
}

func TestIsDateValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"2024-01-15", true},
		{"2024-12-31T23:59", true},
		{"2024-01-15 10:30", true},
		{"2024-01-15T10:30:45", true},
		{"not a date", false},
		{"2024", false},
		{"2024-1-1", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isDateValue(tt.input); got != tt.want {
			t.Errorf("isDateValue(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestRenderFrontmatterHTML_EmptyMap(t *testing.T) {
	t.Parallel()
	html := renderFrontmatterHTML(map[string]interface{}{}, nil)
	if html != "" {
		t.Error("expected empty string for empty properties map")
	}
}

func TestRenderFrontmatterHTML_NilMap(t *testing.T) {
	t.Parallel()
	html := renderFrontmatterHTML(nil, nil)
	if html != "" {
		t.Error("expected empty string for nil properties map")
	}
}

func TestRenderFrontmatterHTML_GenericList(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{
		"items": []interface{}{"one", "two", "three"},
	}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `class="frontmatter-pill"`) {
		t.Error("expected frontmatter-pill class for generic list items")
	}
	if !strings.Contains(html, "one") || !strings.Contains(html, "two") {
		t.Error("expected list items in output")
	}
}

func TestRenderFrontmatterHTML_TagWithHashPrefix(t *testing.T) {
	t.Parallel()
	// Tags that already have # prefix should not get double prefix
	props := map[string]interface{}{
		"tags": []interface{}{"#already-prefixed"},
	}
	html := renderFrontmatterHTML(props, nil)
	if strings.Contains(html, "##already-prefixed") {
		t.Error("should not double the # prefix on tags")
	}
	if !strings.Contains(html, "#already-prefixed") {
		t.Error("expected tag with single # prefix")
	}
}

func TestParseFrontmatter_WikiLinks(t *testing.T) {
	t.Parallel()
	input := []byte("---\ntitle: Test\nrelated:\n  - [[Page One]]\n  - [[Page Two]]\n---\n# Content")
	props, body := parseFrontmatter(input)
	if props == nil {
		t.Fatal("expected non-nil properties; wiki links in YAML should be handled")
	}
	if string(body) != "# Content" {
		t.Errorf("body = %q, want %q", body, "# Content")
	}
	related, ok := props["related"].([]interface{})
	if !ok {
		t.Fatalf("expected related to be []interface{}, got %T", props["related"])
	}
	if len(related) != 2 {
		t.Fatalf("expected 2 related items, got %d", len(related))
	}
	if related[0] != "[[Page One]]" {
		t.Errorf("related[0] = %q, want %q", related[0], "[[Page One]]")
	}
}

func TestRenderFrontmatterHTML_WikiLinks(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{
		"related": []interface{}{"[[My Page]]"},
	}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `<a class="wikilink"`) {
		t.Errorf("expected wiki link to be rendered as <a> tag, got %s", html)
	}
	if !strings.Contains(html, "My Page") {
		t.Error("expected wiki link display text")
	}
}

func TestMarkdown_MermaidBlock(t *testing.T) {
	t.Parallel()
	input := []byte("Before.\n\n```mermaid\ngraph LR\n    A --> B\n```\n\nAfter.")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, `<pre class="mermaid">`) {
		t.Errorf("expected <pre class=\"mermaid\"> wrapper, got %q", result.HTML)
	}
	// Diagram content should be HTML-escaped so browser does not parse it as HTML.
	if !strings.Contains(result.HTML, "A --&gt; B") {
		t.Errorf("expected escaped arrow in mermaid content, got %q", result.HTML)
	}
	// Should not be wrapped in chroma-highlighted <pre><code>.
	if strings.Contains(result.HTML, "chroma") || strings.Contains(result.HTML, "<code>graph LR") {
		t.Errorf("mermaid block should skip syntax highlighting, got %q", result.HTML)
	}
	// Surrounding paragraphs still render.
	if !strings.Contains(result.HTML, "<p>Before.</p>") || !strings.Contains(result.HTML, "<p>After.</p>") {
		t.Errorf("expected surrounding paragraphs preserved, got %q", result.HTML)
	}
}

func TestMarkdown_MermaidPreservesObsidianSyntax(t *testing.T) {
	t.Parallel()
	// Mermaid comments (%%...%%) and subroutine shapes ([[...]]) must survive
	// even though Obsidian preprocessing would normally consume them.
	input := []byte("```mermaid\n%%{init: {'theme':'dark'}}%%\ngraph LR\n    A[[Subroutine]] --> B\n```\n")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, "%%{init:") {
		t.Errorf("expected mermaid directive preserved, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, "A[[Subroutine]]") {
		t.Errorf("expected [[Subroutine]] preserved (not converted to wikilink), got %q", result.HTML)
	}
	if strings.Contains(result.HTML, "wikilink") {
		t.Errorf("[[...]] inside mermaid must not become a wikilink, got %q", result.HTML)
	}
}

func TestMarkdown_MultipleMermaidBlocks(t *testing.T) {
	t.Parallel()
	input := []byte("```mermaid\ngraph A\n```\n\nmiddle\n\n```mermaid\ngraph B\n```\n")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(result.HTML, `<pre class="mermaid">`) != 2 {
		t.Errorf("expected 2 mermaid blocks, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, "graph A") || !strings.Contains(result.HTML, "graph B") {
		t.Errorf("expected both diagrams retained, got %q", result.HTML)
	}
}

func TestMarkdown_MermaidAtDocumentStart(t *testing.T) {
	t.Parallel()
	// No preceding newline — regex must still match at \A.
	input := []byte("```mermaid\ngraph LR\n    A --> B\n```\n")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, `<pre class="mermaid">`) {
		t.Errorf("expected mermaid block at document start, got %q", result.HTML)
	}
	if strings.Contains(result.HTML, "MERMAIDBLOCK") {
		t.Errorf("placeholder token leaked into output: %q", result.HTML)
	}
}

func TestMarkdown_MermaidAtDocumentEnd(t *testing.T) {
	t.Parallel()
	// No trailing newline after closing fence.
	input := []byte("intro\n\n```mermaid\ngraph LR\n    A --> B\n```")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, `<pre class="mermaid">`) {
		t.Errorf("expected mermaid block at document end, got %q", result.HTML)
	}
	if strings.Contains(result.HTML, "MERMAIDBLOCK") {
		t.Errorf("placeholder token leaked into output: %q", result.HTML)
	}
}

func TestMarkdown_MermaidHTMLEscaping(t *testing.T) {
	t.Parallel()
	// A diagram containing HTML-ish content must be HTML-escaped in the output
	// so the browser does not parse it as real markup. This is the first line
	// of defense against accidental HTML/XSS from diagram source.
	input := []byte("```mermaid\ngraph LR\n    A[\"<script>alert('x')</script>\"] --> B\n```\n")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.HTML, "<script>alert") {
		t.Errorf("raw <script> must not appear in output, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, "&lt;script&gt;alert") {
		t.Errorf("expected escaped <script> inside mermaid, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, "&#39;x&#39;") && !strings.Contains(result.HTML, "'x'") {
		// htmlEscaper does not rewrite apostrophes; just ensure the quoted
		// string is retained in some form.
		t.Logf("apostrophe handling: %q", result.HTML)
	}
}

func TestMarkdown_MermaidDoesNotAffectOtherCodeBlocks(t *testing.T) {
	t.Parallel()
	// A non-mermaid fenced block must still go through goldmark's highlighting
	// path (producing a <pre> with <code>), not be rewritten as a mermaid block.
	input := []byte("```go\nfmt.Println(\"hi\")\n```\n\n```mermaid\ngraph LR\n    A --> B\n```\n")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(result.HTML, `<pre class="mermaid">`) != 1 {
		t.Errorf("expected exactly one mermaid block, got %q", result.HTML)
	}
	// goldmark-highlighting splits identifiers into <span> tags so we can't
	// match "fmt.Println" directly; just check for a distinctive token.
	if !strings.Contains(result.HTML, "Println") {
		t.Errorf("expected Go code block content retained, got %q", result.HTML)
	}
	goStart := strings.Index(result.HTML, "Println")
	pre := strings.LastIndex(result.HTML[:goStart], "<pre")
	if pre < 0 || strings.Contains(result.HTML[pre:goStart], `class="mermaid"`) {
		t.Errorf("go block should not be wrapped as mermaid, got %q", result.HTML)
	}
}

func TestMarkdown_MermaidWithFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("---\ntitle: Diagrams\n---\n\n```mermaid\ngraph LR\n    A --> B\n```\n")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, `class="frontmatter-properties"`) {
		t.Errorf("expected frontmatter rendered alongside mermaid, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, `<pre class="mermaid">`) {
		t.Errorf("expected mermaid block after frontmatter, got %q", result.HTML)
	}
}

func TestMarkdown_MermaidWithLanguageInfo(t *testing.T) {
	t.Parallel()
	// Some writers add info after the language token, e.g. ```mermaid theme=dark.
	input := []byte("```mermaid extra-info\ngraph LR\n    A --> B\n```\n")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, `<pre class="mermaid">`) {
		t.Errorf("expected mermaid block despite trailing info string, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, "A --&gt; B") {
		t.Errorf("expected diagram content preserved, got %q", result.HTML)
	}
}

func TestExtractMermaidBlocks_CountAndContent(t *testing.T) {
	t.Parallel()
	source := []byte("intro\n\n```mermaid\ngraph A\n    X --> Y\n```\n\nmiddle\n\n```mermaid\nsequenceDiagram\n    A->>B: hi\n```\n")
	_, blocks := extractMermaidBlocks(source)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 extracted blocks, got %d", len(blocks))
	}
	if blocks[0] != "graph A\n    X --> Y" {
		t.Errorf("block[0] = %q, want %q", blocks[0], "graph A\n    X --> Y")
	}
	if blocks[1] != "sequenceDiagram\n    A->>B: hi" {
		t.Errorf("block[1] = %q, want %q", blocks[1], "sequenceDiagram\n    A->>B: hi")
	}
}

func TestExtractMermaidBlocks_EmitsPlaceholders(t *testing.T) {
	t.Parallel()
	source := []byte("x\n\n```mermaid\ng\n```\n\ny\n")
	processed, blocks := extractMermaidBlocks(source)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	p := string(processed)
	if !strings.Contains(p, "MERMAIDBLOCK0PLACEHOLDER") {
		t.Errorf("expected placeholder in processed source, got %q", p)
	}
	if strings.Contains(p, "```mermaid") {
		t.Errorf("expected mermaid fence to be removed, got %q", p)
	}
}

func TestExtractMermaidBlocks_NoMermaid(t *testing.T) {
	t.Parallel()
	source := []byte("# Title\n\n```go\nfmt.Println()\n```\n\nplain text.")
	processed, blocks := extractMermaidBlocks(source)
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks when no mermaid present, got %d", len(blocks))
	}
	if string(processed) != string(source) {
		t.Errorf("expected source unchanged when no mermaid, got %q", processed)
	}
}

func TestMermaidPlaceholder_Unique(t *testing.T) {
	t.Parallel()
	// The placeholder tokens must differ per index so restoreMermaidBlocks can
	// unambiguously match each one back to its diagram.
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		p := mermaidPlaceholder(i)
		if seen[p] {
			t.Errorf("duplicate placeholder for index %d: %q", i, p)
		}
		seen[p] = true
	}
}

func TestRestoreMermaidBlocks_ReplacesAndEscapes(t *testing.T) {
	t.Parallel()
	html := "<p>before</p>\n<p>" + mermaidPlaceholder(0) + "</p>\n<p>after</p>"
	restored := restoreMermaidBlocks(html, []string{"graph LR\n    A --> <B>"})
	if !strings.Contains(restored, `<pre class="mermaid">`) {
		t.Errorf("expected mermaid wrapper, got %q", restored)
	}
	if !strings.Contains(restored, "--&gt; &lt;B&gt;") {
		t.Errorf("expected angle brackets escaped, got %q", restored)
	}
	if strings.Contains(restored, mermaidPlaceholder(0)) {
		t.Errorf("placeholder should be gone, got %q", restored)
	}
}

func TestRestoreMermaidBlocks_NoBlocks(t *testing.T) {
	t.Parallel()
	html := "<p>hello</p>"
	restored := restoreMermaidBlocks(html, nil)
	if restored != html {
		t.Errorf("expected unchanged html, got %q", restored)
	}
}

func TestMarkdown_MermaidInlineCodeSpansNotAffected(t *testing.T) {
	t.Parallel()
	// An inline code span containing the token "```mermaid" should not be
	// treated as a fenced block. The regex requires a newline before the
	// opening fence, which backticks inside a paragraph do not provide.
	input := []byte("This paragraph mentions `mermaid` inline.")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.HTML, `<pre class="mermaid">`) {
		t.Errorf("inline mention of mermaid must not produce a diagram, got %q", result.HTML)
	}
}

func TestMarkdown_MermaidWithSurroundingMarkdown(t *testing.T) {
	t.Parallel()
	// End-to-end: headings, wiki links, and callouts around a mermaid block
	// should all render normally, and the block itself should render once.
	input := []byte("# Heading\n\nSee [[Other Page]].\n\n```mermaid\ngraph LR\n    A --> B\n```\n\n> [!note] Reminder\n> Body text.\n")
	result, err := Markdown(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.HTML, `<pre class="mermaid">`) {
		t.Errorf("expected mermaid block, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, `class="wikilink"`) {
		t.Errorf("expected wikilink rendered, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, "callout-note") {
		t.Errorf("expected callout rendered, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, `<h1 id="heading"`) {
		t.Errorf("expected heading rendered, got %q", result.HTML)
	}
}

func TestRenderFrontmatterHTML_WikiLinkStringValue(t *testing.T) {
	t.Parallel()
	props := map[string]interface{}{
		"source": "[[Some Note]]",
	}
	html := renderFrontmatterHTML(props, nil)
	if !strings.Contains(html, `<a class="wikilink"`) {
		t.Errorf("expected wiki link string rendered as <a> tag, got %s", html)
	}
}
