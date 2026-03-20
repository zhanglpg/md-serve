package main

import "testing"

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
