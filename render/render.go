package render

import (
	"bytes"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
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

// RenderOptions provides context for resolving wiki links during rendering.
type RenderOptions struct {
	// VaultDir is the root directory of the vault, used to search for wiki link targets.
	VaultDir string
	// URLPrefix is prepended to all generated wiki link hrefs (e.g. "/vaultName" in multi-vault mode).
	URLPrefix string
}

// Markdown converts markdown source bytes to an HTML Result with TOC.
// If opts is provided, wiki links are resolved by searching the vault directory.
func Markdown(source []byte, opts *RenderOptions) (*Result, error) {
	// Pre-process Obsidian-specific syntax before goldmark parsing.
	// Embeds are handled here so goldmark does not interfere with ![[...]] syntax.
	processed := preprocessObsidian(source, opts)

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
	output := postprocessObsidian(buf.String(), opts)

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
	// ![[embed]] and ![[embed|alt text]] for images/attachments
	embedRe = regexp.MustCompile(`!\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	// [[wikilink]] and [[wikilink|display]]
	wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	// > [!type] callout
	calloutStartRe = regexp.MustCompile(`(?m)^<blockquote>\n<p>\[!(\w+)\]\s*(.*)`)
	// %%comment%%
	commentRe = regexp.MustCompile(`%%[^%]*%%`)

	// imageExts lists file extensions that should be rendered as <img> embeds.
	imageExts = map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".svg": true, ".webp": true, ".bmp": true, ".ico": true,
		".avif": true, ".apng": true, ".tiff": true, ".tif": true,
	}
)

// preprocessObsidian handles syntax that must be processed before goldmark.
// Embeds (![[...]]) are converted here to prevent goldmark from interfering
// with the bracket syntax.
func preprocessObsidian(source []byte, opts *RenderOptions) []byte {
	s := string(source)

	// Remove Obsidian comments %%...%%
	s = commentRe.ReplaceAllString(s, "")

	// Convert ==highlight== to <mark> tags (in non-code contexts)
	s = highlightRe.ReplaceAllString(s, "<mark>$1</mark>")

	// Convert embeds: ![[target]] or ![[target|alt text]]
	// Must be done before goldmark to avoid parser interference with ![[ syntax.
	prefix := ""
	vaultDir := ""
	if opts != nil {
		prefix = opts.URLPrefix
		vaultDir = opts.VaultDir
	}
	s = embedRe.ReplaceAllStringFunc(s, func(match string) string {
		parts := embedRe.FindStringSubmatch(match)
		target := parts[1]
		alt := target
		if parts[2] != "" {
			alt = parts[2]
		}
		resolved := resolveWikiTarget(vaultDir, target)
		pathForURL := target
		if resolved != "" {
			pathForURL = resolved
		}
		href := prefix + "/" + urlEncodePath(pathForURL)
		ext := strings.ToLower(filepath.Ext(target))
		if imageExts[ext] {
			return `<img src="` + href + `" alt="` + alt + `" />`
		}
		return `<a class="wikilink embed" href="` + href + `">` + alt + `</a>`
	})

	return []byte(s)
}

// isAttachment returns true if the target path has a non-markdown file extension,
// indicating it's an attachment (image, PDF, excalidraw, etc.) rather than a note.
func isAttachment(target string) bool {
	ext := strings.ToLower(filepath.Ext(target))
	return ext != "" && ext != ".md" && ext != ".markdown"
}

// urlEncodePath URL-encodes each segment of a slash-separated path.
func urlEncodePath(path string) string {
	segments := strings.Split(path, "/")
	for i, p := range segments {
		segments[i] = url.PathEscape(p)
	}
	return strings.Join(segments, "/")
}

// resolveWikiTarget searches the vault directory for a file matching the target.
// It returns the relative path from the vault root, or empty string if not found.
func resolveWikiTarget(vaultDir, target string) string {
	if vaultDir == "" {
		return ""
	}

	// Determine the filename to search for
	searchName := target
	if !isAttachment(target) && !strings.HasSuffix(strings.ToLower(target), ".md") {
		searchName = target + ".md"
	}

	// First, try direct path relative to vault root
	directPath := filepath.Join(vaultDir, filepath.Clean(searchName))
	if _, err := os.Stat(directPath); err == nil {
		rel, _ := filepath.Rel(vaultDir, directPath)
		return rel
	}

	// Search by basename (case-insensitive), matching Obsidian behavior
	basename := strings.ToLower(filepath.Base(searchName))
	altBasename := ""
	if strings.Contains(basename, " ") {
		altBasename = strings.ReplaceAll(basename, " ", "-")
	} else if strings.Contains(basename, "-") {
		altBasename = strings.ReplaceAll(basename, "-", " ")
	}

	var match string
	filepath.WalkDir(vaultDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := strings.ToLower(filepath.Base(path))
		if name == basename || (altBasename != "" && name == altBasename) {
			rel, err := filepath.Rel(vaultDir, path)
			if err == nil {
				match = rel
				return filepath.SkipAll
			}
		}
		return nil
	})
	return match
}

// postprocessObsidian handles HTML-level transformations after rendering.
func postprocessObsidian(html string, opts *RenderOptions) string {
	prefix := ""
	vaultDir := ""
	if opts != nil {
		prefix = opts.URLPrefix
		vaultDir = opts.VaultDir
	}

	// Convert wiki-links: [[target]] or [[target|display]]
	html = wikilinkRe.ReplaceAllStringFunc(html, func(match string) string {
		parts := wikilinkRe.FindStringSubmatch(match)
		target := parts[1]
		display := target
		if parts[2] != "" {
			display = parts[2]
		}
		href := target
		// Only add .md suffix for note links, not attachments
		if !isAttachment(href) && !strings.HasSuffix(href, ".md") {
			href += ".md"
		}
		// For display text of path links, show only the filename (without path)
		if parts[2] == "" && strings.Contains(display, "/") {
			display = filepath.Base(display)
		}
		// Resolve the wiki link target path in the vault
		resolved := resolveWikiTarget(vaultDir, target)
		if resolved != "" {
			href = resolved
		}
		href = urlEncodePath(href)
		return `<a class="wikilink" href="` + prefix + `/` + href + `">` + display + `</a>`
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
