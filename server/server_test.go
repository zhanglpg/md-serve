package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to create a temp directory with markdown files for testing.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create a markdown file
	err := os.WriteFile(filepath.Join(dir, "hello.md"), []byte("# Hello\n\nWorld"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create an index.md
	err = os.WriteFile(filepath.Join(dir, "index.md"), []byte("# Home\n\nWelcome"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with README.md
	subDir := filepath.Join(dir, "docs")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(subDir, "README.md"), []byte("# Docs\n\nDocumentation"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create a plain text file
	err = os.WriteFile(filepath.Join(dir, "data.txt"), []byte("plain text"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create a hidden file
	err = os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

// newSingleVault is a helper that creates a single-vault server (backward-compat mode).
func newSingleVault(dir, title string) *Server {
	return New([]Vault{{Name: "test", Path: dir}}, title)
}

func TestNew(t *testing.T) {
	srv := New([]Vault{{Name: "vault1", Path: "/tmp"}}, "Test Site")
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.vaults[0].Path != "/tmp" {
		t.Errorf("expected vault path /tmp, got %s", srv.vaults[0].Path)
	}
	if srv.siteTitle != "Test Site" {
		t.Errorf("expected siteTitle 'Test Site', got %s", srv.siteTitle)
	}
}

func TestServeMarkdownFile(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/hello.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html content type, got %s", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Hello") {
		t.Error("expected rendered markdown content in response")
	}
}

func TestServeMarkdownWithoutExtension(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Hello") {
		t.Error("expected rendered markdown content when .md is omitted")
	}
}

func TestServeDirectoryWithIndex(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Home") {
		t.Error("expected index.md content for root directory")
	}
}

func TestServeDirectoryWithReadme(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Docs") {
		t.Error("expected README.md content for docs directory")
	}
}

func TestServeDirectoryListing(t *testing.T) {
	dir := setupTestDir(t)
	// Create a directory without index or README
	emptyDir := filepath.Join(dir, "empty")
	os.Mkdir(emptyDir, 0755)
	os.WriteFile(filepath.Join(emptyDir, "file1.md"), []byte("# File 1"), 0644)

	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/empty", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "file1.md") {
		t.Error("expected directory listing to contain file1.md")
	}
}

func TestServeDirectoryHidesHiddenFiles(t *testing.T) {
	dir := setupTestDir(t)
	// Remove index.md so we get a directory listing
	os.Remove(filepath.Join(dir, "index.md"))

	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, ".hidden") {
		t.Error("expected hidden files to be excluded from directory listing")
	}
}

func TestNotFound(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPathTraversal(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/../../../etc/passwd", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should either be forbidden or not found - never serve the file
	if w.Code == http.StatusOK {
		body := w.Body.String()
		if strings.Contains(body, "root:") {
			t.Error("path traversal should be blocked")
		}
	}
}

func TestSearchHandler(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/search?q=world", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "hello") {
		t.Error("expected search results to include hello.md")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/search?q=", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
}

func TestSearchNoResults(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/search?q=zzzznonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// --- Wiki link resolution tests ---

func TestWikiLinkResolution_FileInSubdir(t *testing.T) {
	dir := setupTestDir(t)
	// Create a file in a subdirectory that we'll link to without a path
	subDir := filepath.Join(dir, "notes")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "My-Page.md"), []byte("# My Page"), 0644)

	srv := newSingleVault(dir, "Test")

	// Request /My-Page.md which doesn't exist at root - should redirect to /notes/My-Page.md
	req := httptest.NewRequest("GET", "/My-Page.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/notes/My-Page.md" {
		t.Errorf("expected redirect to /notes/My-Page.md, got %q", loc)
	}
}

func TestWikiLinkResolution_WithoutExtension(t *testing.T) {
	dir := setupTestDir(t)
	subDir := filepath.Join(dir, "deep", "nested")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "Target.md"), []byte("# Target"), 0644)

	srv := newSingleVault(dir, "Test")

	// Request /Target (without .md) - should find Target.md in deep/nested/
	req := httptest.NewRequest("GET", "/Target", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/deep/nested/Target.md" {
		t.Errorf("expected redirect to /deep/nested/Target.md, got %q", loc)
	}
}

