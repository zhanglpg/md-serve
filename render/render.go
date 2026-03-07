package render

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// TOCEntry represents a heading in the table of contents.
type TOCEntry struct {
	Level int
	ID    string
	Title string
}

// Result holds the rendered HTML and extracted metadata.
type Result struct {
	HTML string
	TOC  []TOCEntry
}

// Markdown converts markdown source bytes to an HTML Result with TOC.
func Markdown(source []byte) (*Result, error) {
	// Pre-process Obsidian-specific syntax before goldmark parsing
	processed := preprocessObsidian(source)

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithGuessLanguage(true),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(processed, &buf); err != nil {
		return nil, err
	}

	// Extract TOC from source
	toc := extractTOC(source)

	// Post-process for remaining Obsidian features
	output := postprocessObsidian(buf.String())

	return &Result{
		HTML: output,
		TOC:  toc,
	}, nil
}

var headingRe = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)

// extractTOC extracts headings from the raw markdown source.
func extractTOC(source []byte) []TOCEntry {
	matches := headingRe.FindAllSubmatch(source, -1)
	var entries []TOCEntry
	for _, m := range matches {
		level := len(m[1])
		title := strings.TrimSpace(string(m[2]))
		// Strip inline markdown (bold, italic, code, links)
		title = regexp.MustCompile(`[*_` + "`" + `]`).ReplaceAllString(title, "")
		title = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(title, "$1")
		id := generateHeadingID(title)
		entries = append(entries, TOCEntry{Level: level, ID: id, Title: title})
	}
	return entries
}

func generateHeadingID(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^\w\s-]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

var (
	// ==highlight==
	highlightRe = regexp.MustCompile(`==([^=]+)==`)
	// [[wikilink]] and [[wikilink|display]]
	wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	// > [!type] callout
	calloutStartRe = regexp.MustCompile(`(?m)^<blockquote>\n<p>\[!(\w+)\]\s*(.*)`)
	// %%comment%%
	commentRe = regexp.MustCompile(`%%[^%]*%%`)
)

// preprocessObsidian handles syntax that must be processed before goldmark.
func preprocessObsidian(source []byte) []byte {
	s := string(source)

	// Remove Obsidian comments %%...%%
	s = commentRe.ReplaceAllString(s, "")

	// Convert ==highlight== to <mark> tags (in non-code contexts)
	s = highlightRe.ReplaceAllString(s, "<mark>$1</mark>")

	return []byte(s)
}

// postprocessObsidian handles HTML-level transformations after rendering.
func postprocessObsidian(html string) string {
	// Convert wiki-links: [[target]] or [[target|display]]
	html = wikilinkRe.ReplaceAllStringFunc(html, func(match string) string {
		parts := wikilinkRe.FindStringSubmatch(match)
		target := parts[1]
		display := target
		if parts[2] != "" {
			display = parts[2]
		}
		// Link to the markdown file path
		href := strings.ReplaceAll(target, " ", "-")
		if !strings.HasSuffix(href, ".md") {
			href += ".md"
		}
		return `<a class="wikilink" href="/` + href + `">` + display + `</a>`
	})

	// Convert callouts: > [!type] title
	html = calloutStartRe.ReplaceAllStringFunc(html, func(match string) string {
		parts := calloutStartRe.FindStringSubmatch(match)
		calloutType := strings.ToLower(parts[1])
		title := parts[2]
		if title == "" {
			title = strings.Title(calloutType)
		}
		return `<blockquote class="callout callout-` + calloutType + `">` +
			`<div class="callout-title"><span class="callout-icon"></span>` + title + `</div>` +
			"\n<p>"
	})

	return html
}
