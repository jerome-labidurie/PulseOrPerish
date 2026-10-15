// Package config loads and validates the application configuration from
// command-line flags and environment variables.
package config

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
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
	defaultDeleteMode = "rm"
	defaultWipeArgs   = "-q -Q 1"
	randomPasswordLen = 32
)

var validDeleteModes = []string{"rm", "wipe", "crypt/rm", "crypt/wipe"}

var sizedNumberPattern = regexp.MustCompile(`^\d+([KMG])?$`)

// Config holds the resolved runtime configuration for PulseOrPerish.
type Config struct {
	ListenAddr    string
	Password      string
	Interval      time.Duration
	DryRun        bool
	DeleteMode    string
	WipeArgs      string
	CryptPassword string
	DataDirs      []string
	StateDir      string
	LogPath       string
	LogLevel      string
}

// Redacted returns a copy of the config safe to emit in logs.
func (c Config) Redacted() Config {
	copy := c
	if copy.Password != "" {
		copy.Password = "***"
	}
	if copy.CryptPassword != "" {
		copy.CryptPassword = "***"
	}
	return copy
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
	deleteMode := fs.String("delete-method", strings.ToLower(envOrDefault("POP_DELETE_METHOD", defaultDeleteMode)), "deletion method: rm|wipe|crypt/rm|crypt/wipe")
	wipeArgs := fs.String("wipe-args", envOrDefault("POP_WIPE_ARGS", defaultWipeArgs), "wipe arguments (whitelisted options only)")
	cryptPassword := fs.String("crypt-password", envOrDefault("POP_CRYPT_PASSWORD", ""), "password source for crypt delete methods")
	dataDir := fs.String("data-dirs", envOrDefault("POP_DATA_DIRS", ""), "directories whose content will be erased")
	stateDir := fs.String("state-dir", envOrDefault("POP_STATE_DIR", defaultStateDir), "directory used for persistent state")
	logPath := fs.String("log-path", envOrDefault("POP_LOG_PATH", defaultLogPath), "log directory (if set, logs are also written to a timestamped file)")
	logLevel := fs.String("log-level", strings.ToLower(envOrDefault("POP_LOG_LEVEL", defaultLogLevel)), "log level: debug|info|warn|error")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	dataDirs := strings.SplitN(strings.TrimSpace(*dataDir), ",", -1)

	resolvedCryptPassword, err := ResolveCryptPassword(strings.TrimSpace(*cryptPassword), *password)
	if err != nil {
		return Config{}, err
	}

	cfg = Config{
		ListenAddr:    strings.TrimSpace(*listen),
		Password:      *password,
		Interval:      *interval,
		DryRun:        *dryRun,
		DeleteMode:    strings.ToLower(strings.TrimSpace(*deleteMode)),
		WipeArgs:      strings.TrimSpace(*wipeArgs),
		CryptPassword: resolvedCryptPassword,
		DataDirs:      dataDirs,
		StateDir:      strings.TrimSpace(*stateDir),
		LogPath:       strings.TrimSpace(*logPath),
		LogLevel:      strings.ToLower(strings.TrimSpace(*logLevel)),
	}

	// fmt.Printf("conf: %d, %v, '%s'\n", len(cfg.DataDirs), cfg, cfg.DataDirs[0])

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
		return errors.New("interval must be > 0 " + c.Interval.String())
	}
	if len(c.DataDirs) == 1 && c.DataDirs[0] == "" {
		return errors.New("data-dirs is required")
	}
	for _, dir := range c.DataDirs {
		if !filepath.IsAbs(dir) {
			return errors.New("data-dirs must be an absolute path " + dir)
		}
		if filepath.Clean(dir) == "/" {
			return errors.New("data-dirs cannot be root path")
		}
	}
	if c.StateDir == "" {
		return errors.New("state-dir is required")
	}
	if !filepath.IsAbs(c.StateDir) {
		return errors.New("state-dir must be an absolute path " + c.StateDir)
	}

	validDeleteMode := false
	for _, mode := range validDeleteModes {
		if c.DeleteMode == mode {
			validDeleteMode = true
			break
		}
	}
	if !validDeleteMode {
		return fmt.Errorf("invalid delete-method %q, must be one of %v", c.DeleteMode, validDeleteModes)
	}
	if c.DeleteMode == "wipe" || c.DeleteMode == "crypt/wipe" {
		if err := validateWipeArgs(c.WipeArgs); err != nil {
			return err
		}
	}
	if strings.HasPrefix(c.DeleteMode, "crypt/") && c.CryptPassword == "" {
		return errors.New("crypt-password resolved to an empty value")
	}
	validLevels := []string{"debug", "info", "warn", "error"}
	validLevel := false
	for _, l := range validLevels {
		if c.LogLevel == l {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid log-level %q, must be one of %v", c.LogLevel, validLevels)
	}
	return nil
}

