package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var (
	buildOnce       sync.Once
	builtBinaryPath string
	builtBinaryErr  error
)

// findProjectRoot searches for the go.mod file starting from the current
// working directory and moving up the directory tree.
func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for go.mod
	current := cwd
	for {
		goModPath := filepath.Join(current, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached root directory
			return "", fmt.Errorf("go.mod not found")
		}
		current = parent
	}
}

// AppProcess represents a running instance of the PulseOrPerish application.
type AppProcess struct {
	cmd        *exec.Cmd
	stdout     *bytes.Buffer
	stderr     *bytes.Buffer
	listenAddr string
	t          *testing.T
}

// StartApp starts the PulseOrPerish application as a separate process.
// It returns an error if the build or startup fails.
func StartApp(t *testing.T, listenAddr, dataDir, stateDir, password string, interval time.Duration) (*AppProcess, error) {
	t.Helper()
	return startAppWithOptions(t, listenAddr, dataDir, stateDir, password, interval, false, "", "")
}

// StartAppWithDryRun starts the application with the dry-run flag enabled.
func StartAppWithDryRun(t *testing.T, listenAddr, dataDir, stateDir, password string, interval time.Duration) (*AppProcess, error) {
	t.Helper()
	return startAppWithOptions(t, listenAddr, dataDir, stateDir, password, interval, true, "", "")
}

// StartAppWithDeleteMethod starts the application with an explicit delete method and optional wipe args.
func StartAppWithDeleteMethod(t *testing.T, listenAddr, dataDir, stateDir, password string, interval time.Duration, deleteMethod, wipeArgs string) (*AppProcess, error) {
	t.Helper()
	return startAppWithOptions(t, listenAddr, dataDir, stateDir, password, interval, false, deleteMethod, wipeArgs)
}

// ensureBuiltBinary builds the app binary once and returns its cached path.
func ensureBuiltBinary() (string, error) {
	buildOnce.Do(func() {
		projectRoot, err := findProjectRoot()
		if err != nil {
			builtBinaryErr = fmt.Errorf("failed to find project root: %w", err)
			return
		}

		tmpDir, err := os.MkdirTemp("", "pulseorperish-e2e-bin-*")
		if err != nil {
			builtBinaryErr = fmt.Errorf("failed to create temp dir for binary: %w", err)
			return
		}

		binaryPath := filepath.Join(tmpDir, "pulseorperish")
		buildCmd := exec.CommandContext(context.Background(), "go", "build", "-o", binaryPath, "./cmd/pulseorperish")
		buildCmd.Dir = projectRoot

		buildOut := &bytes.Buffer{}
		buildErr := &bytes.Buffer{}
		buildCmd.Stdout = buildOut
		buildCmd.Stderr = buildErr

		if err := buildCmd.Run(); err != nil {
			errMsg := buildErr.String()
			if errMsg == "" {
				errMsg = buildOut.String()
			}
			builtBinaryErr = fmt.Errorf("failed to build app: %w\nbuild output:\n%s", err, errMsg)
			return
		}

		builtBinaryPath = binaryPath
	})

	if builtBinaryErr != nil {
		return "", builtBinaryErr
	}
	return builtBinaryPath, nil
}

// startAppWithOptions starts the app process with optional dry-run mode.
func startAppWithOptions(t *testing.T, listenAddr, dataDir, stateDir, password string, interval time.Duration, dryRun bool, deleteMethod, wipeArgs string) (*AppProcess, error) {
	t.Helper()

	binaryPath, err := ensureBuiltBinary()
	if err != nil {
		return nil, err
	}

	// Prepare stdout and stderr
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Build the command
	cmd := exec.Command(binaryPath,
		"-listen", listenAddr,
		"-data-dirs", dataDir,
		"-state-dir", stateDir,
		"-password", password,
		"-interval", interval.String(),
	)
	if dryRun {
		cmd.Args = append(cmd.Args, "-dry-run")
	}
	if deleteMethod != "" {
		cmd.Args = append(cmd.Args, "-delete-method", deleteMethod)
	}
	if wipeArgs != "" {
		cmd.Args = append(cmd.Args, "-wipe-args", wipeArgs)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start app: %w", err)
	}

	proc := &AppProcess{
		cmd:        cmd,
		stdout:     stdout,
		stderr:     stderr,
		listenAddr: listenAddr,
		t:          t,
	}

	// Wait for the app to be ready
	if err := proc.WaitForReady(context.Background(), 10*time.Second); err != nil {
		_ = proc.Stop()
		return nil, fmt.Errorf("app did not become ready: %w", err)
	}

	return proc, nil
}

// WaitForReady waits until the app responds to a health check.
// It returns an error if the timeout is exceeded or the app fails.
func (p *AppProcess) WaitForReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	backoff := 100 * time.Millisecond

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for app readiness")
		}

		resp, err := http.Get(fmt.Sprintf("http://%s/health", p.listenAddr))
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}

		// Check if process has exited
		if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			return fmt.Errorf("app process exited unexpectedly: %s", p.stderr.String())
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Continue retry
		}

		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}

// Stop gracefully shuts down the app process.
func (p *AppProcess) Stop() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// Send SIGTERM for graceful shutdown
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		// Process might already be dead
		p.cmd.Process.Kill()
	}

	// Wait for process to exit with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		p.cmd.Process.Kill()
		<-done
		return fmt.Errorf("process did not exit gracefully")
	}
}

// Stdout returns the accumulated stdout output from the app.
func (p *AppProcess) Stdout() string {
	return p.stdout.String()
}

// WaitForStdoutContains waits until stdout contains at least one of the
// provided substrings, or returns when ctx is done.
func (p *AppProcess) WaitForStdoutContains(ctx context.Context, substrings ...string) error {
	if len(substrings) == 0 {
		return fmt.Errorf("at least one substring is required")
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		out := p.stdout.String()
		for _, s := range substrings {
			if s != "" && bytes.Contains([]byte(out), []byte(s)) {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Stderr returns the accumulated stderr output from the app.
func (p *AppProcess) Stderr() string {
	return p.stderr.String()
}

// ListenAddr returns the HTTP listen address of the app.
func (p *AppProcess) ListenAddr() string {
	return p.listenAddr
}
