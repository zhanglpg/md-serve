package server

import (
	"html/template"

	"github.com/zhanglpg/md-serve/render"
)

type pageData struct {
	SiteTitle   string
	PageTitle   string
	Content     template.HTML
	TOC         []render.TOCEntry
	Breadcrumbs []Breadcrumb
}

type dirData struct {
	SiteTitle   string
	DirName     string
	Dirs        []DirEntry
	Files       []DirEntry
	Breadcrumbs []Breadcrumb
	SortBy      string // "name" or "date"
	ReadmeHTML  template.HTML
}

type searchData struct {
	SiteTitle string
	Query     string
	Results   []searchResult
}

type imageViewerData struct {
	SiteTitle   string
	FileName    string
	ImageURL    string
	Breadcrumbs []Breadcrumb
}

type excalidrawViewerData struct {
	SiteTitle       string
	FileName        string
	ExcalidrawJSON  string
	Breadcrumbs     []Breadcrumb
}

type landingData struct {
	SiteTitle string
	Vaults    []Vault
}

const baseCSS = `
:root {
  --bg: #ffffff;
  --bg-secondary: #f8f9fa;
  --bg-tertiary: #e9ecef;
  --text: #212529;
  --text-secondary: #6c757d;
  --accent: #7c3aed;
  --accent-light: #ede9fe;
  --border: #dee2e6;
  --link: #6d28d9;
  --link-hover: #5b21b6;
  --code-bg: #282a36;
  --callout-info: #dbeafe;
  --callout-warning: #fef3c7;
  --callout-danger: #fee2e2;
  --callout-tip: #d1fae5;
  --callout-note: #e0e7ff;
  --callout-example: #fae8ff;
  --mark-bg: #fef08a;
  --sidebar-width: 260px;
  --toc-width: 240px;
}

[data-theme="dark"] {
    --bg: #1a1b26;
    --bg-secondary: #24283b;
    --bg-tertiary: #343a52;
    --text: #c0caf5;
    --text-secondary: #565f89;
    --accent: #bb9af7;
    --accent-light: #2d2054;
    --border: #3b4261;
    --link: #bb9af7;
    --link-hover: #9d7cd8;
    --code-bg: #1a1b26;
    --callout-info: #1e3a5f;
    --callout-warning: #3d2e00;
    --callout-danger: #3d1515;
    --callout-tip: #0d3b2a;
    --callout-note: #1e2456;
    --callout-example: #2e1541;
    --mark-bg: #854d0e;
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Noto Sans', sans-serif;
  color: var(--text);
  background: var(--bg);
  line-height: 1.7;
  font-size: 16px;
}

a { color: var(--link); text-decoration: none; }
a:hover { color: var(--link-hover); text-decoration: underline; }

.layout {
  display: flex;
  min-height: 100vh;
}

.topbar {
  position: sticky;
  top: 0;
  z-index: 100;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  padding: 0.5rem 1.5rem;
  display: flex;
  align-items: center;
  gap: 1rem;
}

.topbar .site-title {
  font-weight: 700;
  font-size: 1.1rem;
  color: var(--accent);
  white-space: nowrap;
}

.topbar .search-form {
  flex: 1;
  max-width: 400px;
}

.topbar .search-form input {
  width: 100%;
  padding: 0.4rem 0.8rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  color: var(--text);
  font-size: 0.9rem;
}

.breadcrumbs {
  padding: 0.5rem 0;
  font-size: 0.85rem;
  color: var(--text-secondary);
}
.breadcrumbs a { color: var(--text-secondary); }
.breadcrumbs a:hover { color: var(--accent); }
.breadcrumbs .sep { margin: 0 0.3rem; }

.main-content {
  flex: 1;
  min-width: 0;
  max-width: 900px;
  margin: 0 auto;
  padding: 1rem 2rem 3rem;
}

/* Table of Contents */
.toc-sidebar {
  position: sticky;
  top: 60px;
  width: var(--toc-width);
  max-height: calc(100vh - 80px);
  overflow-y: auto;
  padding: 1rem;
  font-size: 0.82rem;
  flex-shrink: 0;
  border-left: 1px solid var(--border);
}
.toc-sidebar .toc-title {
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  font-size: 0.72rem;
  letter-spacing: 0.05em;
  margin-bottom: 0.5rem;
}
.toc-sidebar ul { list-style: none; }
.toc-sidebar li { padding: 0.15rem 0; }
.toc-sidebar a {
  color: var(--text-secondary);
  font-size: 0.82rem;
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.toc-sidebar a:hover { color: var(--accent); text-decoration: none; }
.toc-level-2 { padding-left: 0; }
.toc-level-3 { padding-left: 0.8rem; }
.toc-level-4 { padding-left: 1.6rem; }
.toc-level-5 { padding-left: 2.4rem; }
.toc-level-6 { padding-left: 3.2rem; }

@media (max-width: 1100px) {
  .toc-sidebar { display: none; }
}

/* Markdown Content Styles */
.md-content h1, .md-content h2, .md-content h3,
.md-content h4, .md-content h5, .md-content h6 {
  margin-top: 1.5em;
  margin-bottom: 0.5em;
  font-weight: 600;
  line-height: 1.3;
}
.md-content h1 { font-size: 2em; border-bottom: 2px solid var(--border); padding-bottom: 0.3em; }
.md-content h2 { font-size: 1.5em; border-bottom: 1px solid var(--border); padding-bottom: 0.2em; }
.md-content h3 { font-size: 1.25em; }
.md-content h4 { font-size: 1.1em; }

.md-content p { margin: 0.8em 0; }

.md-content img {
  max-width: 100%;
  height: auto;
  border-radius: 6px;
}

.md-content blockquote {
  border-left: 4px solid var(--accent);
  margin: 1em 0;
  padding: 0.5em 1em;
  background: var(--bg-secondary);
  border-radius: 0 6px 6px 0;
}

.md-content pre {
  background: var(--code-bg);
  border-radius: 8px;
  padding: 1em;
  overflow-x: auto;
  margin: 1em 0;
  font-size: 0.9em;
}
.md-content code {
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', Consolas, monospace;
  font-size: 0.9em;
}
.md-content :not(pre) > code {
  background: var(--bg-tertiary);
  padding: 0.15em 0.4em;
  border-radius: 4px;
}

.md-content table {
  border-collapse: collapse;
  width: 100%;
  margin: 1em 0;
}
.md-content th, .md-content td {
  border: 1px solid var(--border);
  padding: 0.5em 0.8em;
  text-align: left;
}
.md-content th {
  background: var(--bg-secondary);
  font-weight: 600;
}
.md-content tr:nth-child(even) { background: var(--bg-secondary); }

.md-content ul, .md-content ol { padding-left: 1.5em; margin: 0.5em 0; }
.md-content li { margin: 0.25em 0; }

.md-content input[type="checkbox"] {
  margin-right: 0.4em;
  accent-color: var(--accent);
}

.md-content hr {
  border: none;
  border-top: 2px solid var(--border);
  margin: 2em 0;
}

.md-content mark {
  background: var(--mark-bg);
  padding: 0.1em 0.2em;
  border-radius: 2px;
}

/* Wikilinks */
.md-content .wikilink {
  color: var(--accent);
  border-bottom: 1px dashed var(--accent);
}
.md-content .wikilink:hover { border-bottom-style: solid; }

/* Callouts (Obsidian-style) */
.md-content .callout {
  border-left: 4px solid;
  border-radius: 0 8px 8px 0;
  margin: 1em 0;
  padding: 0.8em 1em;
}
.md-content .callout-title {
  font-weight: 600;
  margin-bottom: 0.3em;
  display: flex;
  align-items: center;
  gap: 0.4em;
}
.md-content .callout-info { border-color: #3b82f6; background: var(--callout-info); }
.md-content .callout-warning { border-color: #f59e0b; background: var(--callout-warning); }
.md-content .callout-danger, .md-content .callout-error { border-color: #ef4444; background: var(--callout-danger); }
.md-content .callout-tip, .md-content .callout-hint { border-color: #10b981; background: var(--callout-tip); }
.md-content .callout-note { border-color: #6366f1; background: var(--callout-note); }
.md-content .callout-example { border-color: #a855f7; background: var(--callout-example); }
.md-content .callout-abstract, .md-content .callout-summary { border-color: #06b6d4; background: var(--callout-info); }
.md-content .callout-question, .md-content .callout-faq { border-color: #f59e0b; background: var(--callout-warning); }
.md-content .callout-success, .md-content .callout-check { border-color: #10b981; background: var(--callout-tip); }
.md-content .callout-failure, .md-content .callout-fail { border-color: #ef4444; background: var(--callout-danger); }
.md-content .callout-bug { border-color: #ef4444; background: var(--callout-danger); }
.md-content .callout-quote, .md-content .callout-cite { border-color: var(--text-secondary); background: var(--bg-secondary); }

/* Math via KaTeX */
.katex-display { overflow-x: auto; overflow-y: hidden; padding: 0.5em 0; }

/* Mermaid diagrams */
.mermaid { margin: 1em 0; text-align: center; }

/* Footnotes */
.footnotes { margin-top: 2em; border-top: 1px solid var(--border); padding-top: 1em; font-size: 0.9em; }

/* Directory listing */
.dir-list { list-style: none; padding: 0; }
.dir-list li {
  border-bottom: 1px solid var(--border);
}
.dir-list a {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.6rem 0.4rem;
  color: var(--text);
  transition: background 0.15s;
}
.dir-list a:hover {
  background: var(--bg-secondary);
  text-decoration: none;
}
.dir-list .icon { font-size: 1.2rem; width: 1.5rem; text-align: center; }
.dir-list .name { flex: 1; }
.dir-list .size { color: var(--text-secondary); font-size: 0.85rem; }
.dir-list .date { color: var(--text-secondary); font-size: 0.85rem; white-space: nowrap; }
.dir-list .dir-name { font-weight: 600; }
.sort-controls { display: flex; align-items: center; gap: 0.5rem; margin: 0.8rem 0; font-size: 0.85rem; color: var(--text-secondary); }
.sort-controls a { color: var(--text-secondary); padding: 0.2rem 0.6rem; border-radius: 4px; transition: background 0.15s, color 0.15s; }
.sort-controls a:hover { background: var(--bg-tertiary); color: var(--text); text-decoration: none; }
.sort-controls a.active { background: var(--accent-light); color: var(--accent); font-weight: 600; }

/* Search */
.search-results { list-style: none; padding: 0; }
.search-results li { margin: 1em 0; padding-bottom: 1em; border-bottom: 1px solid var(--border); }
.search-results .result-title { font-weight: 600; font-size: 1.1em; }
.search-results .result-path { font-size: 0.8em; color: var(--text-secondary); }
.search-results .result-snippet { font-size: 0.9em; color: var(--text-secondary); margin-top: 0.3em; }

/* Landing page - vault grid */
.vault-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 1.5rem;
  padding: 2rem 0;
}
.vault-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2.5rem 1.5rem;
  border-radius: 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  transition: transform 0.15s, box-shadow 0.15s, border-color 0.15s;
  text-align: center;
  color: var(--text);
}
.vault-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0,0,0,0.12);
  border-color: var(--accent);
  text-decoration: none;
  color: var(--text);
}
.vault-card .vault-icon {
  font-size: 4rem;
  margin-bottom: 0.75rem;
  line-height: 1;
}
.vault-card .vault-name {
  font-size: 1.2rem;
  font-weight: 600;
}

/* Theme toggle */
.theme-toggle {
  background: none;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.35rem;
  cursor: pointer;
  color: var(--text);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-left: auto;
  flex-shrink: 0;
}
.theme-toggle:hover { background: var(--bg-tertiary); }
.theme-toggle svg { width: 18px; height: 18px; }
[data-theme="light"] .theme-toggle .icon-sun { display: none; }
[data-theme="light"] .theme-toggle .icon-moon { display: inline; }
[data-theme="dark"] .theme-toggle .icon-sun { display: inline; }
[data-theme="dark"] .theme-toggle .icon-moon { display: none; }
`

