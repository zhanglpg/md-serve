package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
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

// frontmatterRe matches YAML frontmatter delimited by --- at the start of the file.
var frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.+?)\r?\n---\r?\n?`)

// wikiLinkYAMLRe matches Obsidian wiki links inside YAML values so they can be
// quoted before parsing. Unquoted [[...]] is interpreted as nested YAML flow
// sequences which causes parse failures.
var wikiLinkYAMLRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// parseFrontmatter extracts YAML frontmatter from the source and returns the
// parsed properties and the remaining markdown body. If no valid frontmatter
// is found, properties is nil and body equals source. The frontmatter block is
// always stripped from the body when the regex matches, even if YAML parsing fails.
func parseFrontmatter(source []byte) (properties map[string]interface{}, body []byte) {
	m := frontmatterRe.FindSubmatch(source)
	if m == nil {
		return nil, source
	}
	// Always strip frontmatter from body, even if YAML parsing fails.
	body = source[len(m[0]):]

	// Pre-process: quote Obsidian wiki links so YAML doesn't treat [[...]] as flow sequences.
	yamlBytes := wikiLinkYAMLRe.ReplaceAll(m[1], []byte(`"[[$1]]"`))

	var props map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &props); err != nil {
		return nil, body
	}
	return props, body
}

// renderFrontmatterHTML renders parsed frontmatter properties as a styled HTML section.
func renderFrontmatterHTML(props map[string]interface{}, opts *RenderOptions) string {
	if len(props) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="frontmatter-properties">`)
	b.WriteString("\n")
	for _, key := range sortedKeys(props) {
		val := props[key]
		b.WriteString(`<div class="frontmatter-property">`)
		b.WriteString(`<span class="frontmatter-key">` + htmlEscaper.Replace(key) + `</span>`)
		b.WriteString(`<span class="frontmatter-value">`)
		b.WriteString(renderPropertyValue(key, val, opts))
		b.WriteString(`</span>`)
		b.WriteString("</div>\n")
	}
	b.WriteString("</section>\n")
	return b.String()
}

// sortedKeys returns map keys in sorted order for deterministic output.
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Sort alphabetically
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// renderPropertyValue renders a single frontmatter property value as HTML.
func renderPropertyValue(key string, val interface{}, opts *RenderOptions) string {
	if val == nil {
		return `<span class="frontmatter-empty">—</span>`
	}
	switch v := val.(type) {
	case bool:
		if v {
			return `<input type="checkbox" checked disabled>`
		}
		return `<input type="checkbox" disabled>`
	case []interface{}:
		return renderListValue(key, v, opts)
	case string:
		return renderStringValue(key, v, opts)
	case int:
		return htmlEscaper.Replace(fmt.Sprintf("%d", v))
	case float64:
		s := fmt.Sprintf("%g", v)
		return htmlEscaper.Replace(s)
	case time.Time:
		return renderTimeValue(v)
	default:
		return htmlEscaper.Replace(fmt.Sprintf("%v", v))
	}
}

// renderListValue renders a list property as a series of tags/pills.
func renderListValue(key string, items []interface{}, opts *RenderOptions) string {
	if len(items) == 0 {
		return `<span class="frontmatter-empty">—</span>`
	}
	var b strings.Builder
	b.WriteString(`<span class="frontmatter-list">`)
	for _, item := range items {
		s := fmt.Sprintf("%v", item)
		// Render wiki links as clickable links
		if wikiTarget := extractWikiLink(s); wikiTarget != "" {
			b.WriteString(renderFrontmatterWikilink(wikiTarget, opts))
			continue
		}
		cssClass := "frontmatter-pill"
		display := htmlEscaper.Replace(s)
		if key == "tags" {
			cssClass = "frontmatter-tag"
			if !strings.HasPrefix(s, "#") {
				display = "#" + display
			}
		} else if key == "aliases" {
			cssClass = "frontmatter-alias"
		} else if key == "cssclasses" || key == "cssclass" {
			cssClass = "frontmatter-cssclass"
		}
		b.WriteString(`<span class="` + cssClass + `">` + display + `</span>`)
	}
	b.WriteString(`</span>`)
	return b.String()
}

// extractWikiLink returns the target if s is a wiki link like "[[Target]]", or "" otherwise.
func extractWikiLink(s string) string {
	if strings.HasPrefix(s, "[[") && strings.HasSuffix(s, "]]") {
		return s[2 : len(s)-2]
	}
	return ""
}

// renderFrontmatterWikilink renders a wiki link target as a clickable <a> tag in frontmatter.
func renderFrontmatterWikilink(target string, opts *RenderOptions) string {
	display := target
	if strings.Contains(display, "/") {
		display = filepath.Base(display)
	}
	// Handle display alias: [[target|alias]]
	if idx := strings.Index(target, "|"); idx >= 0 {
		display = target[idx+1:]
		target = target[:idx]
	}
	var prefix, vaultDir string
	if opts != nil {
		prefix = opts.URLPrefix
		vaultDir = opts.VaultDir
	}
	href := resolveWikiHref(vaultDir, target)
	if href == target && !isAttachment(href) && !strings.HasSuffix(href, ".md") {
		href += ".md"
	}
	href = URLEncodePath(href)
	return `<a class="wikilink" href="` + prefix + `/` + href + `">` + htmlEscaper.Replace(display) + `</a>`
}

// renderStringValue renders a string property, detecting dates and wiki links.
func renderStringValue(key string, s string, opts *RenderOptions) string {
	// Detect wiki links
	if wikiTarget := extractWikiLink(s); wikiTarget != "" {
		return renderFrontmatterWikilink(wikiTarget, opts)
	}
	escaped := htmlEscaper.Replace(s)
	// Detect date/datetime patterns
	if isDateValue(s) {
		return `<time class="frontmatter-date">` + escaped + `</time>`
	}
	// Render single tag value with # prefix
	if key == "tags" && !strings.HasPrefix(s, "#") {
		return `<span class="frontmatter-tag">#` + escaped + `</span>`
	}
	return escaped
}

