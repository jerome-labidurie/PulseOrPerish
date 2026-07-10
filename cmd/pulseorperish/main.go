// Package main contains the PulseOrPerish application entrypoint.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"pulseorperish/internal/config"
	"pulseorperish/internal/delete"
	"pulseorperish/internal/httpapi"
	"pulseorperish/internal/logx"
	"pulseorperish/internal/monitor"
	"pulseorperish/internal/state"

	"github.com/rs/zerolog"
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

	st := state.NewStore(cfg.StateDir)
	del := delete.NewSafeDeleter(logger, cfg.DryRun)
	mon := monitor.NewService(logger, st, del, cfg.Interval, cfg.DryRun, cfg.DataDir)

	// Validate write/delete permissions on dataDir
	if err := validateDataDirPermissions(logger, cfg.DataDir); err != nil {
		logger.Fatal().Err(err).Str("dataDir", cfg.DataDir).Msg("insufficient permissions on data directory")
	}

	if err := mon.LoadInitialState(); err != nil {
		logger.Fatal().Err(err).Msg("failed loading state")
	}

	srv := httpapi.NewServer(logger, cfg.Password, mon)
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

	logger.Info().Str("listen", cfg.ListenAddr).Dur("interval", cfg.Interval).Dur("tick", mon.Tick()).Bool("dryRun", cfg.DryRun).Str("dataDir", cfg.DataDir).Str("stateDir", cfg.StateDir).Msg("PulseOrPerish started")
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal().Err(err).Msg("http server failed")
	}
}