const themeInitScript = `<script>
(function(){var t=localStorage.getItem('theme');if(!t)t=window.matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light';document.documentElement.setAttribute('data-theme',t)})();
</script>`

const themeToggleBtn = `<button class="theme-toggle" onclick="toggleTheme()" aria-label="Toggle dark mode">
  <svg class="icon-sun" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>
  <svg class="icon-moon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
</button>`

const themeToggleScript = `<script>
function toggleTheme(){var h=document.documentElement;var t=h.getAttribute('data-theme')==='dark'?'light':'dark';h.setAttribute('data-theme',t);localStorage.setItem('theme',t)}
</script>`

var pageTmpl = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.PageTitle}} - {{.SiteTitle}}</title>
<style>` + baseCSS + `</style>
` + themeInitScript + `
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.css">
</head>
<body>
<div class="topbar">
  <a href="/" class="site-title">{{.SiteTitle}}</a>
  <form class="search-form" action="/search" method="get">
    <input type="text" name="q" placeholder="Search files..." autocomplete="off">
  </form>
  ` + themeToggleBtn + `
</div>
<div class="layout">
  <div class="main-content">
    <nav class="breadcrumbs">
      {{range $i, $b := .Breadcrumbs}}{{if $i}}<span class="sep">/</span>{{end}}<a href="{{$b.Path}}">{{$b.Name}}</a>{{end}}
    </nav>
    <article class="md-content">
      {{.Content}}
    </article>
  </div>
  {{if .TOC}}
  <aside class="toc-sidebar">
    <div class="toc-title">On this page</div>
    <ul>
      {{range .TOC}}
      <li class="toc-level-{{.Level}}"><a href="#{{.ID}}">{{.Title}}</a></li>
      {{end}}
    </ul>
  </aside>
  {{end}}
