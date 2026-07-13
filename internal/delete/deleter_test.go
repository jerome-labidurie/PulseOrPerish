package delete

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"pulseorperish/internal/testkit/fshelpers"

	"github.com/rs/zerolog"
)

func TestClearDirectory_RefusesDangerousPaths(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{name: "empty", dir: ""},
		{name: "dot", dir: "."},
		{name: "root", dir: "/"},
	}

	del := NewSafeDeleter(zerolog.Nop(), false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := del.ClearDirectory(context.Background(), tt.dir)
			if err == nil {
				t.Fatal("expected an error for dangerous path")
			}
			if !strings.Contains(err.Error(), "dangerous path") {
				t.Fatalf("expected dangerous path error, got: %v", err)
			}
		})
	}
}

func TestClearDirectory_NonExistentDirectoryReturnsNil(t *testing.T) {
	d := t.TempDir()
	missing := filepath.Join(d, "does-not-exist")

	del := NewSafeDeleter(zerolog.Nop(), false)
	if err := del.ClearDirectory(context.Background(), missing); err != nil {
		t.Fatalf("expected nil for non-existent directory, got: %v", err)
	}
}

func TestClearDirectory_DeletesRecursivelyAndKeepsRoot(t *testing.T) {
	d := t.TempDir()
	nested := filepath.Join(d, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("failed creating nested directories: %v", err)
	}

	fshelpers.CreateTestFile(t, d, "root.txt")
	fshelpers.CreateTestFile(t, filepath.Join(d, "a"), "a.txt")
	fshelpers.CreateTestFile(t, filepath.Join(d, "a", "b"), "b.txt")
	fshelpers.CreateTestFile(t, nested, "c.txt")

	del := NewSafeDeleter(zerolog.Nop(), false)
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if _, err := os.Stat(d); err != nil {
		t.Fatalf("expected root directory to remain: %v", err)
	}
	fshelpers.AssertDirIsEmpty(t, d)
}

func TestClearDirectory_DryRunKeepsContent(t *testing.T) {
	d := t.TempDir()
	fshelpers.CreateTestFile(t, d, "file.txt")

	del := NewSafeDeleter(zerolog.Nop(), true)
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if got := fshelpers.CountFilesInDir(t, d); got != 1 {
		t.Fatalf("expected directory content unchanged in dry-run, got %d entries", got)
	}
}

func TestClearDirectory_ContinuesWhenRemoveAllFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based deletion failure is not portable on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("permission-based deletion failure is not reliable when running as root")
	}

	d := t.TempDir()
	deletableFile := fshelpers.CreateTestFile(t, d, "deletable.txt")
	blockedDir := filepath.Join(d, "blocked")
	if err := os.Mkdir(blockedDir, 0o755); err != nil {
		t.Fatalf("failed to create blocked directory: %v", err)
	}
	blockedFile := fshelpers.CreateTestFile(t, blockedDir, "blocked.txt")

	if err := os.Chmod(blockedDir, 0o000); err != nil {
		t.Fatalf("failed to lock blocked directory permissions: %v", err)
	}
	defer os.Chmod(blockedDir, 0o755)

	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	del := NewSafeDeleter(logger, false)

	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("expected nil despite deletion errors, got: %v", err)
	}
	if err := os.Chmod(blockedDir, 0o755); err != nil {
		t.Fatalf("failed to restore blocked directory permissions for assertions: %v", err)
	}

	if _, err := os.Stat(deletableFile); !os.IsNotExist(err) {
		t.Fatalf("expected removable sibling file to be deleted, got err=%v", err)
	}
	if _, err := os.Stat(blockedDir); err != nil {
		t.Fatalf("expected blocked directory to remain after failed deletion: %v", err)
	}
	if _, err := os.Stat(blockedFile); err != nil {
		t.Fatalf("expected blocked file to remain after failed deletion: %v", err)
	}

	logs := buf.String()
	if strings.Count(logs, "failed deleting entry") != 1 {
		t.Fatalf("expected exactly one logged deletion failure, logs: %s", logs)
	}
	if !strings.Contains(logs, blockedDir) {
		t.Fatalf("expected logs to contain the blocked directory path, logs: %s", logs)
	}
}