func TestWikiLinkResolution_CaseInsensitive(t *testing.T) {
	dir := setupTestDir(t)
	subDir := filepath.Join(dir, "wiki")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "my-notes.md"), []byte("# Notes"), 0644)

	srv := newSingleVault(dir, "Test")

	// Request with different casing
	req := httptest.NewRequest("GET", "/My-Notes.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/wiki/my-notes.md" {
		t.Errorf("expected redirect to /wiki/my-notes.md, got %q", loc)
	}
}

func TestWikiLinkResolution_DirectFileStillWorks(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	// Request /hello.md which exists directly - should serve normally, not redirect
	req := httptest.NewRequest("GET", "/hello.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWikiLinkResolution_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	srv := newSingleVault(dir, "Test")

	// Request something that doesn't exist anywhere
	req := httptest.NewRequest("GET", "/totally-missing-page.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestWikiLinkResolution_FileWithSpaces(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "notes")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "My Page.md"), []byte("# My Page"), 0644)

	srv := newSingleVault(dir, "Test")

	// Request with URL-encoded spaces (browser sends %20 for spaces)
	req := httptest.NewRequest("GET", "/My%20Page.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/notes/My Page.md" {
		t.Errorf("expected redirect to /notes/My Page.md, got %q", loc)
	}
}

func TestWikiLinkResolution_FileWithSpecialChars(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "notes")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "Page (draft).md"), []byte("# Draft"), 0644)

	srv := newSingleVault(dir, "Test")

	// Request with URL-encoded special characters
	req := httptest.NewRequest("GET", "/Page%20%28draft%29.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/notes/Page (draft).md" {
		t.Errorf("expected redirect to /notes/Page (draft).md, got %q", loc)
	}
}

func TestWikiLinkResolution_SpaceHyphenInterop(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "notes")
	os.Mkdir(subDir, 0755)
	// File on disk uses spaces
	os.WriteFile(filepath.Join(subDir, "My Page.md"), []byte("# My Page"), 0644)

	srv := newSingleVault(dir, "Test")

	// Request with hyphens should still find the file with spaces
	req := httptest.NewRequest("GET", "/My-Page.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/notes/My Page.md" {
		t.Errorf("expected redirect to /notes/My Page.md, got %q", loc)
	}
}

func TestServeMarkdownFile_WithSpaces(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "My Page.md"), []byte("# My Page\n\nContent"), 0644)

	srv := newSingleVault(dir, "Test")

	// Request file with spaces directly (URL-encoded)
	req := httptest.NewRequest("GET", "/My%20Page.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "My Page") {
		t.Error("expected rendered markdown content with spaces in filename")
	}
}

// --- Attachment serving tests ---

func TestServeAttachment_DirectPath(t *testing.T) {
	dir := t.TempDir()
	// Create a PNG file
	os.WriteFile(filepath.Join(dir, "photo.png"), []byte("fake png data"), 0644)

	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/photo.png", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body != "fake png data" {
		t.Errorf("expected raw file content, got %q", body)
	}
}

func TestServeAttachment_InSubdir(t *testing.T) {
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	os.Mkdir(assetsDir, 0755)
	os.WriteFile(filepath.Join(assetsDir, "photo.png"), []byte("fake png data"), 0644)

	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/assets/photo.png", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWikiLinkResolution_Attachment(t *testing.T) {
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	os.Mkdir(assetsDir, 0755)
	os.WriteFile(filepath.Join(assetsDir, "photo.png"), []byte("fake png data"), 0644)

	srv := newSingleVault(dir, "Test")

	// Request /photo.png which doesn't exist at root - should redirect to /assets/photo.png
	req := httptest.NewRequest("GET", "/photo.png", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/assets/photo.png" {
		t.Errorf("expected redirect to /assets/photo.png, got %q", loc)
	}
}

