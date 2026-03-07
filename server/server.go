package server

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zhanglpg/md-serve/render"
)

// Server serves markdown files over HTTP.
type Server struct {
	rootDir   string
	siteTitle string
	mux       *http.ServeMux
}

// New creates a new Server.
func New(rootDir, siteTitle string) *Server {
	s := &Server{
		rootDir:   rootDir,
		siteTitle: siteTitle,
		mux:       http.NewServeMux(),
	}
	s.mux.HandleFunc("/", s.handleRequest)
	s.mux.HandleFunc("/search", s.handleSearch)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Resolve the requested path
	reqPath := filepath.Clean(r.URL.Path)
	fullPath := filepath.Join(s.rootDir, reqPath)

	// Security: ensure path is within root
	if !strings.HasPrefix(fullPath, s.rootDir) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		// Try appending .md extension
		mdPath := fullPath + ".md"
		if _, err2 := os.Stat(mdPath); err2 == nil {
			s.serveMarkdown(w, r, mdPath, reqPath+".md")
			return
		}
		// Obsidian-style wiki link resolution: search entire root for matching filename
		if resolved := s.resolveWikiLink(filepath.Base(fullPath)); resolved != "" {
			http.Redirect(w, r, "/"+resolved, http.StatusFound)
			return
		}
		if resolved := s.resolveWikiLink(filepath.Base(mdPath)); resolved != "" {
			http.Redirect(w, r, "/"+resolved, http.StatusFound)
			return
		}
		http.NotFound(w, r)
		return
	}

	if info.IsDir() {
		// Check for index.md in directory
		indexPath := filepath.Join(fullPath, "index.md")
		if _, err := os.Stat(indexPath); err == nil {
			s.serveMarkdown(w, r, indexPath, filepath.Join(reqPath, "index.md"))
			return
		}
		// Check for README.md
		readmePath := filepath.Join(fullPath, "README.md")
		if _, err := os.Stat(readmePath); err == nil {
			s.serveMarkdown(w, r, readmePath, filepath.Join(reqPath, "README.md"))
			return
		}
		s.serveDirectory(w, r, fullPath, reqPath)
		return
	}

	ext := strings.ToLower(filepath.Ext(fullPath))
	if ext == ".md" || ext == ".markdown" {
		s.serveMarkdown(w, r, fullPath, reqPath)
		return
	}

	// Serve other files as-is (images, etc.)
	http.ServeFile(w, r, fullPath)
}

func (s *Server) serveMarkdown(w http.ResponseWriter, r *http.Request, filePath, reqPath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	result, err := render.Markdown(data)
	if err != nil {
		http.Error(w, "Error rendering markdown", http.StatusInternalServerError)
		return
	}

	// Derive page title from filename
	base := filepath.Base(filePath)
	pageTitle := strings.TrimSuffix(base, filepath.Ext(base))
	pageTitle = strings.ReplaceAll(pageTitle, "-", " ")

	// Build breadcrumbs
	breadcrumbs := buildBreadcrumbs(reqPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = pageTmpl.Execute(w, pageData{
		SiteTitle:   s.siteTitle,
		PageTitle:   pageTitle,
		Content:     template.HTML(result.HTML),
		TOC:         result.TOC,
		Breadcrumbs: breadcrumbs,
	})
	if err != nil {
		log.Printf("Template error: %v", err)
	}
}

// DirEntry holds info about a file or directory for listing.
type DirEntry struct {
	Name  string
	Path  string
	IsDir bool
	Size  string
}

func (s *Server) serveDirectory(w http.ResponseWriter, r *http.Request, dirPath, reqPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}

	var dirs, files []DirEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // skip hidden files
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		entry := DirEntry{
			Name:  e.Name(),
			Path:  filepath.Join(reqPath, e.Name()),
			IsDir: e.IsDir(),
			Size:  formatSize(info.Size()),
		}
		if e.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	dirName := filepath.Base(reqPath)
	if reqPath == "/" || reqPath == "." {
		dirName = s.siteTitle
	}

	breadcrumbs := buildBreadcrumbs(reqPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = dirTmpl.Execute(w, dirData{
		SiteTitle:   s.siteTitle,
		DirName:     dirName,
		Dirs:        dirs,
		Files:       files,
		Breadcrumbs: breadcrumbs,
	})
	if err != nil {
		log.Printf("Template error: %v", err)
	}
}

// Breadcrumb represents a navigation breadcrumb.
type Breadcrumb struct {
	Name string
	Path string
}

func buildBreadcrumbs(reqPath string) []Breadcrumb {
	parts := strings.Split(strings.Trim(reqPath, "/"), "/")
	var crumbs []Breadcrumb
	crumbs = append(crumbs, Breadcrumb{Name: "Home", Path: "/"})
	if parts[0] == "" || parts[0] == "." {
		return crumbs
	}
	for i, part := range parts {
		path := "/" + strings.Join(parts[:i+1], "/")
		name := strings.TrimSuffix(part, filepath.Ext(part))
		name = strings.ReplaceAll(name, "-", " ")
		crumbs = append(crumbs, Breadcrumb{Name: name, Path: path})
	}
	return crumbs
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	var results []searchResult
	filepath.WalkDir(s.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if !strings.Contains(strings.ToLower(content), query) {
			return nil
		}

		relPath, _ := filepath.Rel(s.rootDir, path)
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		name = strings.ReplaceAll(name, "-", " ")

		// Extract a snippet around the match
		idx := strings.Index(strings.ToLower(content), query)
		start := idx - 80
		if start < 0 {
			start = 0
		}
		end := idx + len(query) + 80
		if end > len(content) {
			end = len(content)
		}
		snippet := content[start:end]
		snippet = strings.ReplaceAll(snippet, "\n", " ")

		results = append(results, searchResult{
			Name:    name,
			Path:    "/" + relPath,
			Snippet: snippet,
		})
		return nil
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := searchTmpl.Execute(w, searchData{
		SiteTitle: s.siteTitle,
		Query:     query,
		Results:   results,
	})
	if err != nil {
		log.Printf("Template error: %v", err)
	}
}

type searchResult struct {
	Name    string
	Path    string
	Snippet string
}

// resolveWikiLink searches the entire root directory for a markdown file
// matching the given basename (case-insensitive). This supports Obsidian-style
// wiki links where [[Page Name]] can link to any file in the vault regardless
// of its directory location.
func (s *Server) resolveWikiLink(basename string) string {
	target := strings.ToLower(basename)
	var match string
	filepath.WalkDir(s.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Base(path)) == target {
			rel, err := filepath.Rel(s.rootDir, path)
			if err == nil {
				match = rel
				return filepath.SkipAll
			}
		}
		return nil
	})
	return match
}

func formatSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
}
