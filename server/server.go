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
	"time"

	"github.com/zhanglpg/md-serve/render"
)

// Vault maps a name to a local directory path.
type Vault struct {
	Name string
	Path string
}

// Server serves markdown files from one or more vaults over HTTP.
type Server struct {
	vaults    []Vault
	vaultMap  map[string]string // name -> path
	siteTitle string
	mux       *http.ServeMux
}

// New creates a new Server with the given vaults.
func New(vaults []Vault, siteTitle string) *Server {
	vm := make(map[string]string, len(vaults))
	for _, v := range vaults {
		vm[v.Name] = v.Path
	}
	s := &Server{
		vaults:    vaults,
		vaultMap:  vm,
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

// singleVaultMode returns true when there is exactly one vault.
func (s *Server) singleVaultMode() bool {
	return len(s.vaults) == 1
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	reqPath := filepath.Clean(r.URL.Path)

	// In single-vault mode, behave exactly like the old code (no prefix needed).
	if s.singleVaultMode() {
		s.handleVaultRequest(w, r, s.vaults[0].Name, s.vaults[0].Path, reqPath)
		return
	}

	// Multi-vault: root shows landing page.
	if reqPath == "/" || reqPath == "." {
		s.serveLanding(w, r)
		return
	}

	// Extract vault name from the first path component.
	parts := strings.SplitN(strings.TrimPrefix(reqPath, "/"), "/", 2)
	vaultName := parts[0]
	vaultPath, ok := s.vaultMap[vaultName]
	if !ok {
		http.NotFound(w, r)
		return
	}

	innerPath := "/"
	if len(parts) > 1 {
		innerPath = "/" + parts[1]
	}
	s.handleVaultRequest(w, r, vaultName, vaultPath, innerPath)
}

func (s *Server) handleVaultRequest(w http.ResponseWriter, r *http.Request, vaultName, rootDir, reqPath string) {
	fullPath := filepath.Join(rootDir, filepath.Clean(reqPath))

	// Security: ensure path is within root
	if !strings.HasPrefix(fullPath, rootDir) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	info, err := s.statOrWait(r, fullPath)
	if err == errFileWaitTimeout {
		w.Header().Set("Retry-After", "5")
		http.Error(w, "File is not yet available, please retry", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		s.resolveNotFound(w, r, vaultName, rootDir, fullPath, reqPath)
		return
	}

	if info.IsDir() {
		s.serveDirectory(w, r, vaultName, rootDir, fullPath, reqPath)
		return
	}

	s.serveFile(w, r, vaultName, rootDir, fullPath, reqPath)
}

// statOrWait stats the file, waiting for cloud-synced resources if needed.
func (s *Server) statOrWait(r *http.Request, fullPath string) (os.FileInfo, error) {
	info, err := os.Stat(fullPath)
	if err == nil {
		return info, nil
	}
	ext := strings.ToLower(filepath.Ext(fullPath))
	if ext != "" && ext != ".md" && ext != ".markdown" && isResourceRequest(r) {
		waitInfo, waitErr := waitForFile(r.Context(), fullPath, fileWaitTimeout)
		if waitErr == nil {
			return waitInfo, nil
		}
		if waitErr == errFileWaitTimeout {
			return nil, waitErr
		}
	}
	return nil, err
}

// resolveNotFound handles requests for files that don't exist, trying .md
// extension and wiki link resolution before returning 404.
func (s *Server) resolveNotFound(w http.ResponseWriter, r *http.Request, vaultName, rootDir, fullPath, reqPath string) {
	// Try appending .md extension
	mdPath := fullPath + ".md"
	if _, err := os.Stat(mdPath); err == nil {
		s.serveMarkdown(w, r, vaultName, rootDir, mdPath, reqPath+".md")
		return
	}
	// Obsidian-style wiki link resolution — only redirect if the file
	// resolves to a different location.
	cleanReq := strings.TrimPrefix(filepath.Clean(reqPath), "/")
	if resolved := render.ResolveWikiTarget(rootDir, filepath.Base(fullPath)); resolved != "" && resolved != cleanReq {
		http.Redirect(w, r, s.vaultPrefix(vaultName)+"/"+render.URLEncodePath(resolved), http.StatusFound)
		return
	}
	if resolved := render.ResolveWikiTarget(rootDir, filepath.Base(mdPath)); resolved != "" && resolved != cleanReq+".md" {
		http.Redirect(w, r, s.vaultPrefix(vaultName)+"/"+render.URLEncodePath(resolved), http.StatusFound)
		return
	}
	http.NotFound(w, r)
}

// serveFile dispatches an existing file to the appropriate handler based on type.
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, vaultName, rootDir, fullPath, reqPath string) {
	ext := strings.ToLower(filepath.Ext(fullPath))
	if ext == ".md" || ext == ".markdown" {
		s.serveMarkdown(w, r, vaultName, rootDir, fullPath, reqPath)
		return
	}

	if render.ImageExts[ext] && isNavigationRequest(r) {
		s.serveImageViewer(w, r, vaultName, fullPath, reqPath)
		return
	}

	if ext == ".excalidraw" {
		if isNavigationRequest(r) {
			s.serveExcalidrawViewer(w, r, vaultName, fullPath, reqPath)
			return
		}
		s.serveExcalidrawShadow(w, r, fullPath)
		return
	}

	serveFileContent(w, r, fullPath)
}

// vaultPrefix returns the URL prefix for a vault. Empty in single-vault mode.
func (s *Server) vaultPrefix(vaultName string) string {
	if s.singleVaultMode() {
		return ""
	}
	return "/" + vaultName
}

func (s *Server) serveMarkdown(w http.ResponseWriter, r *http.Request, vaultName, rootDir, filePath, reqPath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	result, err := render.Markdown(data, &render.RenderOptions{
		VaultDir:  rootDir,
		URLPrefix: s.vaultPrefix(vaultName),
	})
	if err != nil {
		http.Error(w, "Error rendering markdown", http.StatusInternalServerError)
		return
	}

	base := filepath.Base(filePath)
	pageTitle := strings.TrimSuffix(base, filepath.Ext(base))
	pageTitle = strings.ReplaceAll(pageTitle, "-", " ")

	breadcrumbs := s.buildBreadcrumbs(vaultName, reqPath)

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
	Name    string
	Path    string
	IsDir   bool
	Size    string
	ModTime int64  // Unix timestamp for sorting
	ModFmt  string // Human-readable modification time
}

func (s *Server) serveDirectory(w http.ResponseWriter, r *http.Request, vaultName, rootDir, dirPath, reqPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}

	readmeHTML, skipFile := s.renderDirReadme(dirPath, rootDir, vaultName)
	dirs, files := s.buildDirEntries(entries, skipFile, vaultName, reqPath)
	sortBy := sortDirEntries(r, dirs, files)

	dirName := filepath.Base(reqPath)
	if reqPath == "/" || reqPath == "." {
		dirName = vaultName
	}

	breadcrumbs := s.buildBreadcrumbs(vaultName, reqPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = dirTmpl.Execute(w, dirData{
		SiteTitle:   s.siteTitle,
		DirName:     dirName,
		Dirs:        dirs,
		Files:       files,
		Breadcrumbs: breadcrumbs,
		SortBy:      sortBy,
		ReadmeHTML:  readmeHTML,
	})
	if err != nil {
		log.Printf("Template error: %v", err)
	}
}

// renderDirReadme renders index.md or README.md if present, returning the HTML
// and the filename to skip in directory listings.
func (s *Server) renderDirReadme(dirPath, rootDir, vaultName string) (template.HTML, string) {
	for _, name := range []string{"index.md", "README.md"} {
		mdPath := filepath.Join(dirPath, name)
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		result, err := render.Markdown(data, &render.RenderOptions{
			VaultDir:  rootDir,
			URLPrefix: s.vaultPrefix(vaultName),
		})
		if err == nil {
			return template.HTML(result.HTML), name
		}
		break
	}
	return "", ""
}

// buildDirEntries partitions directory entries into dirs and files, skipping
// hidden files and the specified skipFile.
func (s *Server) buildDirEntries(entries []os.DirEntry, skipFile, vaultName, reqPath string) ([]DirEntry, []DirEntry) {
	var dirs, files []DirEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || e.Name() == skipFile {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		entry := DirEntry{
			Name:    e.Name(),
			Path:    filepath.Join(s.vaultPrefix(vaultName), reqPath, e.Name()),
			IsDir:   e.IsDir(),
			Size:    formatSize(info.Size()),
			ModTime: info.ModTime().Unix(),
			ModFmt:  info.ModTime().Format(time.DateTime),
		}
		if e.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}
	return dirs, files
}

// sortDirEntries sorts dirs and files by the sort query parameter (name or date).
func sortDirEntries(r *http.Request, dirs, files []DirEntry) string {
	sortBy := r.URL.Query().Get("sort")
	if sortBy != "date" {
		sortBy = "name"
	}
	if sortBy == "date" {
		sort.Slice(dirs, func(i, j int) bool { return dirs[i].ModTime > dirs[j].ModTime })
		sort.Slice(files, func(i, j int) bool { return files[i].ModTime > files[j].ModTime })
	} else {
		sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
		sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	}
	return sortBy
}

// Breadcrumb represents a navigation breadcrumb.
type Breadcrumb struct {
	Name string
	Path string
}

func (s *Server) buildBreadcrumbs(vaultName, reqPath string) []Breadcrumb {
	var crumbs []Breadcrumb
	crumbs = append(crumbs, Breadcrumb{Name: "Home", Path: "/"})

	if !s.singleVaultMode() {
		crumbs = append(crumbs, Breadcrumb{Name: vaultName, Path: "/" + vaultName})
	}

	parts := strings.Split(strings.Trim(reqPath, "/"), "/")
	if parts[0] == "" || parts[0] == "." {
		return crumbs
	}
	for i, part := range parts {
		path := s.vaultPrefix(vaultName) + "/" + strings.Join(parts[:i+1], "/")
		name := strings.TrimSuffix(part, filepath.Ext(part))
		name = strings.ReplaceAll(name, "-", " ")
		crumbs = append(crumbs, Breadcrumb{Name: name, Path: path})
	}
	return crumbs
}

func (s *Server) serveLanding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := landingTmpl.Execute(w, landingData{
		SiteTitle: s.siteTitle,
		Vaults:    s.vaults,
	})
	if err != nil {
		log.Printf("Template error: %v", err)
	}
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	var results []searchResult

	// Determine which vaults to search
	vaultsToSearch := s.vaults
	if vaultParam := r.URL.Query().Get("vault"); vaultParam != "" {
		if path, ok := s.vaultMap[vaultParam]; ok {
			vaultsToSearch = []Vault{{Name: vaultParam, Path: path}}
		}
	}

	for _, vault := range vaultsToSearch {
		results = s.searchVault(vault, query, results)
	}

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

// searchVault walks a vault directory and appends matching results.
func (s *Server) searchVault(vault Vault, query string, results []searchResult) []searchResult {
	filepath.WalkDir(vault.Path, func(path string, d fs.DirEntry, err error) error {
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

		relPath, _ := filepath.Rel(vault.Path, path)
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		name = strings.ReplaceAll(name, "-", " ")

		results = append(results, searchResult{
			Name:    name,
			Path:    s.vaultPrefix(vault.Name) + "/" + relPath,
			Snippet: extractSnippet(content, query),
		})
		return nil
	})
	return results
}

// extractSnippet returns a text snippet around the first occurrence of query.
func extractSnippet(content, query string) string {
	idx := strings.Index(strings.ToLower(content), query)
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 80
	if end > len(content) {
		end = len(content)
	}
	return strings.ReplaceAll(content[start:end], "\n", " ")
}

type searchResult struct {
	Name    string
	Path    string
	Snippet string
}

// isResourceRequest returns true when the request originates from a resource
// load (e.g., <img> tag, CSS background, fetch API) rather than direct
// navigation. Used to decide whether to wait for cloud-synced files.
func isResourceRequest(r *http.Request) bool {
	dest := r.Header.Get("Sec-Fetch-Dest")
	return dest == "image" || dest == "style" || dest == "script" || dest == "font" || dest == "audio" || dest == "video"
}

// isNavigationRequest returns true when the request originates from direct
// browser navigation (e.g., clicking a link or typing a URL) rather than
// from an <img> tag or programmatic fetch.
func isNavigationRequest(r *http.Request) bool {
	// Explicit raw override
	if r.URL.Query().Get("raw") == "1" {
		return false
	}
	dest := r.Header.Get("Sec-Fetch-Dest")
	if dest == "image" {
		return false
	}
	if dest == "document" {
		return true
	}
	// Fallback for older browsers: navigation requests include text/html
	// in Accept while <img> requests do not.
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}

func (s *Server) serveImageViewer(w http.ResponseWriter, r *http.Request, vaultName, fullPath, reqPath string) {
	fileName := filepath.Base(fullPath)
	rawURL := r.URL.Path + "?raw=1"

	breadcrumbs := s.buildBreadcrumbs(vaultName, reqPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := imageViewerTmpl.Execute(w, imageViewerData{
		SiteTitle:   s.siteTitle,
		FileName:    fileName,
		ImageURL:    rawURL,
		Breadcrumbs: breadcrumbs,
	})
	if err != nil {
		log.Printf("Template error: %v", err)
	}
}

func (s *Server) serveExcalidrawViewer(w http.ResponseWriter, r *http.Request, vaultName, fullPath, reqPath string) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	fileName := filepath.Base(fullPath)
	breadcrumbs := s.buildBreadcrumbs(vaultName, reqPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = excalidrawViewerTmpl.Execute(w, excalidrawViewerData{
		SiteTitle:      s.siteTitle,
		FileName:       fileName,
		ExcalidrawJSON: string(data),
		Breadcrumbs:    breadcrumbs,
	})
	if err != nil {
		log.Printf("Template error: %v", err)
	}
}

// serveExcalidrawShadow serves the shadow SVG or PNG file that Obsidian
// exports alongside an .excalidraw file. It checks for .excalidraw.svg first,
// then .excalidraw.png. If no shadow exists it falls back to serving the raw
// excalidraw JSON.
func (s *Server) serveExcalidrawShadow(w http.ResponseWriter, r *http.Request, excalidrawPath string) {
	for _, ext := range []string{".svg", ".png"} {
		candidate := excalidrawPath + ext
		if _, err := os.Stat(candidate); err == nil {
			serveFileContent(w, r, candidate)
			return
		}
	}
	// No shadow found — serve raw excalidraw JSON as fallback.
	serveFileContent(w, r, excalidrawPath)
}

// serveFileContent serves a file using http.ServeContent, bypassing
// http.ServeFile's URL path checks that can interfere with vault paths.
func serveFileContent(w http.ResponseWriter, r *http.Request, filePath string) {
	f, err := os.Open(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
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