</div>
<script src="https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/contrib/auto-render.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
<script>
document.addEventListener("DOMContentLoaded", function() {
  // Render LaTeX math
  renderMathInElement(document.querySelector('.md-content'), {
    delimiters: [
      {left: '$$', right: '$$', display: true},
      {left: '$', right: '$', display: false},
      {left: '\\[', right: '\\]', display: true},
      {left: '\\(', right: '\\)', display: false}
    ],
    throwOnError: false
  });
  // Render Mermaid diagrams
  var codeBlocks = document.querySelectorAll('pre > code.language-mermaid');
  codeBlocks.forEach(function(block) {
    var pre = block.parentElement;
    var div = document.createElement('div');
    div.className = 'mermaid';
    div.textContent = block.textContent;
    pre.parentNode.replaceChild(div, pre);
  });
  if (document.querySelector('.mermaid')) {
    mermaid.initialize({ startOnLoad: true, theme: document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'default' });
  }
});
</script>
` + themeToggleScript + `
</body>
</html>`))

var dirTmpl = template.Must(template.New("dir").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.DirName}} - {{.SiteTitle}}</title>
<style>` + baseCSS + `</style>
` + themeInitScript + `
</head>
<body>
<div class="topbar">
  <a href="/" class="site-title">{{.SiteTitle}}</a>
  <form class="search-form" action="/search" method="get">
    <input type="text" name="q" placeholder="Search files..." autocomplete="off">
  </form>
  ` + themeToggleBtn + `
</div>
<div class="layout">
  <div class="main-content">
    <nav class="breadcrumbs">
      {{range $i, $b := .Breadcrumbs}}{{if $i}}<span class="sep">/</span>{{end}}<a href="{{$b.Path}}">{{$b.Name}}</a>{{end}}
    </nav>
    <h1>{{.DirName}}</h1>
    <div class="sort-controls">
      <span>Sort by:</span>
      <a href="?sort=name"{{if eq .SortBy "name"}} class="active"{{end}}>Name</a>
      <a href="?sort=date"{{if eq .SortBy "date"}} class="active"{{end}}>Date modified</a>
    </div>
    <ul class="dir-list">
      {{range .Dirs}}
      <li><a href="{{.Path}}"><span class="icon">&#x1F4C1;</span><span class="name dir-name">{{.Name}}</span><span class="date">{{.ModFmt}}</span></a></li>
      {{end}}
      {{range .Files}}
      <li><a href="{{.Path}}"><span class="icon">&#x1F4C4;</span><span class="name">{{.Name}}</span><span class="date">{{.ModFmt}}</span><span class="size">{{.Size}}</span></a></li>
      {{end}}
    </ul>
    {{if .ReadmeHTML}}
    <hr>
    <div class="md-content">
      {{.ReadmeHTML}}
    </div>
    {{end}}
  </div>
</div>
` + themeToggleScript + `
</body>
</html>`))

