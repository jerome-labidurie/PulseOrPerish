// Package delete provides the logic for safely removing directory contents
// when the proof-of-life deadline is exceeded.
package delete

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	log        zerolog.Logger
	dryRun     bool
	deleteMode string
	wipeArgs   string
	logLevel   string
	runner     func(context.Context, string, ...string) ([]byte, error)
}

// NewSafeDeleter returns a SafeDeleter. When dryRun is true, deletions are
// logged but no file is actually removed.
func NewSafeDeleter(log zerolog.Logger, dryRun bool, deleteMode, wipeArgs, logLevel string) *SafeDeleter {
	return &SafeDeleter{
		log:        log,
		dryRun:     dryRun,
		deleteMode: strings.ToLower(strings.TrimSpace(deleteMode)),
		wipeArgs:   strings.TrimSpace(wipeArgs),
		logLevel:   strings.ToLower(strings.TrimSpace(logLevel)),
		runner:     runCommand,
	}
}

// ClearDirectory deletes all entries directly inside dir.
// It returns an error if dir resolves to a dangerous path (empty, ".", or "/"),
// or if the directory listing fails. Individual entry removal errors are logged
// but do not abort the iteration. Returns ctx.Err() if the context is cancelled.
func (d *SafeDeleter) ClearDirectory(ctx context.Context, dir string) error {
	files := []string{}

	clean := filepath.Clean(dir)
	if clean == "." || clean == "" || clean == "/" {
		return errors.New("refusing to clear dangerous path")
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		files = append(files, filepath.Join(clean, e.Name()))
	}

	if d.deleteMode == "wipe" {
		return d.clearWithWipe(ctx, files)
	}

	return d.clearWithRm(ctx, files)
}

func (d *SafeDeleter) clearWithRm(ctx context.Context, files []string) error {
	for _, target := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
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

func (d *SafeDeleter) clearWithWipe(ctx context.Context, files []string) error {
	if d.dryRun {
		d.log.Warn().Str("paths", strings.Join(files, " ")).Msg("dry-run enabled: would delete entry")
		return nil
	}

	args := buildWipeArgs(d.wipeArgs, d.logLevel, files)
	d.log.Info().Str("args", strings.Join(args, " ")).Msg("Launching wipe")
	out, err := d.runner(ctx, "wipe", args...)
	if err != nil {
		d.log.Error().Err(err).Str("output", string(out)).Msg("wipe command failed")
		return fmt.Errorf("wipe failed: %w", err)
	}
	d.log.Debug().Str("output", string(out)).Msg("wipe command runned")
	d.log.Debug().Msg("wiped directory")
	return nil
}

func buildWipeArgs(configuredArgs string, logLevel string, files []string) []string {
	args := strings.Fields(strings.TrimSpace(configuredArgs))
	args = append(args, "-c", "-r", "-f")
	if strings.ToLower(strings.TrimSpace(logLevel)) != "debug" && !containsArg(args, "-s") {
		args = append(args, "-s")
	}
	args = append(args, files...)
	return args
}

func containsArg(args []string, needle string) bool {
	for _, arg := range args {
		if arg == needle {
			return true
		}
	}
	return false
}

func runCommand(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	return cmd.CombinedOutput()
}
