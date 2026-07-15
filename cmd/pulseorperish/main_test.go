package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDataDirReturnsCanonicalPath(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("failed to create target directory: %v", err)
	}

	linkDir := filepath.Join(root, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	resolved, err := resolveDataDir(linkDir)
	if err != nil {
		t.Fatalf("resolveDataDir() failed: %v", err)
	}
	if resolved != realDir {
		t.Fatalf("expected canonical path %q, got %q", realDir, resolved)
	}
}

func TestResolveDataDirFailsForMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	resolved, err := resolveDataDir(missing)
	if err == nil {
		t.Fatal("expected an error for missing data directory")
	}
	if resolved != "" {
		t.Fatalf("expected empty resolved path on error, got %q", resolved)
	}
}
