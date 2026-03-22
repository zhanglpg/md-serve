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

	// htmlEscaper escapes HTML-significant characters in display text.
	htmlEscaper = strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)

	// ImageExts lists file extensions that should be rendered as <img> embeds.
	ImageExts = map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".svg": true, ".webp": true, ".bmp": true, ".ico": true,
		".avif": true, ".apng": true, ".tiff": true, ".tif": true,
	}

	// excalidrawExt is the file extension for Excalidraw drawings.
	excalidrawExt = ".excalidraw"
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
		return renderEmbed(embedRe.FindStringSubmatch(match), vaultDir, prefix)
	})

	// Convert wiki links: [[target]] or [[target|display]]
	// Processed before goldmark to prevent angle brackets (< > 《 》) in
	// targets from being interpreted as HTML tags by the markdown parser.
	s = wikilinkRe.ReplaceAllStringFunc(s, func(match string) string {
		return renderWikilink(wikilinkRe.FindStringSubmatch(match), vaultDir, prefix)
	})

	return []byte(s)
}

// renderEmbed converts an ![[embed]] match into an <img> or <a> tag.
func renderEmbed(parts []string, vaultDir, prefix string) string {
	target := parts[1]
	alt := target
	if parts[2] != "" {
		alt = parts[2]
	}
	pathForURL := resolveWikiHref(vaultDir, target)
	href := prefix + "/" + URLEncodePath(pathForURL)
	escapedAlt := htmlEscaper.Replace(alt)
	ext := strings.ToLower(filepath.Ext(target))
	if ImageExts[ext] || ext == excalidrawExt {
		return `<img src="` + href + `" alt="` + escapedAlt + `" />`
	}
	return `<a class="wikilink embed" href="` + href + `">` + escapedAlt + `</a>`
}

// renderWikilink converts a [[wikilink]] match into an <a> tag.
func renderWikilink(parts []string, vaultDir, prefix string) string {
	target := parts[1]
	display := target
	if parts[2] != "" {
		display = parts[2]
	}
	if parts[2] == "" && strings.Contains(display, "/") {
		display = filepath.Base(display)
	}
	href := resolveWikiHref(vaultDir, target)
	if href == target && !isAttachment(href) && !strings.HasSuffix(href, ".md") {
		href += ".md"
	}
	href = URLEncodePath(href)
	return `<a class="wikilink" href="` + prefix + `/` + href + `">` + htmlEscaper.Replace(display) + `</a>`
}

// findExcalidrawShadow looks for a shadow SVG or PNG file exported by Obsidian
// alongside the .excalidraw file. Returns the relative path from vaultDir if found.
//
// When the direct path check fails, it falls back to a vault-wide search
// using resolveWikiTarget, matching Obsidian's behavior where wiki links
// don't include relative paths.
func findExcalidrawShadow(vaultDir, filePath string) string {
	if vaultDir == "" {
		return ""
	}
	absBase := filepath.Join(vaultDir, filepath.Clean(filePath))
	for _, ext := range []string{".svg", ".png"} {
		candidate := absBase + ext
		if fileExists(candidate) {
			rel, _ := filepath.Rel(vaultDir, candidate)
			return rel
		}
	}
	// Fallback: search the whole vault for the shadow file by basename,
	// reusing resolveWikiTarget which handles vault-wide resolution.
	baseName := filepath.Base(filePath)
	for _, ext := range []string{".svg", ".png"} {
		if resolved := ResolveWikiTarget(vaultDir, baseName+ext); resolved != "" {
			return resolved
		}
	}
	return ""
}

// resolveWikiHref resolves a wiki link target to the best available path.
// It performs vault-wide resolution and, for excalidraw files, returns the
// shadow SVG/PNG path if available.
func resolveWikiHref(vaultDir, target string) string {
	resolved := ResolveWikiTarget(vaultDir, target)
	href := target
	if resolved != "" {
		href = resolved
	}
	if strings.ToLower(filepath.Ext(href)) == excalidrawExt && vaultDir != "" {
		if shadow := findExcalidrawShadow(vaultDir, href); shadow != "" {
			return shadow
		}
	}
	return href
}

// fileExists returns true if the file exists on disk.
func fileExists(fullPath string) bool {
	_, err := os.Stat(fullPath)
	return err == nil
}

// isAttachment returns true if the target path has a non-markdown file extension,
// indicating it's an attachment (image, PDF, excalidraw, etc.) rather than a note.
func isAttachment(target string) bool {
	ext := strings.ToLower(filepath.Ext(target))
	return ext != "" && ext != ".md" && ext != ".markdown"
}

// URLEncodePath URL-encodes each segment of a slash-separated path.
func URLEncodePath(path string) string {
	segments := strings.Split(path, "/")
	for i, p := range segments {
		segments[i] = url.PathEscape(p)
	}
	return strings.Join(segments, "/")
}

// ResolveWikiTarget searches the vault directory for a file matching the target.
// It returns the relative path from the vault root, or empty string if not found.
// For non-attachment targets without a .md suffix, it appends .md automatically.
// Resolution tries a direct path first, then falls back to a vault-wide
// case-insensitive basename search with space/hyphen interoperability.
func ResolveWikiTarget(vaultDir, target string) string {
	if vaultDir == "" {
		return ""
	}

	searchName := wikiSearchName(target)

	// First, try direct path relative to vault root
	directPath := filepath.Join(vaultDir, filepath.Clean(searchName))
	if fileExists(directPath) {
		rel, _ := filepath.Rel(vaultDir, directPath)
		return rel
	}

	return searchVaultByBasename(vaultDir, searchName)
}

// wikiSearchName returns the filename to search for, appending .md for note targets.
func wikiSearchName(target string) string {
	if !isAttachment(target) && !strings.HasSuffix(strings.ToLower(target), ".md") {
		return target + ".md"
	}
	return target
}

// altBasename returns an alternative basename with spaces/hyphens swapped, or empty.
func altBasename(name string) string {
	if strings.Contains(name, " ") {
		return strings.ReplaceAll(name, " ", "-")
	}
	if strings.Contains(name, "-") {
		return strings.ReplaceAll(name, "-", " ")
	}
	return ""
}

// searchVaultByBasename walks the vault looking for a file matching by basename
// (case-insensitive, with space/hyphen interoperability).
func searchVaultByBasename(vaultDir, searchName string) string {
	basename := strings.ToLower(filepath.Base(searchName))
	alt := altBasename(basename)

	var match string
	filepath.WalkDir(vaultDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := strings.ToLower(filepath.Base(path))
		if name == basename || (alt != "" && name == alt) {
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
	// Use eager loading for all images (block until loaded)
	html = strings.ReplaceAll(html, `<img src=`, `<img loading="eager" src=`)

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
