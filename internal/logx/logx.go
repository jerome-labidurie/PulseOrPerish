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
	zlog "github.com/rs/zerolog/log"
)

// New creates a zerolog.Logger at the given level. Logs are written in a human-
// readable console format to stdout. When path is non-empty it must be an
// existing directory: a timestamped file (format 20060102-150405.log) is
// created inside it and logs are also written there in JSON format. The
// returned io.Closer must be closed by the caller when path is non-empty.
// It also wires the logger into the global zerolog logger used by packages
// that log through zerolog/log.
func New(level, path string) (zerolog.Logger, io.Closer, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return zerolog.Logger{}, nil, err
	}

	stdoutWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	var w io.Writer = stdoutWriter
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
		w = io.MultiWriter(stdoutWriter, f)
		c = f
	}

	logger := zerolog.New(w).With().Timestamp().Logger().Level(lvl)
	zerolog.SetGlobalLevel(logger.GetLevel())
	zlog.Logger = logger
	return logger, c, nil
}

// parseLevel converts a user-provided level string into a zerolog level.
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
		return zerolog.InfoLevel, fmt.Errorf("invalid log-level: %q", level)
	}
}
