package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhanglpg/md-serve/server"
)

// dirFlags collects multiple -dir flag values.
type dirFlags []string

func (d *dirFlags) String() string { return strings.Join(*d, ", ") }
func (d *dirFlags) Set(value string) error {
	*d = append(*d, value)
	return nil
}

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	title := flag.String("title", "MD Serve", "site title")

	var dirs dirFlags
	flag.Var(&dirs, "dir", "vault directory in name=/path format (repeatable); plain path uses folder name")
	flag.Parse()

	// Also accept positional args as dirs
	for _, arg := range flag.Args() {
		dirs = append(dirs, arg)
	}

	// Default to current directory if nothing specified
	if len(dirs) == 0 {
		dirs = append(dirs, ".")
	}

	vaults, err := resolveVaults(dirs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	srv := server.New(vaults, *title)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Serving %d vault(s) on http://localhost%s", len(vaults), addr)
	for _, v := range vaults {
		log.Printf("  /%s -> %s", v.Name, v.Path)
	}
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// resolveVaults converts dir flag values into validated Vault entries.
func resolveVaults(dirs []string) ([]server.Vault, error) {
	vaults := make([]server.Vault, 0, len(dirs))
	for _, d := range dirs {
		name, path := parseDir(d)
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("error resolving directory %q: %v", path, err)
		}
		info, err := os.Stat(absPath)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("error: %s is not a valid directory", absPath)
		}
		if name == "" {
			name = filepath.Base(absPath)
		}
		vaults = append(vaults, server.Vault{Name: name, Path: absPath})
	}
	return vaults, nil
}

// parseDir splits "name=/path" into (name, path). Plain paths return ("", path).
func parseDir(s string) (string, string) {
	if i := strings.Index(s, "="); i > 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}
