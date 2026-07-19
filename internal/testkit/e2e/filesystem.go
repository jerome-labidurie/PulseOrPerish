package e2e

import (
	"context"
	"fmt"
	"os"
	"time"
)

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