// renderTimeValue formats a time.Time value (parsed by YAML) as an HTML <time> element.
func renderTimeValue(t time.Time) string {
	var display string
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
		display = t.Format("2006-01-02")
	} else {
		display = t.Format("2006-01-02 15:04")
	}
	return `<time class="frontmatter-date">` + display + `</time>`
}

// dateRe matches ISO date (YYYY-MM-DD) and datetime (YYYY-MM-DDTHH:MM or YYYY-MM-DD HH:MM) formats.
var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}([ T]\d{2}:\d{2}(:\d{2})?)?$`)

// isDateValue returns true if the string looks like a date or datetime.
func isDateValue(s string) bool {
	return dateRe.MatchString(s)
}

// Markdown converts markdown source bytes to an HTML Result with TOC.
// If opts is provided, wiki links are resolved by searching the vault directory.
func Markdown(source []byte, opts *RenderOptions) (*Result, error) {
	// Extract frontmatter before processing
	props, body := parseFrontmatter(source)

	// Extract mermaid blocks before any other preprocessing so their contents
	// (which can include %%...%% and [[...]]) are not rewritten as comments,
	// wiki links, or sent through syntax highlighting.
	processed, mermaidBlocks := extractMermaidBlocks(body)

	// Pre-process Obsidian-specific syntax before goldmark parsing.
	// Embeds are handled here so goldmark does not interfere with ![[...]] syntax.
	processed = preprocessObsidian(processed, opts)

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

	// Extract TOC from body (without frontmatter)
	toc := extractTOC(body)

	// Post-process for remaining Obsidian features
	output := postprocessObsidian(buf.String(), opts)

	// Restore mermaid diagram blocks as <pre class="mermaid"> elements.
	output = restoreMermaidBlocks(output, mermaidBlocks)

	// Prepend rendered frontmatter properties
	if props != nil {
		output = renderFrontmatterHTML(props, opts) + output
	}

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
	// ```mermaid fenced code block. Captures inner diagram source. Matches at
	// file start or after a newline so it doesn't fire inside inline spans.
	mermaidBlockRe = regexp.MustCompile("(?s)(?:\\A|\\n)```mermaid[^\\n]*\\n(.*?)\\n```")

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

// extractMermaidBlocks finds fenced ```mermaid code blocks, replaces each with
// a unique placeholder paragraph, and returns the rewritten source plus the
// captured diagram sources in order. Placeholders are plain alphanumeric tokens
// surrounded by blank lines so goldmark wraps them in a <p>...</p> that
// restoreMermaidBlocks can later pattern-match and replace.
func extractMermaidBlocks(source []byte) ([]byte, []string) {
	var blocks []string
	processed := mermaidBlockRe.ReplaceAllFunc(source, func(match []byte) []byte {
		sub := mermaidBlockRe.FindSubmatch(match)
		idx := len(blocks)
		blocks = append(blocks, string(sub[1]))
		// Preserve a leading newline if the match consumed one so paragraph
		// boundaries upstream are not disturbed.
		prefix := ""
		if len(match) > 0 && match[0] == '\n' {
			prefix = "\n"
		}
		return []byte(prefix + "\n" + mermaidPlaceholder(idx) + "\n")
	})
	return processed, blocks
}

func mermaidPlaceholder(idx int) string {
	return fmt.Sprintf("MERMAIDBLOCK%dPLACEHOLDER", idx)
}

// restoreMermaidBlocks swaps mermaid placeholder paragraphs in the rendered
// HTML back for <pre class="mermaid"> elements containing the HTML-escaped
// diagram source.
func restoreMermaidBlocks(html string, blocks []string) string {
	for i, block := range blocks {
		placeholder := "<p>" + mermaidPlaceholder(i) + "</p>"
		replacement := `<pre class="mermaid">` + htmlEscaper.Replace(block) + `</pre>`
		html = strings.Replace(html, placeholder, replacement, 1)
	}
	return html
}

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
// shadowExts lists the file extensions for Excalidraw shadow files.
var shadowExts = []string{".svg", ".png"}

// findShadowDirect checks for a shadow file adjacent to the excalidraw file.
func findShadowDirect(vaultDir, absBase string) string {
	for _, ext := range shadowExts {
		candidate := absBase + ext
		if fileExists(candidate) {
			rel, _ := filepath.Rel(vaultDir, candidate)
			return rel
		}
	}
	return ""
}

// findShadowBySearch does a vault-wide search for a shadow file by basename.
func findShadowBySearch(vaultDir, baseName string) string {
	for _, ext := range shadowExts {
		if resolved := ResolveWikiTarget(vaultDir, baseName+ext); resolved != "" {
			return resolved
		}
	}
	return ""
}

func findExcalidrawShadow(vaultDir, filePath string) string {
	if vaultDir == "" {
		return ""
	}
	absBase := filepath.Join(vaultDir, filepath.Clean(filePath))
	if found := findShadowDirect(vaultDir, absBase); found != "" {
		return found
	}
	return findShadowBySearch(vaultDir, filepath.Base(filePath))
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

// basenameMatches reports whether name matches the target or its space/hyphen alternative.
func basenameMatches(name, target, alt string) bool {
	return name == target || (alt != "" && name == alt)
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
		if basenameMatches(strings.ToLower(filepath.Base(path)), basename, alt) {
			if rel, relErr := filepath.Rel(vaultDir, path); relErr == nil {
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
