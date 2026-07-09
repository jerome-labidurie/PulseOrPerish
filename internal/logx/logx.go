// Package logx configures a zerolog.Logger from a log level string and an
// optional file path.
package logx

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// New creates a zerolog.Logger at the given level. Logs are always written to
// stdout. When path is non-empty it must be an existing directory: a timestamped
// file (format 20060102-150405.log) is created inside it and logs are written
// to both stdout and that file. The returned io.Closer must be closed by the
// caller when path is non-empty.
func New(level, path string) (zerolog.Logger, io.Closer, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return zerolog.Logger{}, nil, err
	}

	var w io.Writer = os.Stdout
	var c io.Closer
	if strings.TrimSpace(path) != "" {
		info, err := os.Stat(path)
		if err != nil {
			return zerolog.Logger{}, nil, fmt.Errorf("log-path: %w", err)
		}
		if !info.IsDir() {
			return zerolog.Logger{}, nil, fmt.Errorf("log-path %q is not a directory", path)
		}
		filename := time.Now().Format("20060102-150405.log")
		f, err := os.OpenFile(path+string(os.PathSeparator)+filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return zerolog.Logger{}, nil, fmt.Errorf("log-path: %w", err)
		}
		w = io.MultiWriter(os.Stdout, f)
		c = f
	}

	logger := zerolog.New(w).With().Timestamp().Logger().Level(lvl)
	return logger, c, nil
}

func parseLevel(level string) (zerolog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zerolog.DebugLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "warn", "warning":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	default:
		return zerolog.InfoLevel, nil
	}
}
