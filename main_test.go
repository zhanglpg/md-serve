package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveVaults(t *testing.T) {
	t.Parallel()

	t.Run("valid current dir", func(t *testing.T) {
		vaults, err := resolveVaults([]string{"."})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vaults) != 1 {
			t.Fatalf("expected 1 vault, got %d", len(vaults))
		}
		cwd, _ := os.Getwd()
		if vaults[0].Path != cwd {
			t.Errorf("path = %q, want %q", vaults[0].Path, cwd)
		}
		if vaults[0].Name != filepath.Base(cwd) {
			t.Errorf("name = %q, want %q", vaults[0].Name, filepath.Base(cwd))
		}
	})

	t.Run("named vault", func(t *testing.T) {
		dir := t.TempDir()
		vaults, err := resolveVaults([]string{"docs=" + dir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vaults[0].Name != "docs" {
			t.Errorf("name = %q, want %q", vaults[0].Name, "docs")
		}
		if vaults[0].Path != dir {
			t.Errorf("path = %q, want %q", vaults[0].Path, dir)
		}
	})

	t.Run("plain path uses basename", func(t *testing.T) {
		dir := t.TempDir()
		vaults, err := resolveVaults([]string{dir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vaults[0].Name != filepath.Base(dir) {
			t.Errorf("name = %q, want %q", vaults[0].Name, filepath.Base(dir))
		}
	})

	t.Run("non-existent directory", func(t *testing.T) {
		_, err := resolveVaults([]string{"/no/such/path/exists"})
		if err == nil {
			t.Fatal("expected error for non-existent dir")
		}
		if !strings.Contains(err.Error(), "not a valid directory") {
			t.Errorf("error = %q, want it to contain 'not a valid directory'", err)
		}
	})

	t.Run("file not directory", func(t *testing.T) {
		f, err := os.CreateTemp("", "test-*")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		defer os.Remove(f.Name())

		_, err = resolveVaults([]string{f.Name()})
		if err == nil {
			t.Fatal("expected error for file path")
		}
		if !strings.Contains(err.Error(), "not a valid directory") {
			t.Errorf("error = %q, want it to contain 'not a valid directory'", err)
		}
	})

	t.Run("multiple vaults", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		vaults, err := resolveVaults([]string{"a=" + dir1, "b=" + dir2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vaults) != 2 {
			t.Fatalf("expected 2 vaults, got %d", len(vaults))
		}
		if vaults[0].Name != "a" || vaults[1].Name != "b" {
			t.Errorf("names = [%q, %q], want [a, b]", vaults[0].Name, vaults[1].Name)
		}
	})
}

func TestDirFlags(t *testing.T) {
	t.Parallel()

	t.Run("String with values", func(t *testing.T) {
		d := dirFlags{"a", "b"}
		if got := d.String(); got != "a, b" {
			t.Errorf("String() = %q, want %q", got, "a, b")
		}
	})

	t.Run("String empty", func(t *testing.T) {
		d := dirFlags{}
		if got := d.String(); got != "" {
			t.Errorf("String() = %q, want %q", got, "")
		}
	})

	t.Run("Set accumulates values", func(t *testing.T) {
		var d dirFlags
		d.Set("x")
		d.Set("y")
		if len(d) != 2 || d[0] != "x" || d[1] != "y" {
			t.Errorf("after Set: got %v, want [x y]", d)
		}
	})
}

func TestParseDir(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		wantName string
		wantPath string
	}{
		{"vault=/path/to/dir", "vault", "/path/to/dir"},
		{"/plain/path", "", "/plain/path"},
		{"name=", "name", ""},
		{"=value", "", "=value"},      // = at position 0, i > 0 is false
		{"", "", ""},                  // empty string
		{".", "", "."},                // current dir
		{"a=b=c", "a", "b=c"},        // multiple = signs, splits on first
		{"my-vault=/home/user", "my-vault", "/home/user"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			gotName, gotPath := parseDir(tc.input)
			if gotName != tc.wantName {
				t.Errorf("parseDir(%q) name = %q, want %q", tc.input, gotName, tc.wantName)
			}
			if gotPath != tc.wantPath {
				t.Errorf("parseDir(%q) path = %q, want %q", tc.input, gotPath, tc.wantPath)
			}
		})
	}
}
