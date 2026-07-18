package fshelpers

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// CreateTestFile creates a single file in the given directory with dummy content.
// Returns the full path to the created file.
func CreateTestFile(t *testing.T, dir, name string) string {
	t.Helper()

	filePath := filepath.Join(dir, name)
	if err := os.WriteFile(filePath, []byte("test data"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %s %v", filePath, err)
	}
	return filePath
}

// CreateTestFiles creates multiple test files in the given directory.
// Returns the list of full paths to the created files.
func CreateTestFiles(t *testing.T, dir string, count int) []string {
	t.Helper()

	var files []string
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("testfile_%d", i)
		filePath := CreateTestFile(t, dir, name)
		files = append(files, filePath)
	}
	return files
}

// CreateNestedTestFiles create multiple test file in nested filepaths
// Returns the list of full paths to the created files.
func CreateNestedTestFiles(t *testing.T, dir string) []string {
	var files []string

	multipledirs := [][]string{
		{dir, "a", "b", "c"},
		{dir, "azerty", "qwerty"},
	}

	for _, dirs := range multipledirs {
		nested := filepath.Join(dirs...)
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("failed creating nested directories: %v", err)
		}

		for i := range len(dirs) {
			subdir := filepath.Join(dirs[:i+1]...)
			created := CreateTestFiles(t, subdir, 2)
			files = append(files, created...)
		}
	}
	// t.Logf("%v", files)
	return files

}

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