func TestWikiLinkResolution_Excalidraw(t *testing.T) {
	dir := t.TempDir()
	drawDir := filepath.Join(dir, "drawings")
	os.Mkdir(drawDir, 0755)
	os.WriteFile(filepath.Join(drawDir, "diagram.excalidraw"), []byte("{}"), 0644)

	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/diagram.excalidraw", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/drawings/diagram.excalidraw" {
		t.Errorf("expected redirect to /drawings/diagram.excalidraw, got %q", loc)
	}
}

func TestMarkdownWithAttachmentLinks(t *testing.T) {
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	os.Mkdir(assetsDir, 0755)
	os.WriteFile(filepath.Join(assetsDir, "photo.png"), []byte("fake png"), 0644)

	// Create a markdown file that links to the attachment
	content := "# Notes\n\nSee [[photo.png]] for the image.\n\n![[photo.png]]\n"
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte(content), 0644)

	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/notes.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	// The link should NOT have .md appended
	if strings.Contains(body, "photo.png.md") {
		t.Error("attachment link should not have .md appended")
	}
	// Should contain a proper link to the png
	if !strings.Contains(body, `href="/photo.png"`) {
		t.Error("expected link href to point to /photo.png")
	}
	// Should contain an img tag for the embed
	if !strings.Contains(body, `<img src="/photo.png"`) {
		t.Error("expected img tag for embedded image")
	}
}

// --- Breadcrumb tests ---

func TestBuildBreadcrumbs_Root(t *testing.T) {
	srv := newSingleVault("/tmp", "Test")
	crumbs := srv.buildBreadcrumbs("test", "/")
	if len(crumbs) != 1 {
		t.Fatalf("expected 1 breadcrumb, got %d", len(crumbs))
	}
	if crumbs[0].Name != "Home" {
		t.Errorf("expected 'Home', got %q", crumbs[0].Name)
	}
}

func TestBuildBreadcrumbs_NestedPath(t *testing.T) {
	srv := newSingleVault("/tmp", "Test")
	crumbs := srv.buildBreadcrumbs("test", "/docs/getting-started.md")
	// Single vault: Home / docs / getting started
	if len(crumbs) != 3 {
		t.Fatalf("expected 3 breadcrumbs, got %d", len(crumbs))
	}
	if crumbs[0].Name != "Home" {
		t.Errorf("crumb 0: expected 'Home', got %q", crumbs[0].Name)
	}
	if crumbs[1].Name != "docs" {
		t.Errorf("crumb 1: expected 'docs', got %q", crumbs[1].Name)
	}
	if crumbs[2].Name != "getting started" {
		t.Errorf("crumb 2: expected 'getting started', got %q", crumbs[2].Name)
	}
}

// --- formatSize tests ---

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{2621440, "2.5 MB"},
	}
	for _, tc := range tests {
		got := formatSize(tc.size)
		if got != tc.expected {
			t.Errorf("formatSize(%d) = %q, want %q", tc.size, got, tc.expected)
		}
	}
}

// --- Multi-vault tests ---

func TestMultiVault_LandingPage(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "hello.md"), []byte("# Hello"), 0644)
	os.WriteFile(filepath.Join(dir2, "world.md"), []byte("# World"), 0644)

	srv := New([]Vault{
		{Name: "notes", Path: dir1},
		{Name: "wiki", Path: dir2},
	}, "Test")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "notes") {
		t.Error("expected landing page to contain vault 'notes'")
	}
	if !strings.Contains(body, "wiki") {
		t.Error("expected landing page to contain vault 'wiki'")
	}
	if !strings.Contains(body, "Vaults") {
		t.Error("expected landing page to show 'Vaults' heading")
	}
}

