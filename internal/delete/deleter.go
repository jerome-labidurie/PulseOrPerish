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
	"pulseorperish/internal/fscrypt"
	"strings"

	"github.com/rs/zerolog"
)

// Deleter removes all entries inside a directory.
type Deleter interface {
	// ClearDirectory deletes every entry directly inside dir.
	// It returns an error if the directory cannot be read, or ctx.Err() if the
	// context is cancelled before all entries are processed.
	ClearDirectory(ctx context.Context, dir string) error
	ClearDirectories(ctx context.Context, dirs []string) error
}

// Crypter creates encrypted archives for top-level entries before deletion.
type Crypter interface {
	EncryptPaths(filesin []string, fileout string) error
	GetCryptedFileName(idx int) string
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
	crypter    Crypter
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

// SetCrypter injects the archive crypter used by crypt/* delete modes.
func (d *SafeDeleter) SetCrypter(crypter Crypter) *SafeDeleter {
	d.crypter = crypter
	return d
}

// ClearDirectory deletes all entries directly inside a dataDir.
// It returns an error if dir resolves to a dangerous path (empty, ".", or "/"),
// or if the directory listing fails. Individual entry removal errors are logged
// but do not abort the iteration. Returns ctx.Err() if the context is cancelled.
func (d *SafeDeleter) ClearDirectory(ctx context.Context, dir string) error {
	files := []string{}

	clean, err := ResolveSafeDir(dir)
	if err != nil {
		return fmt.Errorf("validate data directory: %w", err)
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		// TODO: if error, still delete already returned entries ?
		return fmt.Errorf("read directory %q: %w", clean, err)
	}
	for _, e := range entries {
		if strings.HasPrefix(d.deleteMode, "crypt/") && isManagedArchiveEntry(e.Name()) {
			d.log.Debug().Str("path", filepath.Join(clean, e.Name())).Msg("skipping existing encrypted archive")
			continue
		}
		files = append(files, filepath.Join(clean, e.Name()))
	}

	if strings.HasPrefix(d.deleteMode, "crypt/") {
		return d.clearWithCrypt(ctx, clean, files)
	}

	if d.deleteMode == "wipe" {
		return d.clearWithWipe(ctx, files)
	}

	return d.clearWithRm(ctx, files)
}

// deletes content of multiples dataDirs
func (d *SafeDeleter) ClearDirectories(ctx context.Context, dirs []string) error {
	for _, dir := range dirs {
		if err := d.ClearDirectory(ctx, dir); err != nil {
			return err
		}
	}
	return nil
}

func (d *SafeDeleter) clearWithCrypt(ctx context.Context, dataDir string, files []string) error {
	if d.crypter == nil {
		return errors.New("crypt delete mode requires a crypter")
	}

	nextArchiveIdx := 0
	for _, target := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		archivePath, archiveIdx, err := d.nextArchivePath(dataDir, nextArchiveIdx)
		if err != nil {
			return err
		}
		nextArchiveIdx = archiveIdx + 1

		if d.dryRun {
			d.log.Warn().Str("path", target).Str("archive", archivePath).Msg("dry-run enabled: would encrypt and delete entry")
			continue
		}

		if err := d.crypter.EncryptPaths([]string{target}, archivePath); err != nil {
			if errors.Is(err, fscrypt.ErrNoFilesToEncrypt) {
				d.log.Debug().Str("path", target).Msg("deleting empty entry without encryption")
				d.deleteTarget(ctx, target)
				continue
			}
			d.log.Error().Err(err).Str("path", target).Str("archive", archivePath).Msg("failed encrypting entry")
			continue
		}
		d.log.Debug().Str("path", target).Str("archive", archivePath).Msg("encrypted entry")

		d.deleteTarget(ctx, target)
	}

	return nil
}

// rm files & dirs from files[]
func (d *SafeDeleter) clearWithRm(ctx context.Context, files []string) error {
	return d.clearTargets(ctx, files, func(target string) { d.deleteWithRm(target) })
}

// wipe files & dirs from files[]
func (d *SafeDeleter) clearWithWipe(ctx context.Context, files []string) error {
	return d.clearTargets(ctx, files, func(target string) { d.deleteWithWipe(ctx, target) })
}

// calls clearFn on each file entry from  files[]
func (d *SafeDeleter) clearTargets(ctx context.Context, files []string, clearFn func(target string)) error {
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
		clearFn(target)
	}
	return nil
}

func (d *SafeDeleter) deleteTarget(ctx context.Context, target string) {
	if d.baseDeleteMode() == "wipe" {
		d.deleteWithWipe(ctx, target)
		return
	}
	d.deleteWithRm(target)
}

func (d *SafeDeleter) deleteWithRm(target string) {
	if err := os.RemoveAll(target); err != nil {
		d.log.Error().Err(err).Str("path", target).Msg("failed deleting entry")
	} else {
		d.log.Debug().Str("path", target).Msg("deleted entry")
	}
}

func (d *SafeDeleter) deleteWithWipe(ctx context.Context, target string) {
	args := buildWipeArgs(d.wipeArgs, d.logLevel, target)
	d.log.Info().Str("args", strings.Join(args, " ")).Msg("Starting wipe")
	out, err := d.runner(ctx, "wipe", args...)
	if err != nil {
		d.log.Error().Err(err).Str("output", string(out)).Msg("wipe command failed")
	}
	d.log.Debug().Str("path", target).Str("output", string(out)).Msg("wipe command executed")
}

func (d *SafeDeleter) baseDeleteMode() string {
	if strings.HasPrefix(d.deleteMode, "crypt/") {
		return strings.TrimPrefix(d.deleteMode, "crypt/")
	}
	return d.deleteMode
}

func (d *SafeDeleter) nextArchivePath(dataDir string, startIdx int) (string, int, error) {
	for idx := startIdx; ; idx++ {
		archivePath := filepath.Join(dataDir, d.crypter.GetCryptedFileName(idx))
		_, err := os.Stat(archivePath)
		if err == nil {
			continue
		}
		if os.IsNotExist(err) {
			return archivePath, idx, nil
		}
		return "", 0, fmt.Errorf("stat archive path %q: %w", archivePath, err)
	}
}

func isManagedArchiveEntry(name string) bool {
	return strings.HasPrefix(name, "file_") && strings.HasSuffix(name, fscrypt.FileExtension)
}

func buildWipeArgs(configuredArgs string, logLevel string, file string) []string {
	args := strings.Fields(strings.TrimSpace(configuredArgs))
	args = append(args, "-c", "-r", "-f")
	if strings.ToLower(strings.TrimSpace(logLevel)) != "debug" && !containsArg(args, "-s") {
		args = append(args, "-s")
	}
	args = append(args, file)
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