// ResolveCryptPassword returns the password to use for crypt delete methods.
// An empty spec falls back to the main password. The special value "random"
// generates a one-time password. A "file:/path" spec loads the password from a file.
func ResolveCryptPassword(spec string, fallback string) (string, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return fallback, nil
	}
	if trimmed == "random" {
		return generateRandomPassword(randomPasswordLen)
	}
	if strings.HasPrefix(trimmed, "file:") {
		return resolveCryptPasswordFile(strings.TrimPrefix(trimmed, "file:"))
	}
	return trimmed, nil
}

func resolveCryptPasswordFile(path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", errors.New("crypt-password file path is empty")
	}
	data, err := os.ReadFile(trimmedPath)
	if err != nil {
		return "", fmt.Errorf("read crypt-password file: %w", err)
	}
	password := strings.TrimRight(string(data), "\r\n")
	if password == "" {
		return "", errors.New("crypt-password file is empty")
	}
	return password, nil
}

func generateRandomPassword(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("generate random crypt-password: %w", err)
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	chars := make([]byte, length)
	for i, value := range buf {
		chars[i] = alphabet[int(value)%len(alphabet)]
	}
	return string(chars), nil
}

// envOrDefault returns the trimmed environment value or fallback when empty.
func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

// durationEnvOrDefault parses a duration from env or returns fallback on failure.
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

// boolEnvOrDefault parses a boolean from env or returns fallback on failure.
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

func validateWipeArgs(args string) error {
	tokens := strings.Fields(strings.TrimSpace(args))
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok {
		case "-q", "-e", "-F", "-k", "-c", "-r", "-f", "-s", "-Z":
			continue
		case "-b", "-P", "-Q", "-T":
			next, ok := nextToken(tokens, &i, tok)
			if !ok {
				return fmt.Errorf("%s requires a value", tok)
			}
			if err := requirePositiveInt(next, tok); err != nil {
				return err
			}
			if tok == "-b" {
				bufPow, _ := strconv.Atoi(next)
				if bufPow < 1 || bufPow > 30 {
					return fmt.Errorf("-b value %q must be between 1 and 30", next)
				}
			}
		case "-l", "-o":
			next, ok := nextToken(tokens, &i, tok)
			if !ok {
				return fmt.Errorf("%s requires a value", tok)
			}
			if !sizedNumberPattern.MatchString(next) {
				return fmt.Errorf("%s value %q must be an integer with optional K/M/G suffix", tok, next)
			}
		case "-M":
			next, ok := nextToken(tokens, &i, tok)
			if !ok {
				return errors.New("-M requires a value")
			}
			if next != "l" && next != "r" && next != "a" {
				return fmt.Errorf("-M value %q must be one of l, r, a", next)
			}
		case "-R":
			next, ok := nextToken(tokens, &i, tok)
			if !ok {
				return errors.New("-R requires a value")
			}
			if strings.HasPrefix(next, "-") {
				return fmt.Errorf("-R value %q is invalid", next)
			}
		case "-S":
			next, ok := nextToken(tokens, &i, tok)
			if !ok {
				return errors.New("-S requires a value")
			}
			if next != "r" && next != "c" && next != "p" {
				return fmt.Errorf("-S value %q must be one of r, c, p", next)
			}
		default:
			if strings.HasPrefix(tok, "-") {
				return fmt.Errorf("unsupported wipe option %q", tok)
			}
			return fmt.Errorf("unexpected positional argument %q in wipe args", tok)
		}
	}
	return nil
}

func nextToken(tokens []string, i *int, opt string) (string, bool) {
	if *i+1 >= len(tokens) {
		return "", false
	}
	*i = *i + 1
	next := tokens[*i]
	if strings.HasPrefix(next, "-") {
		return "", false
	}
	return next, true
}

func requirePositiveInt(raw, opt string) error {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("%s value %q must be an integer", opt, raw)
	}
	if v <= 0 {
		return fmt.Errorf("%s value %q must be > 0", opt, raw)
	}
	if v > math.MaxInt32 {
		return fmt.Errorf("%s value %q is too large", opt, raw)
	}
	return nil
}