var searchTmpl = template.Must(template.New("search").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Search: {{.Query}} - {{.SiteTitle}}</title>
<style>` + baseCSS + `</style>
` + themeInitScript + `
</head>
<body>
<div class="topbar">
  <a href="/" class="site-title">{{.SiteTitle}}</a>
  <form class="search-form" action="/search" method="get">
    <input type="text" name="q" value="{{.Query}}" placeholder="Search files..." autocomplete="off">
  </form>
  ` + themeToggleBtn + `
</div>
<div class="layout">
  <div class="main-content">
    <h1>Search results for "{{.Query}}"</h1>
    {{if .Results}}
    <ul class="search-results">
      {{range .Results}}
      <li>
        <a href="{{.Path}}" class="result-title">{{.Name}}</a>
        <div class="result-path">{{.Path}}</div>
        <div class="result-snippet">...{{.Snippet}}...</div>
      </li>
      {{end}}
    </ul>
    {{else}}
    <p>No results found.</p>
    {{end}}
  </div>
</div>
` + themeToggleScript + `
</body>
</html>`))

var imageViewerTmpl = template.Must(template.New("imageviewer").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.FileName}} - {{.SiteTitle}}</title>
<style>` + baseCSS + `
</style>
` + themeInitScript + `
<style>
.image-viewer {
  text-align: center;
  padding: 1rem 0;
}
.image-viewer img {
  max-width: 100%;
  height: auto;
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.10);
}
.image-viewer .image-name {
  font-size: 1.3em;
  font-weight: 600;
  margin-bottom: 0.8rem;
}
.image-viewer .image-actions {
  margin-top: 1rem;
  font-size: 0.9em;
}
.image-viewer .image-actions a {
  color: var(--link);
  margin: 0 0.5rem;
}
</style>
</head>
<body>
<div class="topbar">
  <a href="/" class="site-title">{{.SiteTitle}}</a>
  <form class="search-form" action="/search" method="get">
    <input type="text" name="q" placeholder="Search files..." autocomplete="off">
  </form>
  ` + themeToggleBtn + `
</div>
<div class="layout">
  <div class="main-content">
    <nav class="breadcrumbs">
      {{range $i, $b := .Breadcrumbs}}{{if $i}}<span class="sep">/</span>{{end}}<a href="{{$b.Path}}">{{$b.Name}}</a>{{end}}
    </nav>
    <div class="image-viewer">
      <div class="image-name">{{.FileName}}</div>
      <img src="{{.ImageURL}}" alt="{{.FileName}}">
      <div class="image-actions">
        <a href="{{.ImageURL}}" download>Download</a>
        <a href="{{.ImageURL}}" target="_blank">Open raw</a>
      </div>
    </div>
  </div>
</div>
` + themeToggleScript + `
</body>
</html>`))

