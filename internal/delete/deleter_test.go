package delete

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"pulseorperish/internal/fscrypt"
	"pulseorperish/internal/testkit/fshelpers"

	"github.com/rs/zerolog"
)

type fakeCrypter struct {
	err     error
	inputs  [][]string
	outputs []string
}

func (f *fakeCrypter) EncryptPaths(filesin []string, fileout string) error {
	regularFiles, err := collectTestRegularFiles(filesin)
	if err != nil {
		return err
	}
	if len(regularFiles) == 0 {
		return fscrypt.ErrNoFilesToEncrypt
	}
	cloned := append([]string(nil), regularFiles...)
	f.inputs = append(f.inputs, cloned)
	f.outputs = append(f.outputs, fileout)
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(fileout, []byte(strings.Join(regularFiles, "\n")), 0o600)
}

func (f *fakeCrypter) GetCryptedFileName(idx int) string {
	return fmt.Sprintf("file_%04d.tar.gz.pop", idx)
}

func collectTestRegularFiles(roots []string) ([]string, error) {
	var files []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type().IsRegular() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func TestWipeBinaryAvailableInTestEnvironment(t *testing.T) {
	if _, err := exec.LookPath("wipe"); err != nil {
		t.Fatalf("wipe binary is required for wipe-mode tests: %v", err)
	}
}

func TestClearDirectory_RefusesDangerousPaths(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{name: "empty", dir: ""},
		{name: "dot", dir: "."},
		{name: "root", dir: "/"},
	}

	del := NewSafeDeleter(zerolog.Nop(), false, "rm", "", "info")

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

func testClearDirectory_NonExistentDirectoryReturnsError(t *testing.T, deleteMode string) {
	d := t.TempDir()
	missing := filepath.Join(d, "does-not-exist")

	del := NewSafeDeleter(zerolog.Nop(), false, deleteMode, "-q -Q 1", "info")
	if err := del.ClearDirectory(context.Background(), missing); err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}
func TestRmClearDirectory_NonExistentDirectoryReturnsError(t *testing.T) {
	testClearDirectory_NonExistentDirectoryReturnsError(t, "rm")
}
func TestWipeClearDirectory_NonExistentDirectoryReturnsError(t *testing.T) {
	testClearDirectory_NonExistentDirectoryReturnsError(t, "wipe")
}

func testClearDirectory_DeletesRecursivelyAndKeepsRoot(t *testing.T, deleteMode string) {
	d := t.TempDir()
	fshelpers.CreateNestedTestFiles(t, d)

	del := NewSafeDeleter(zerolog.Nop(), false, deleteMode, "-q -Q 1", "info")
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if _, err := os.Stat(d); err != nil {
		t.Fatalf("expected root directory to remain: %v", err)
	}
	fshelpers.AssertDirIsEmpty(t, d)
}

func TestRmClearDirectory_DeletesRecursivelyAndKeepsRoot(t *testing.T) {
	testClearDirectory_DeletesRecursivelyAndKeepsRoot(t, "rm")
}

func TestWipeClearDirectory_DeletesRecursivelyAndKeepsRoot(t *testing.T) {
	testClearDirectory_DeletesRecursivelyAndKeepsRoot(t, "wipe")
}

func testClearDirectory_DryRunKeepsContent(t *testing.T, deleteMode string) {
	d := t.TempDir()
	fshelpers.CreateTestFile(t, d, "file.txt")

	del := NewSafeDeleter(zerolog.Nop(), true, deleteMode, "-q -Q 1", "info")
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("%s clear failed: %v", deleteMode, err)
	}

	if got := fshelpers.CountFilesInDir(t, d); got != 1 {
		t.Fatalf("expected directory content unchanged in %s dry-run, got %d entries", deleteMode, got)
	}
}

func TestRmClearDirectory_DryRunKeepsContent(t *testing.T) {
	testClearDirectory_DryRunKeepsContent(t, "rm")
}

func TestWipeClearDirectory_DryRunKeepsContent(t *testing.T) {
	testClearDirectory_DryRunKeepsContent(t, "wipe")
}

func testClearDirectory_RespectsCanceledContext(t *testing.T, deleteMode string) {
	d := t.TempDir()
	fshelpers.CreateTestFile(t, d, "file.txt")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	del := NewSafeDeleter(zerolog.Nop(), false, deleteMode, "-q -Q 1", "info")
	err := del.ClearDirectory(ctx, d)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error in %s mode, got: %v", deleteMode, err)
	}
}

