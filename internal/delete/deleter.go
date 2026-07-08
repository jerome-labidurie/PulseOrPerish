// Package delete provides the logic for safely removing directory contents
// when the proof-of-life deadline is exceeded.
package delete

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// Deleter removes all entries inside a directory.
type Deleter interface {
	// ClearDirectory deletes every entry directly inside dir.
	// It returns an error if the directory cannot be read, or ctx.Err() if the
	// context is cancelled before all entries are processed.
	ClearDirectory(ctx context.Context, dir string) error
}

// SafeDeleter is a Deleter that refuses to operate on dangerous paths such as
// the filesystem root, and supports a dry-run mode that only logs intended
// deletions without actually removing anything.
type SafeDeleter struct {
	log    zerolog.Logger
	dryRun bool
}

// NewSafeDeleter returns a SafeDeleter. When dryRun is true, deletions are
// logged but no file is actually removed.
func NewSafeDeleter(log zerolog.Logger, dryRun bool) *SafeDeleter {
	return &SafeDeleter{log: log, dryRun: dryRun}
}

// ClearDirectory deletes all entries directly inside dir.
// It returns an error if dir resolves to a dangerous path (empty, ".", or "/"),
// or if the directory listing fails. Individual entry removal errors are logged
// but do not abort the iteration. Returns ctx.Err() if the context is cancelled.
func (d *SafeDeleter) ClearDirectory(ctx context.Context, dir string) error {
	clean := filepath.Clean(dir)
	if clean == "." || clean == "" || clean == "/" {
		return errors.New("refusing to clear dangerous path")
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		target := filepath.Join(clean, e.Name())
		if d.dryRun {
			d.log.Warn().Str("path", target).Msg("dry-run enabled: would delete entry")
			continue
		}
		if err := os.RemoveAll(target); err != nil {
			d.log.Error().Err(err).Str("path", target).Msg("failed deleting entry")
		} else {
			d.log.Debug().Str("path", target).Msg("deleted entry")
		}
	}
	return nil
}
