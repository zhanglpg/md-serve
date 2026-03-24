package server

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"
)

// errFileWaitTimeout is returned when a file does not appear on disk
// within the configured timeout (e.g. still syncing from a cloud provider).
var errFileWaitTimeout = errors.New("file did not appear within timeout")

// fileWaitTimeout is the maximum time to wait for a missing file to appear.
// Exported as a variable so tests can reduce it.
var fileWaitTimeout = 60 * time.Second

// pollIntervals defines the increasing backoff intervals for file polling.
var pollIntervals = []time.Duration{
	200 * time.Millisecond,
	500 * time.Millisecond,
	1 * time.Second,
}

const pollRepeatInterval = 2 * time.Second

// pollDelay returns the delay for the given polling step using increasing backoff.
func pollDelay(step int) time.Duration {
	if step < len(pollIntervals) {
		return pollIntervals[step]
	}
	return pollRepeatInterval
}

// waitForFile polls for a missing file to appear on disk, useful when files
// are synced from cloud providers (iCloud, Dropbox, etc.) and may not be
// immediately available. Returns the FileInfo on success, or
// errFileWaitTimeout if the file does not appear in time.
func waitForFile(ctx context.Context, fullPath string, timeout time.Duration) (os.FileInfo, error) {
	log.Printf("waitfile: waiting for file to appear: %s", filepath.Base(fullPath))
	deadline := time.Now().Add(timeout)
	for step := 0; ; step++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollDelay(step)):
		}
		info, err := os.Stat(fullPath)
		if err == nil {
			log.Printf("waitfile: file appeared: %s", filepath.Base(fullPath))
			return info, nil
		}
		if time.Now().After(deadline) {
			log.Printf("waitfile: timed out waiting for: %s", filepath.Base(fullPath))
			return nil, errFileWaitTimeout
		}
	}
}
