package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestICloudPlaceholderPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/vault/photo.png", "/vault/.photo.png.icloud"},
		{"/vault/sub/image.jpg", "/vault/sub/.image.jpg.icloud"},
		{"/vault/My File.png", "/vault/.My File.png.icloud"},
	}
	for _, tc := range tests {
		got := icloudPlaceholderPath(tc.input)
		if got != tc.expected {
			t.Errorf("icloudPlaceholderPath(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestHasICloudPlaceholder_NoPlaceholder(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "photo.png")
	if hasICloudPlaceholder(fullPath) {
		t.Error("expected false when no placeholder exists")
	}
}

func TestHasICloudPlaceholder_WithPlaceholder(t *testing.T) {
	dir := t.TempDir()
	// Create the iCloud placeholder
	placeholder := filepath.Join(dir, ".photo.png.icloud")
	os.WriteFile(placeholder, []byte("icloud placeholder"), 0644)

	fullPath := filepath.Join(dir, "photo.png")
	if !hasICloudPlaceholder(fullPath) {
		t.Error("expected true when placeholder exists")
	}
}

func TestWaitForICloudFile_NoPlaceholder(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "photo.png")

	// Should return error immediately without waiting
	start := time.Now()
	_, err := waitForICloudFile(context.Background(), fullPath, 5*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error when no placeholder exists")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected immediate return, took %v", elapsed)
	}
}

func TestWaitForICloudFile_FileAppears(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "photo.png")

	// Create the iCloud placeholder
	os.WriteFile(filepath.Join(dir, ".photo.png.icloud"), []byte("placeholder"), 0644)

	// Create the real file after a short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.WriteFile(fullPath, []byte("real image data"), 0644)
	}()

	info, err := waitForICloudFile(context.Background(), fullPath, 5*time.Second)
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

func TestWaitForICloudFile_Timeout(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "photo.png")

	// Create the iCloud placeholder but never create the real file
	os.WriteFile(filepath.Join(dir, ".photo.png.icloud"), []byte("placeholder"), 0644)

	start := time.Now()
	_, err := waitForICloudFile(context.Background(), fullPath, 1*time.Second)
	elapsed := time.Since(start)

	if err != errICloudTimeout {
		t.Errorf("expected errICloudTimeout, got %v", err)
	}
	if elapsed < 1*time.Second {
		t.Errorf("expected at least 1s wait, got %v", elapsed)
	}
}

func TestWaitForICloudFile_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "photo.png")

	// Create the iCloud placeholder
	os.WriteFile(filepath.Join(dir, ".photo.png.icloud"), []byte("placeholder"), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := waitForICloudFile(ctx, fullPath, 30*time.Second)
	elapsed := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
	if elapsed > 1*time.Second {
		t.Errorf("expected prompt return after context cancel, took %v", elapsed)
	}
}

func TestHandleVaultRequest_ICloudImage(t *testing.T) {
	dir := t.TempDir()

	// Create the iCloud placeholder for photo.png
	os.WriteFile(filepath.Join(dir, ".photo.png.icloud"), []byte("placeholder"), 0644)

	// Create the real file after a short delay
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.WriteFile(filepath.Join(dir, "photo.png"), []byte("real png data"), 0644)
	}()

	srv := newSingleVault(dir, "Test")

	// Request from <img> tag
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

func TestHandleVaultRequest_ICloudTimeout503(t *testing.T) {
	dir := t.TempDir()

	// Create placeholder but never create the real file
	os.WriteFile(filepath.Join(dir, ".photo.png.icloud"), []byte("placeholder"), 0644)

	// Reduce timeout for test speed
	orig := icloudWaitTimeout
	icloudWaitTimeout = 2 * time.Second
	defer func() { icloudWaitTimeout = orig }()

	srv := newSingleVault(dir, "Test")

	// Request from <img> tag
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

func TestHandleVaultRequest_NoPlaceholder_Still404(t *testing.T) {
	dir := t.TempDir()
	// No placeholder, no real file — should be regular 404
	srv := newSingleVault(dir, "Test")

	req := httptest.NewRequest("GET", "/missing.png", nil)
	req.Header.Set("Sec-Fetch-Dest", "image")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleVaultRequest_ICloudMarkdownNotWaited(t *testing.T) {
	dir := t.TempDir()

	// Create placeholder for a markdown file
	os.WriteFile(filepath.Join(dir, ".notes.md.icloud"), []byte("placeholder"), 0644)

	srv := newSingleVault(dir, "Test")

	// Request for markdown file should NOT wait for iCloud
	start := time.Now()
	req := httptest.NewRequest("GET", "/notes.md", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	elapsed := time.Since(start)

	// Should return quickly (404), not wait 30s
	if elapsed > 2*time.Second {
		t.Errorf("expected immediate response for markdown file, took %v", elapsed)
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleVaultRequest_ICloudNavigationRequest(t *testing.T) {
	dir := t.TempDir()

	// Create placeholder
	os.WriteFile(filepath.Join(dir, ".photo.png.icloud"), []byte("placeholder"), 0644)

	// Create the real file after a short delay
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.WriteFile(filepath.Join(dir, "photo.png"), []byte("real png data"), 0644)
	}()

	srv := newSingleVault(dir, "Test")

	// Direct browser navigation should show viewer page after iCloud wait
	req := httptest.NewRequest("GET", "/photo.png", nil)
	req.Header.Set("Sec-Fetch-Dest", "document")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "photo.png") {
		t.Error("expected viewer page to contain filename")
	}
}