func TestRmClearDirectory_RespectsCanceledContext(t *testing.T) {
	testClearDirectory_RespectsCanceledContext(t, "rm")
}

func TestWipeClearDirectory_RespectsCanceledContext(t *testing.T) {
	testClearDirectory_RespectsCanceledContext(t, "wipe")
}

func TestCryptClearDirectory_RequiresCrypter(t *testing.T) {
	d := t.TempDir()
	fshelpers.CreateTestFile(t, d, "file.txt")

	del := NewSafeDeleter(zerolog.Nop(), false, "crypt/rm", "", "info")
	err := del.ClearDirectory(context.Background(), d)
	if err == nil {
		t.Fatal("expected error when crypter is missing")
	}
	if !strings.Contains(err.Error(), "requires a crypter") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testCryptClearDirectory_CreatesArchivePerTopLevelEntry(t *testing.T, deleteMode string) {
	d := t.TempDir()
	topFile := fshelpers.CreateTestFile(t, d, "alpha.txt")
	nestedDir := filepath.Join(d, "nested")
	if err := os.MkdirAll(filepath.Join(nestedDir, "deep"), 0o755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}
	nestedFile := fshelpers.CreateTestFile(t, nestedDir, "beta.txt")
	deepFile := fshelpers.CreateTestFile(t, filepath.Join(nestedDir, "deep"), "gamma.txt")

	crypter := &fakeCrypter{}
	del := NewSafeDeleter(zerolog.Nop(), false, deleteMode, "-q -Q 1", "info").SetCrypter(crypter)
	if deleteMode == "crypt/wipe" {
		del.runner = func(ctx context.Context, bin string, args ...string) ([]byte, error) {
			target := args[len(args)-1]
			if err := os.RemoveAll(target); err != nil {
				return nil, err
			}
			return []byte("wiped"), nil
		}
	}

	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if _, err := os.Stat(d); err != nil {
		t.Fatalf("expected root directory to remain: %v", err)
	}
	fshelpers.AssertFilesDeleted(t, []string{topFile, nestedFile, deepFile})

	entries, err := os.ReadDir(d)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 archives, got %d entries", len(entries))
	}

	var gotNames []string
	for _, entry := range entries {
		gotNames = append(gotNames, entry.Name())
	}
	sort.Strings(gotNames)
	expectedNames := []string{"file_0000.tar.gz.pop", "file_0001.tar.gz.pop"}
	if strings.Join(gotNames, ",") != strings.Join(expectedNames, ",") {
		t.Fatalf("unexpected archive names: got %v want %v", gotNames, expectedNames)
	}

	if len(crypter.inputs) != 2 {
		t.Fatalf("expected 2 encryption calls, got %d", len(crypter.inputs))
	}
	if len(crypter.inputs[0]) != 1 || crypter.inputs[0][0] != topFile {
		t.Fatalf("expected first archive to contain the top-level file, got %v", crypter.inputs[0])
	}
	sort.Strings(crypter.inputs[1])
	expectedNested := []string{deepFile, nestedFile}
	sort.Strings(expectedNested)
	if strings.Join(crypter.inputs[1], ",") != strings.Join(expectedNested, ",") {
		t.Fatalf("expected nested archive contents %v, got %v", expectedNested, crypter.inputs[1])
	}
}

func TestCryptRmClearDirectory_CreatesArchivePerTopLevelEntry(t *testing.T) {
	testCryptClearDirectory_CreatesArchivePerTopLevelEntry(t, "crypt/rm")
}

func TestCryptWipeClearDirectory_CreatesArchivePerTopLevelEntry(t *testing.T) {
	testCryptClearDirectory_CreatesArchivePerTopLevelEntry(t, "crypt/wipe")
}

func TestCryptRmClearDirectory_DryRunKeepsContentAndSkipsArchive(t *testing.T) {
	d := t.TempDir()
	fshelpers.CreateTestFile(t, d, "file.txt")
	crypter := &fakeCrypter{}

	del := NewSafeDeleter(zerolog.Nop(), true, "crypt/rm", "", "info").SetCrypter(crypter)
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if got := fshelpers.CountFilesInDir(t, d); got != 1 {
		t.Fatalf("expected directory content unchanged in dry-run, got %d entries", got)
	}
	if len(crypter.outputs) != 0 {
		t.Fatalf("expected no archive to be created in dry-run, got %v", crypter.outputs)
	}
}

