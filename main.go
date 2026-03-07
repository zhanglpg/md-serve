package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/zhanglpg/md-serve/server"
)

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	dir := flag.String("dir", ".", "root directory to serve markdown files from")
	title := flag.String("title", "MD Serve", "site title")
	flag.Parse()

	rootDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(rootDir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a valid directory\n", rootDir)
		os.Exit(1)
	}

	srv := server.New(rootDir, *title)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Serving markdown files from %s on http://localhost%s", rootDir, addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
