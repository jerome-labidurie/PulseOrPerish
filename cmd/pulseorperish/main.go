// Package main contains the PulseOrPerish application entrypoint.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"pulseorperish/internal/config"
	"pulseorperish/internal/delete"
	"pulseorperish/internal/fscrypt"
	"pulseorperish/internal/httpapi"
	"pulseorperish/internal/logx"
	"pulseorperish/internal/monitor"
	"pulseorperish/internal/state"

	"github.com/rs/zerolog"
)

// Version, BuildDate, and CommitHash are set during build via ldflags.
var (
	Version    = "dev"
	BuildDate  = "unknown"
	CommitHash = "unknown"
)

// validateDataDirPermissions verifies that the process can create and remove a
// file inside dataDir. It returns an error if either operation fails, allowing
// the program to fail fast before the first deletion attempt.
func validateDataDirPermissions(log zerolog.Logger, dataDir string) error {
	// Create test file to verify write and delete permissions
	testFile := filepath.Join(dataDir, ".pulseorperish-test")

	// Try to write
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		return fmt.Errorf("cannot write to data directory: %w", err)
	}

	// Try to delete
	if err := os.Remove(testFile); err != nil {
		return fmt.Errorf("cannot delete from data directory: %w", err)
	}

	log.Debug().Str("dataDir", dataDir).Msg("write/delete permissions verified on data directory")
	return nil
}

// main wires configuration, logging, monitoring and the HTTP server lifecycle.
func main() {
	// Handle -version flag early
	for _, arg := range os.Args[1:] {
		if arg == "-version" {
			fmt.Printf("PulseOrPerish version %s (built %s, commit %s)\n", Version, BuildDate, CommitHash)
			os.Exit(0)
		}
	}

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	logger, closer, err := logx.New(cfg.LogLevel, cfg.LogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger error: %v\n", err)
		os.Exit(2)
	}
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}

	var safeDataDirs []string
	for _, dir := range cfg.DataDirs {
		resolvedDataDir, err := delete.ResolveSafeDir(dir)
		if err != nil {
			logger.Fatal().Err(err).Str("dataDir", dir).Str("resolvedTo", resolvedDataDir).Msg("invalid data directory")
		}
		// Validate write/delete permissions on dataDir
		if err := validateDataDirPermissions(logger, resolvedDataDir); err != nil {
			logger.Fatal().Err(err).Str("dataDir", dir).Str("resolvedTo", resolvedDataDir).Msg("insufficient permissions on data directory")
		}
		safeDataDirs = append(safeDataDirs, resolvedDataDir)
	}
	cfg.DataDirs = safeDataDirs

	st := state.NewStore(cfg.StateDir)
	if cfg.DeleteMode == "wipe" || cfg.DeleteMode == "crypt/wipe" {
		if _, err := exec.LookPath("wipe"); err != nil {
			logger.Fatal().Err(err).Str("deleteMethod", cfg.DeleteMode).Msg("wipe-based delete method requires wipe binary to be installed")
		}
	}
	del := delete.NewSafeDeleter(logger, cfg.DryRun, cfg.DeleteMode, cfg.WipeArgs, cfg.LogLevel)
	if strings.HasPrefix(cfg.DeleteMode, "crypt/") {
		crypter := &fscrypt.FsCrypt{
			Password: []byte(cfg.CryptPassword),
			Compress: "gz",
		}
		crypter.Init()
		defer crypter.Clear()
		del.SetCrypter(crypter)
	}
	mon := monitor.NewService(logger, st, del, cfg.Interval, cfg.DryRun, cfg.DataDirs)

	if err := mon.LoadInitialState(); err != nil {
		logger.Fatal().Err(err).Msg("failed loading state")
	}

	srv := httpapi.NewServer(logger, cfg.Password, mon, Version, BuildDate, CommitHash)
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go mon.Run(ctx)
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(sctx)
	}()

	logger.Info().Str("version", Version).Str("buildDate", BuildDate).Str("commit", CommitHash).Dur("tick", mon.Tick()).Interface("config", cfg.Redacted()).Msg("PulseOrPerish started")
	status := mon.Snapshot(time.Now().UTC())
	logger.Info().Time("nextDeletion", status.NextDeletion).Msg("Monitor started")

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal().Err(err).Msg("http server failed")
	}
}