func TestCryptRmClearDirectory_RespectsCanceledContext(t *testing.T) {
	d := t.TempDir()
	fshelpers.CreateTestFile(t, d, "file.txt")
	crypter := &fakeCrypter{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	del := NewSafeDeleter(zerolog.Nop(), false, "crypt/rm", "", "info").SetCrypter(crypter)
	err := del.ClearDirectory(ctx, d)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got: %v", err)
	}
	if len(crypter.outputs) != 0 {
		t.Fatalf("expected no archive attempt after cancellation, got %v", crypter.outputs)
	}
}

func TestCryptRmClearDirectory_SkipsDeleteWhenEncryptionFails(t *testing.T) {
	d := t.TempDir()
	original := fshelpers.CreateTestFile(t, d, "file.txt")
	crypter := &fakeCrypter{err: errors.New("boom")}

	del := NewSafeDeleter(zerolog.Nop(), false, "crypt/rm", "", "info").SetCrypter(crypter)
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("expected nil despite encryption failure, got: %v", err)
	}

	fshelpers.AssertFilesExist(t, []string{original})
	if got := fshelpers.CountFilesInDir(t, d); got != 1 {
		t.Fatalf("expected only original file to remain, got %d entries", got)
	}
}

func TestCryptRmClearDirectory_DeletesEmptyDirectoriesWithoutArchive(t *testing.T) {
	d := t.TempDir()
	emptyDir := filepath.Join(d, "empty")
	if err := os.MkdirAll(filepath.Join(emptyDir, "nested"), 0o755); err != nil {
		t.Fatalf("failed to create empty nested directory: %v", err)
	}
	crypter := &fakeCrypter{}

	del := NewSafeDeleter(zerolog.Nop(), false, "crypt/rm", "", "info").SetCrypter(crypter)
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if _, err := os.Stat(emptyDir); !os.IsNotExist(err) {
		t.Fatalf("expected empty directory to be deleted, got err=%v", err)
	}
	if len(crypter.outputs) != 0 {
		t.Fatalf("expected no archive for empty directory, got %v", crypter.outputs)
	}
	fshelpers.AssertDirIsEmpty(t, d)
}

func TestCryptRmClearDirectory_SkipsArchiveNamesThatAlreadyExist(t *testing.T) {
	d := t.TempDir()
	_ = fshelpers.CreateTestFile(t, d, "file_0000.tar.gz.pop")
	original := fshelpers.CreateTestFile(t, d, "payload.txt")
	crypter := &fakeCrypter{}

	del := NewSafeDeleter(zerolog.Nop(), false, "crypt/rm", "", "info").SetCrypter(crypter)
	if err := del.ClearDirectory(context.Background(), d); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	fshelpers.AssertFilesDeleted(t, []string{original})
	if len(crypter.outputs) != 1 {
		t.Fatalf("expected one archive output, got %d", len(crypter.outputs))
	}
	if !strings.HasSuffix(crypter.outputs[0], "file_0001.tar.gz.pop") {
		t.Fatalf("expected archive naming to skip existing files, got %v", crypter.outputs)
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
	del := NewSafeDeleter(logger, false, "rm", "", "info")

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

func TestBuildWipeArgs_AppendsSafetyFlagsAndTarget(t *testing.T) {

	args := buildWipeArgs("-q -Q 1", "info", "/data/a")
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "-q") || !strings.Contains(joined, "-Q 1") {
		t.Fatalf("expected configured args to be preserved, got: %v", args)
	}
	for _, required := range []string{"-c", "-r", "-f", "-s"} {
		if !containsArg(args, required) {
			t.Fatalf("expected %s in wipe args: %v", required, args)
		}
	}
}

func TestBuildWipeArgs_DoesNotForceSilentInDebug(t *testing.T) {
	args := buildWipeArgs("-q -Q 1", "debug", "/data/b")
	if containsArg(args, "-s") {
		t.Fatalf("did not expect -s when log level is debug, got: %v", args)
	}
}
