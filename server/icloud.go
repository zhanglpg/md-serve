package server

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"
)

// errICloudTimeout is returned when a file is confirmed to be on iCloud
// (placeholder exists) but has not materialized within the timeout.
var errICloudTimeout = errors.New("icloud: file download timed out")

// icloudWaitTimeout is the maximum time to wait for an iCloud file to materialize.
// Exported as a variable so tests can reduce it.
var icloudWaitTimeout = 30 * time.Second

// icloudPlaceholderPath returns the iCloud placeholder path for a given file.
// macOS replaces evicted files with .<basename>.icloud placeholders.
func icloudPlaceholderPath(fullPath string) string {
	dir := filepath.Dir(fullPath)
	base := filepath.Base(fullPath)
	return filepath.Join(dir, "."+base+".icloud")
}

// hasICloudPlaceholder checks whether an iCloud placeholder file exists
// for the given path, indicating the file is evicted to iCloud.
func hasICloudPlaceholder(fullPath string) bool {
	_, err := os.Stat(icloudPlaceholderPath(fullPath))
	return err == nil
}

// waitForICloudFile waits for an iCloud-evicted file to materialize on disk.
// It triggers a download by accessing the original path, then polls until the
// file appears or the timeout is reached. Returns the FileInfo on success,
// or errICloudTimeout if the file does not appear in time.
// If no iCloud placeholder exists, returns the original stat error immediately.
func waitForICloudFile(ctx context.Context, fullPath string, timeout time.Duration) (os.FileInfo, error) {
	if !hasICloudPlaceholder(fullPath) {
		_, err := os.Stat(fullPath)
		return nil, err
	}

	log.Printf("icloud: waiting for file to materialize: %s", filepath.Base(fullPath))

	// Attempt to open the placeholder to trigger the iCloud daemon download.
	// On macOS, accessing the .icloud placeholder signals the daemon to
	// start downloading the original file.
	placeholderPath := icloudPlaceholderPath(fullPath)
	if f, err := os.Open(placeholderPath); err == nil {
		f.Close()
	}

	// Poll with increasing intervals: 200ms, 500ms, 1s, then 2s repeating.
	intervals := []time.Duration{
		200 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
	}
	const repeatInterval = 2 * time.Second

	deadline := time.Now().Add(timeout)
	step := 0
	for {
		delay := repeatInterval
		if step < len(intervals) {
			delay = intervals[step]
		}
		step++

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}

		info, err := os.Stat(fullPath)
		if err == nil {
			log.Printf("icloud: file materialized: %s", filepath.Base(fullPath))
			return info, nil
		}

		if time.Now().After(deadline) {
			log.Printf("icloud: timed out waiting for: %s", filepath.Base(fullPath))
			return nil, errICloudTimeout
		}
	}
}
