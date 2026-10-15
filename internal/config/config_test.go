package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndValidate(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")

	cfg, err := Load([]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Interval.String() != "720h0m0s" {
		t.Fatalf("unexpected default interval: %s", cfg.Interval)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("unexpected default listen addr: %s", cfg.ListenAddr)
	}
	if cfg.DeleteMode != "rm" {
		t.Fatalf("unexpected default delete method: %s", cfg.DeleteMode)
	}
	if cfg.WipeArgs != "-q -Q 1" {
		t.Fatalf("unexpected default wipe args: %s", cfg.WipeArgs)
	}
}

func TestRequiresPassword(t *testing.T) {
	t.Setenv("POP_DATA_DIRS", "/data")
	_, err := Load([]string{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMultiplesDataDirs(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data1,/data2")

	cfg, err := Load([]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.DataDirs) != 2 {
		t.Fatalf("expected 2 dirs, got %v", len(cfg.DataDirs))
	}
	if cfg.DataDirs[0] != "/data1" || cfg.DataDirs[1] != "/data2" {
		t.Fatalf("didn't get expected data dirs, got %v", cfg.DataDirs)
	}
}

func TestDryRunFromEnv(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DRY_RUN", "true")

	cfg, err := Load([]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.DryRun {
		t.Fatal("expected dry-run to be enabled")
	}
}

func TestDeleteMethodFromEnv(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DELETE_METHOD", "wipe")
	t.Setenv("POP_WIPE_ARGS", "-q -Q 1")

	cfg, err := Load([]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DeleteMode != "wipe" {
		t.Fatalf("expected delete method wipe, got %s", cfg.DeleteMode)
	}
}

func TestDeleteMethodFlagOverridesEnv(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DELETE_METHOD", "wipe")

	cfg, err := Load([]string{"-delete-method=rm"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DeleteMode != "rm" {
		t.Fatalf("expected delete method rm from flag, got %s", cfg.DeleteMode)
	}
}

func TestCryptDeleteMethodFromEnv(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DELETE_METHOD", "crypt/rm")

	cfg, err := Load([]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DeleteMode != "crypt/rm" {
		t.Fatalf("expected delete method crypt/rm, got %s", cfg.DeleteMode)
	}
}

func TestWipeArgsFromEnv(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DELETE_METHOD", "wipe")
	t.Setenv("POP_WIPE_ARGS", "-q -Q 3 -e")

	cfg, err := Load([]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.WipeArgs != "-q -Q 3 -e" {
		t.Fatalf("unexpected wipe args: %s", cfg.WipeArgs)
	}
}

func TestValidateDeleteMethodMustBeRmOrWipe(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")

	_, err := Load([]string{"-delete-method=other"})
	if err == nil {
		t.Fatal("expected error for invalid delete method")
	}
	if !strings.Contains(err.Error(), "invalid delete-method") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCryptWipeArgs(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DELETE_METHOD", "crypt/wipe")

	if _, err := Load([]string{"-wipe-args=-q -Q 2 -e"}); err != nil {
		t.Fatalf("expected crypt/wipe args to be validated like wipe, got: %v", err)
	}
}

func TestValidateWipeArgsRejectsPositionalPath(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DELETE_METHOD", "wipe")

	_, err := Load([]string{"-wipe-args=-q -Q 1 /tmp/evil"})
	if err == nil {
		t.Fatal("expected error for positional path in wipe args")
	}
	if !strings.Contains(err.Error(), "unexpected positional argument") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWipeArgsAcceptsDocumentedOptions(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DELETE_METHOD", "wipe")

	args := "-q -Q 2 -b 12 -e -F -k -l 4K -M a -o 1M -P 1 -R /dev/urandom -S r -T 2 -Z"
	if _, err := Load([]string{"-wipe-args=" + args}); err != nil {
		t.Fatalf("expected documented options to be accepted, got: %v", err)
	}
}

func TestConfigRedactedMasksPassword(t *testing.T) {
	cfg := Config{
		ListenAddr:    ":8080",
		Password:      "super-secret",
		CryptPassword: "encrypt-secret",
		DataDirs:      []string{"/data"},
		StateDir:      "/state",
		LogLevel:      "info",
	}

	redacted := cfg.Redacted()

	if redacted.Password != "***" {
		t.Fatalf("expected masked password, got %q", redacted.Password)
	}
	if redacted.CryptPassword != "***" {
		t.Fatalf("expected masked crypt password, got %q", redacted.CryptPassword)
	}
	if cfg.Password != "super-secret" {
		t.Fatalf("expected original config to remain unchanged, got %q", cfg.Password)
	}
	if cfg.CryptPassword != "encrypt-secret" {
		t.Fatalf("expected original crypt password to remain unchanged, got %q", cfg.CryptPassword)
	}
	if redacted.ListenAddr != cfg.ListenAddr || redacted.DataDirs[0] != cfg.DataDirs[0] || redacted.StateDir != cfg.StateDir || redacted.LogLevel != cfg.LogLevel {
		t.Fatal("expected non-secret fields to be preserved")
	}
}

func TestLoadCryptPasswordFallsBackToMainPassword(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DELETE_METHOD", "crypt/rm")

	cfg, err := Load([]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.CryptPassword != "pw" {
		t.Fatalf("expected crypt password to fall back to main password, got %q", cfg.CryptPassword)
	}
}

func TestLoadCryptPasswordFromFlag(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")

	cfg, err := Load([]string{"-crypt-password=other-secret"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.CryptPassword != "other-secret" {
		t.Fatalf("expected explicit crypt password, got %q", cfg.CryptPassword)
	}
}

func TestLoadCryptPasswordFromFile(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	passwordFile := filepath.Join(t.TempDir(), "crypt-password.txt")
	if err := os.WriteFile(passwordFile, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatalf("failed to write password file: %v", err)
	}

	cfg, err := Load([]string{"-crypt-password=file:" + passwordFile})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.CryptPassword != "file-secret" {
		t.Fatalf("expected crypt password from file, got %q", cfg.CryptPassword)
	}
}

func TestLoadCryptPasswordFromEmptyFileFails(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	passwordFile := filepath.Join(t.TempDir(), "crypt-password.txt")
	if err := os.WriteFile(passwordFile, []byte("\n"), 0o600); err != nil {
		t.Fatalf("failed to write password file: %v", err)
	}

	_, err := Load([]string{"-crypt-password=file:" + passwordFile})
	if err == nil {
		t.Fatal("expected error for empty crypt password file")
	}
	if !strings.Contains(err.Error(), "crypt-password file is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadCryptPasswordRandomGeneratesValue(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIRS", "/data")
	t.Setenv("POP_DELETE_METHOD", "crypt/rm")

	cfg, err := Load([]string{"-crypt-password=random"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.CryptPassword == "" {
		t.Fatal("expected random crypt password to be generated")
	}
	if cfg.CryptPassword == "pw" {
		t.Fatal("expected random crypt password to differ from fallback password")
	}
}
