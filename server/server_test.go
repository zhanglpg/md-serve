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

func TestNew(t *testing.T) {
	srv := New("/tmp", "Test Site")
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.rootDir != "/tmp" {
		t.Errorf("expected rootDir /tmp, got %s", srv.rootDir)
	}
	if srv.siteTitle != "Test Site" {
		t.Errorf("expected siteTitle 'Test Site', got %s", srv.siteTitle)
	}
}

func TestServeMarkdownFile(t *testing.T) {
	dir := setupTestDir(t)
	srv := New(dir, "Test")

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
	srv := New(dir, "Test")

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
	srv := New(dir, "Test")

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
	srv := New(dir, "Test")

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

	srv := New(dir, "Test")

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

	srv := New(dir, "Test")

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
	srv := New(dir, "Test")

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPathTraversal(t *testing.T) {
	dir := setupTestDir(t)
	srv := New(dir, "Test")

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
	srv := New(dir, "Test")

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
	srv := New(dir, "Test")

	req := httptest.NewRequest("GET", "/search?q=", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
}

func TestSearchNoResults(t *testing.T) {
	dir := setupTestDir(t)
	srv := New(dir, "Test")

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

	srv := New(dir, "Test")

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

	srv := New(dir, "Test")

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

	srv := New(dir, "Test")

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
	srv := New(dir, "Test")

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
	srv := New(dir, "Test")

	// Request something that doesn't exist anywhere
	req := httptest.NewRequest("GET", "/totally-missing-page.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Breadcrumb tests ---

func TestBuildBreadcrumbs_Root(t *testing.T) {
	crumbs := buildBreadcrumbs("/")
	if len(crumbs) != 1 {
		t.Fatalf("expected 1 breadcrumb, got %d", len(crumbs))
	}
	if crumbs[0].Name != "Home" {
		t.Errorf("expected 'Home', got %q", crumbs[0].Name)
	}
}

func TestBuildBreadcrumbs_NestedPath(t *testing.T) {
	crumbs := buildBreadcrumbs("/docs/getting-started.md")
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
