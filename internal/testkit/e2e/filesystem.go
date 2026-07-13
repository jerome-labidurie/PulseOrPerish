package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"pulseorperish/internal/testkit/fshelpers"
)

// AssertFilesExist verifies that all given file paths exist.
// It fails the test if any file is missing.
func AssertFilesExist(t *testing.T, filePaths []string) {
	fshelpers.AssertFilesExist(t, filePaths)
}

// AssertFilesDeleted verifies that all given file paths have been deleted.
// It fails the test if any file still exists.
func AssertFilesDeleted(t *testing.T, filePaths []string) {
	fshelpers.AssertFilesDeleted(t, filePaths)
}

// WaitForFilesDeleted waits for files to be deleted, with timeout and retry.
// It returns an error if the timeout is exceeded before all files are deleted.
func WaitForFilesDeleted(ctx context.Context, filePaths []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 100 * time.Millisecond

	for {
		if time.Now().After(deadline) {
			// List remaining files for error message
			remaining := []string{}
			for _, f := range filePaths {
				if _, err := os.Stat(f); err == nil {
					remaining = append(remaining, f)
				}
			}
			return fmt.Errorf("timeout waiting for files to be deleted: %v", remaining)
		}

		allDeleted := true
		for _, filePath := range filePaths {
			if _, err := os.Stat(filePath); err == nil {
				allDeleted = false
				break
			}
		}

		if allDeleted {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(checkInterval):
			// Continue
		}
	}
}

// CountFilesInDir returns the number of files/entries directly inside a directory.
func CountFilesInDir(t *testing.T, dir string) int {
	return fshelpers.CountFilesInDir(t, dir)
}

// AssertDirIsEmpty verifies that a directory contains no entries.
func AssertDirIsEmpty(t *testing.T, dir string) {
	fshelpers.AssertDirIsEmpty(t, dir)
}

// AssertDirNotEmpty verifies that a directory contains at least one entry.
func AssertDirNotEmpty(t *testing.T, dir string) {
	fshelpers.AssertDirNotEmpty(t, dir)
}

// WaitForDirEmpty waits for a directory to become empty, with timeout.
func WaitForDirEmpty(ctx context.Context, dir string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 100 * time.Millisecond

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for directory to be empty: %s", dir)
		}

		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error reading directory: %w", err)
		}

		if len(entries) == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(checkInterval):
			// Continue
		}
	}
}

// FindFirstFileInDir returns the path to the first file found in a directory.
// Returns empty string if directory is empty or does not exist.
func FindFirstFileInDir(dir string) (string, error) {
	return fshelpers.FindFirstFileInDir(dir)
}
