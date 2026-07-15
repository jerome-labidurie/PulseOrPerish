package config

import (
	"strings"
	"testing"
)

func TestLoadAndValidate(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIR", "/data")

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
	t.Setenv("POP_DATA_DIR", "/data")
	_, err := Load([]string{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDryRunFromEnv(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIR", "/data")
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
	t.Setenv("POP_DATA_DIR", "/data")
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
	t.Setenv("POP_DATA_DIR", "/data")
	t.Setenv("POP_DELETE_METHOD", "wipe")

	cfg, err := Load([]string{"-delete-method=rm"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DeleteMode != "rm" {
		t.Fatalf("expected delete method rm from flag, got %s", cfg.DeleteMode)
	}
}

func TestWipeArgsFromEnv(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIR", "/data")
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
	t.Setenv("POP_DATA_DIR", "/data")

	_, err := Load([]string{"-delete-method=other"})
	if err == nil {
		t.Fatal("expected error for invalid delete method")
	}
	if !strings.Contains(err.Error(), "invalid delete-method") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWipeArgsRejectsPositionalPath(t *testing.T) {
	t.Setenv("POP_PASSWORD", "pw")
	t.Setenv("POP_DATA_DIR", "/data")
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
	t.Setenv("POP_DATA_DIR", "/data")
	t.Setenv("POP_DELETE_METHOD", "wipe")

	args := "-q -Q 2 -b 12 -e -F -k -l 4K -M a -o 1M -P 1 -R /dev/urandom -S r -T 2 -Z"
	if _, err := Load([]string{"-wipe-args=" + args}); err != nil {
		t.Fatalf("expected documented options to be accepted, got: %v", err)
	}
}

func TestConfigRedactedMasksPassword(t *testing.T) {
	cfg := Config{
		ListenAddr: ":8080",
		Password:   "super-secret",
		DataDir:    "/data",
		StateDir:   "/state",
		LogLevel:   "info",
	}

	redacted := cfg.Redacted()

	if redacted.Password != "***" {
		t.Fatalf("expected masked password, got %q", redacted.Password)
	}
	if cfg.Password != "super-secret" {
		t.Fatalf("expected original config to remain unchanged, got %q", cfg.Password)
	}
	if redacted.ListenAddr != cfg.ListenAddr || redacted.DataDir != cfg.DataDir || redacted.StateDir != cfg.StateDir || redacted.LogLevel != cfg.LogLevel {
		t.Fatal("expected non-secret fields to be preserved")
	}
}