func TestMultiVault_ServeVaultFile(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "hello.md"), []byte("# Hello from Notes"), 0644)
	os.WriteFile(filepath.Join(dir2, "world.md"), []byte("# World from Wiki"), 0644)

	srv := New([]Vault{
		{Name: "notes", Path: dir1},
		{Name: "wiki", Path: dir2},
	}, "Test")

	// Access file in first vault
	req := httptest.NewRequest("GET", "/notes/hello.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Hello from Notes") {
		t.Error("expected content from notes vault")
	}

	// Access file in second vault
	req = httptest.NewRequest("GET", "/wiki/world.md", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body = w.Body.String()
	if !strings.Contains(body, "World from Wiki") {
		t.Error("expected content from wiki vault")
	}
}

func TestMultiVault_UnknownVault(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.md"), []byte("# Hello"), 0644)

	srv := New([]Vault{
		{Name: "notes", Path: dir},
		{Name: "wiki", Path: dir},
	}, "Test")

	req := httptest.NewRequest("GET", "/unknown/hello.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestMultiVault_VaultRootDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.md"), []byte("# Hello"), 0644)
	os.WriteFile(filepath.Join(dir, "world.md"), []byte("# World"), 0644)

	srv := New([]Vault{
		{Name: "notes", Path: dir},
		{Name: "wiki", Path: dir},
	}, "Test")

	// Accessing /notes should show directory listing of the vault root
	req := httptest.NewRequest("GET", "/notes", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "hello.md") {
		t.Error("expected vault root to list hello.md")
	}
}

func TestMultiVault_SearchAcrossVaults(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "hello.md"), []byte("# Hello\n\nunique_search_term"), 0644)
	os.WriteFile(filepath.Join(dir2, "world.md"), []byte("# World\n\nunique_search_term"), 0644)

	srv := New([]Vault{
		{Name: "notes", Path: dir1},
		{Name: "wiki", Path: dir2},
	}, "Test")

	req := httptest.NewRequest("GET", "/search?q=unique_search_term", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "/notes/hello.md") {
		t.Error("expected search results to include notes/hello.md")
	}
	if !strings.Contains(body, "/wiki/world.md") {
		t.Error("expected search results to include wiki/world.md")
	}
}

func TestMultiVault_SearchSingleVault(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "hello.md"), []byte("# Hello\n\nshared_term"), 0644)
	os.WriteFile(filepath.Join(dir2, "world.md"), []byte("# World\n\nshared_term"), 0644)

	srv := New([]Vault{
		{Name: "notes", Path: dir1},
		{Name: "wiki", Path: dir2},
	}, "Test")

	req := httptest.NewRequest("GET", "/search?q=shared_term&vault=notes", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "/notes/hello.md") {
		t.Error("expected search results to include notes/hello.md")
	}
	if strings.Contains(body, "/wiki/world.md") {
		t.Error("expected search results NOT to include wiki/world.md when scoped to notes")
	}
}

func TestMultiVault_Breadcrumbs(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "page.md"), []byte("# Page"), 0644)

	srv := New([]Vault{
		{Name: "notes", Path: dir},
		{Name: "wiki", Path: dir},
	}, "Test")

	crumbs := srv.buildBreadcrumbs("notes", "/sub/page.md")
	// Multi-vault: Home / notes / sub / page
	if len(crumbs) != 4 {
		t.Fatalf("expected 4 breadcrumbs, got %d: %v", len(crumbs), crumbs)
	}
	if crumbs[0].Name != "Home" {
		t.Errorf("crumb 0: expected 'Home', got %q", crumbs[0].Name)
	}
	if crumbs[1].Name != "notes" {
		t.Errorf("crumb 1: expected 'notes', got %q", crumbs[1].Name)
	}
	if crumbs[2].Name != "sub" {
		t.Errorf("crumb 2: expected 'sub', got %q", crumbs[2].Name)
	}
	if crumbs[3].Name != "page" {
		t.Errorf("crumb 3: expected 'page', got %q", crumbs[3].Name)
	}
}
