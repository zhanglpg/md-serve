package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWaitForFile_FileAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "photo.png")
	os.WriteFile(fullPath, []byte("png data"), 0644)

	info, err := waitForFile(context.Background(), fullPath, 1*time.Second)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil FileInfo")
	}
}

func TestWaitForFile_FileAppears(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "photo.png")

	go func() {
		time.Sleep(500 * time.Millisecond)
		os.WriteFile(fullPath, []byte("real image data"), 0644)
	}()

	info, err := waitForFile(context.Background(), fullPath, 5*time.Second)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil FileInfo")
	}
	if info.Name() != "photo.png" {
		t.Errorf("expected name 'photo.png', got %q", info.Name())
	}
}

func TestWaitForFile_Timeout(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "photo.png")

	start := time.Now()
	_, err := waitForFile(context.Background(), fullPath, 1*time.Second)
	elapsed := time.Since(start)

	if err != errFileWaitTimeout {
		t.Errorf("expected errFileWaitTimeout, got %v", err)
	}
	if elapsed < 1*time.Second {
		t.Errorf("expected at least 1s wait, got %v", elapsed)
	}
}

func TestWaitForFile_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "photo.png")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := waitForFile(ctx, fullPath, 30*time.Second)
	elapsed := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
	if elapsed > 1*time.Second {
		t.Errorf("expected prompt return after context cancel, took %v", elapsed)
	}
}

func TestHandleVaultRequest_FileAppearsLater(t *testing.T) {
	dir := t.TempDir()

	// Create the real file after a short delay
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.WriteFile(filepath.Join(dir, "photo.png"), []byte("real png data"), 0644)
	}()

	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/photo.png", nil)
	req.Header.Set("Sec-Fetch-Dest", "image")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body != "real png data" {
		t.Errorf("expected real image content, got %q", body)
	}
}

func TestHandleVaultRequest_Timeout503(t *testing.T) {
	dir := t.TempDir()

	// Reduce timeout for test speed
	orig := fileWaitTimeout
	fileWaitTimeout = 2 * time.Second
	defer func() { fileWaitTimeout = orig }()

	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/photo.png", nil)
	req.Header.Set("Sec-Fetch-Dest", "image")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
	retryAfter := w.Header().Get("Retry-After")
	if retryAfter != "5" {
		t.Errorf("expected Retry-After: 5, got %q", retryAfter)
	}
}

func TestHandleVaultRequest_MissingFile_Still404(t *testing.T) {
	dir := t.TempDir()

	// Reduce timeout for test speed
	orig := fileWaitTimeout
	fileWaitTimeout = 1 * time.Second
	defer func() { fileWaitTimeout = orig }()

	srv := newSingleVault(dir, "Test")

	// Markdown files should not trigger wait — immediate 404
	req := httptest.NewRequest("GET", "/missing.md", nil)
	w := httptest.NewRecorder()
	start := time.Now()
	srv.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected immediate response for markdown file, took %v", elapsed)
	}
}

func TestHandleVaultRequest_NavigationDoesNotWait(t *testing.T) {
	dir := t.TempDir()
	srv := newSingleVault(dir, "Test")

	// Direct browser navigation for missing files should NOT wait —
	// should fall through to wiki link resolution or 404 immediately.
	start := time.Now()
	req := httptest.NewRequest("GET", "/photo.png", nil)
	req.Header.Set("Sec-Fetch-Dest", "document")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	if elapsed > 2*time.Second {
		t.Errorf("expected immediate response for navigation request, took %v", elapsed)
	}
}
