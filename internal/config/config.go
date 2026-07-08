// Package config loads and validates the application configuration from
// command-line flags and environment variables.
package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListenAddr = ":8080"
	defaultInterval   = 30 * 24 * time.Hour
	defaultStateDir   = "/state"
	defaultLogPath    = ""
	defaultLogLevel   = "info"
)

// Config holds the resolved runtime configuration for PulseOrPerish.
type Config struct {
	ListenAddr string
	Password   string
	Interval   time.Duration
	DryRun     bool
	DataDir    string
	StateDir   string
	LogPath    string
	LogLevel   string
}

// Load parses args and environment variables into a Config.
// Flags take precedence over environment variables.
// It returns an error if parsing fails or the resulting config is invalid.
func Load(args []string) (Config, error) {
	cfg := Config{}

	fs := flag.NewFlagSet("pulseorperish", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	listen := fs.String("listen", envOrDefault("POP_LISTEN", defaultListenAddr), "HTTP listen address")
	password := fs.String("password", envOrDefault("POP_PASSWORD", ""), "password for proof-of-life")
	interval := fs.Duration("interval", durationEnvOrDefault("POP_INTERVAL", defaultInterval), "proof interval")
	dryRun := fs.Bool("dry-run", boolEnvOrDefault("POP_DRY_RUN", false), "enable dry-run mode (no deletion)")
	dataDir := fs.String("data-dir", envOrDefault("POP_DATA_DIR", ""), "directory whose content will be erased")
	stateDir := fs.String("state-dir", envOrDefault("POP_STATE_DIR", defaultStateDir), "directory used for persistent state")
	logPath := fs.String("log-path", envOrDefault("POP_LOG_PATH", defaultLogPath), "log file path (empty for stdout)")
	logLevel := fs.String("log-level", strings.ToLower(envOrDefault("POP_LOG_LEVEL", defaultLogLevel)), "log level: debug|info|warn|error|critical")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg = Config{
		ListenAddr: strings.TrimSpace(*listen),
		Password:   *password,
		Interval:   *interval,
		DryRun:     *dryRun,
		DataDir:    strings.TrimSpace(*dataDir),
		StateDir:   strings.TrimSpace(*stateDir),
		LogPath:    strings.TrimSpace(*logPath),
		LogLevel:   strings.ToLower(strings.TrimSpace(*logLevel)),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks that all required fields are set and contain acceptable values.
func (c Config) Validate() error {
	if c.Password == "" {
		return errors.New("password is required")
	}
	if c.Interval <= 0 {
		return errors.New("interval must be > 0")
	}
	if c.DataDir == "" {
		return errors.New("data-dir is required")
	}
	if !filepath.IsAbs(c.DataDir) {
		return errors.New("data-dir must be an absolute path")
	}
	if filepath.Clean(c.DataDir) == "/" {
		return errors.New("data-dir cannot be root path")
	}
	if c.StateDir == "" {
		return errors.New("state-dir is required")
	}
	if !filepath.IsAbs(c.StateDir) {
		return errors.New("state-dir must be an absolute path")
	}
	if c.LogLevel != "debug" && c.LogLevel != "info" && c.LogLevel != "warn" && c.LogLevel != "error" && c.LogLevel != "critical" {
		return fmt.Errorf("invalid log-level: %s", c.LogLevel)
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func durationEnvOrDefault(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func boolEnvOrDefault(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