var excalidrawViewerTmpl = template.Must(template.New("excalidrawviewer").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.FileName}} - {{.SiteTitle}}</title>
<style>` + baseCSS + `
</style>
` + themeInitScript + `
<style>
.excalidraw-viewer {
  padding: 1rem 0;
}
.excalidraw-viewer .file-name {
  font-size: 1.3em;
  font-weight: 600;
  margin-bottom: 0.8rem;
}
.excalidraw-viewer .excalidraw-container {
  width: 100%;
  height: 70vh;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
.excalidraw-viewer .excalidraw-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-secondary);
  font-size: 0.95em;
}
</style>
</head>
<body>
<div class="topbar">
  <a href="/" class="site-title">{{.SiteTitle}}</a>
  <form class="search-form" action="/search" method="get">
    <input type="text" name="q" placeholder="Search files..." autocomplete="off">
  </form>
  ` + themeToggleBtn + `
</div>
<div class="layout">
  <div class="main-content">
    <nav class="breadcrumbs">
      {{range $i, $b := .Breadcrumbs}}{{if $i}}<span class="sep">/</span>{{end}}<a href="{{$b.Path}}">{{$b.Name}}</a>{{end}}
    </nav>
    <div class="excalidraw-viewer">
      <div class="file-name">{{.FileName}}</div>
      <div class="excalidraw-container" id="excalidraw-container">
        <div class="excalidraw-loading">Loading Excalidraw...</div>
      </div>
    </div>
  </div>
