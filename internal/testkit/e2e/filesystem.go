package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// AssertFilesExist verifies that all given file paths exist.
// It fails the test if any file is missing.
func AssertFilesExist(t *testing.T, filePaths []string) {
	t.Helper()

	for _, filePath := range filePaths {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("file should exist but does not: %s", filePath)
		} else if err != nil {
			t.Errorf("error checking file %s: %v", filePath, err)
		}
	}
}

// AssertFilesDeleted verifies that all given file paths have been deleted.
// It fails the test if any file still exists.
func AssertFilesDeleted(t *testing.T, filePaths []string) {
	t.Helper()

	for _, filePath := range filePaths {
		if _, err := os.Stat(filePath); err == nil {
			t.Errorf("file should be deleted but still exists: %s", filePath)
		} else if !os.IsNotExist(err) {
			t.Errorf("error checking file %s: %v", filePath, err)
		}
	}
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
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("failed to read directory %s: %v", dir, err)
	}

	return len(entries)
}

// AssertDirIsEmpty verifies that a directory contains no entries.
func AssertDirIsEmpty(t *testing.T, dir string) {
	t.Helper()

	count := CountFilesInDir(t, dir)
	if count > 0 {
		t.Errorf("directory should be empty but contains %d entries: %s", count, dir)
	}
}

// AssertDirNotEmpty verifies that a directory contains at least one entry.
func AssertDirNotEmpty(t *testing.T, dir string) {
	t.Helper()

	count := CountFilesInDir(t, dir)
	if count == 0 {
		t.Errorf("directory should not be empty: %s", dir)
	}
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	if len(entries) == 0 {
		return "", nil
	}

	return filepath.Join(dir, entries[0].Name()), nil
}
