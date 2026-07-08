// Package logx configures a zerolog.Logger from a log level string and an
// optional file path.
package logx

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// New creates a zerolog.Logger at the given level. When path is non-empty, log
// lines are written to that file; otherwise they go to stdout. The returned
// io.Closer must be closed by the caller when path is non-empty.
func New(level, path string) (zerolog.Logger, io.Closer, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return zerolog.Logger{}, nil, err
	}

	var w io.Writer = os.Stdout
	var c io.Closer
	if strings.TrimSpace(path) != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return zerolog.Logger{}, nil, err
		}
		w = f
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
	case "critical":
		return zerolog.ErrorLevel, nil
	default:
		return zerolog.InfoLevel, nil
	}
}