</div>
<script src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
<script src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"></script>
<script src="https://unpkg.com/@excalidraw/excalidraw/dist/excalidraw.production.min.js"></script>
<script>
(function() {
  var data = {{.ExcalidrawJSON}};
  var container = document.getElementById("excalidraw-container");
  container.innerHTML = "";
  var isDark = document.documentElement.getAttribute("data-theme") === "dark";
  var root = ReactDOM.createRoot(container);
  root.render(
    React.createElement(ExcalidrawLib.Excalidraw, {
      initialData: {
        elements: data.elements || [],
        appState: Object.assign({}, data.appState || {}, {
          viewBackgroundColor: isDark ? "#1a1b26" : (data.appState && data.appState.viewBackgroundColor) || "#ffffff",
          theme: isDark ? "dark" : "light"
        }),
        files: data.files || {}
      },
      viewModeEnabled: true,
      zenModeEnabled: true,
      gridModeEnabled: false,
      theme: isDark ? "dark" : "light"
    })
  );
})();
</script>
` + themeToggleScript + `
</body>
</html>`))

var landingTmpl = template.Must(template.New("landing").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.SiteTitle}}</title>
<style>` + baseCSS + `</style>
` + themeInitScript + `
</head>
<body>
<div class="topbar">
  <a href="/" class="site-title">{{.SiteTitle}}</a>
  <form class="search-form" action="/search" method="get">
    <input type="text" name="q" placeholder="Search all vaults..." autocomplete="off">
  </form>
  ` + themeToggleBtn + `
</div>
<div class="layout">
  <div class="main-content">
    <h1>Vaults</h1>
    <div class="vault-grid">
      {{range .Vaults}}
      <a class="vault-card" href="/{{.Name}}">
        <div class="vault-icon">&#x1F4DA;</div>
        <div class="vault-name">{{.Name}}</div>
      </a>
      {{end}}
    </div>
  </div>
</div>
` + themeToggleScript + `
</body>
</html>`))
