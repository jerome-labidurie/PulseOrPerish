package main
package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestWalkDirectory_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	files, err := WalkDirectory(tmpDir)
	if err != nil {
		t.Errorf("WalkDirectory failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files in empty directory, got %d", len(files))
	}
}

func TestWalkDirectory_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := WalkDirectory(tmpDir)
	if err != nil {
		t.Errorf("WalkDirectory failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if files[0] != testFile {
		t.Errorf("expected %s, got %s", testFile, files[0])
	}
}

func TestWalkDirectory_NestedDirs(t *testing.T) {
	tmpDir := t.TempDir()
	// Create nested structure
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(subDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := WalkDirectory(tmpDir)
	if err != nil {
		t.Errorf("WalkDirectory failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Sort for consistent comparison
	sort.Strings(files)
	expected := []string{file1, file2}
	sort.Strings(expected)

	for i, file := range files {
		if file != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], file)
		}
	}
}

func TestWalkDirectory_SkipsDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	// Create directories
	subDir1 := filepath.Join(tmpDir, "dir1")
	subDir2 := filepath.Join(tmpDir, "dir2")
	if err := os.MkdirAll(subDir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatal(err)
	}

	// Add a file
	file1 := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(file1, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := WalkDirectory(tmpDir)
	if err != nil {
		t.Errorf("WalkDirectory failed: %v", err)
	}

	// Should only return the file, not the directories
	if len(files) != 1 {
		t.Errorf("expected 1 file (directories should be skipped), got %d files", len(files))
	}
	if files[0] != file1 {
		t.Errorf("expected %s, got %s", file1, files[0])
	}
}

func TestWalkDirectory_InvalidPath(t *testing.T) {
	_, err := WalkDirectory("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestWalkDirectory_EmptyPathArg(t *testing.T) {
	_, err := WalkDirectory("")
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}
