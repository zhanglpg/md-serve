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

	vaults := make([]server.Vault, 0, len(dirs))
	for _, d := range dirs {
		name, path := parseDir(d)
		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving directory %q: %v\n", path, err)
			os.Exit(1)
		}
		info, err := os.Stat(absPath)
		if err != nil || !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: %s is not a valid directory\n", absPath)
			os.Exit(1)
		}
		if name == "" {
			name = filepath.Base(absPath)
		}
		vaults = append(vaults, server.Vault{Name: name, Path: absPath})
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

// parseDir splits "name=/path" into (name, path). Plain paths return ("", path).
func parseDir(s string) (string, string) {
	if i := strings.Index(s, "="); i > 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}
