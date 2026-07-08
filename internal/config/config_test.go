package config

import "testing"

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
