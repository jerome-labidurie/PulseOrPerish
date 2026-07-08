package delete

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestClearDirectoryKeepsRoot(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	del := NewSafeDeleter(zerolog.Nop(), false)
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if _, err := os.Stat(d); err != nil {
		t.Fatalf("expected directory to remain: %v", err)
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty directory, got %d entries", len(entries))
	}
}

func TestDryRunKeepsContent(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	del := NewSafeDeleter(zerolog.Nop(), true)
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected directory content unchanged in dry-run, got %d entries", len(entries))
	}
}
