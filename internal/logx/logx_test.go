package logx

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_WritesConsoleToStdoutAndJSONToFile(t *testing.T) {
	dir := t.TempDir()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	logger, closer, err := New("info", dir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if closer == nil {
		t.Fatal("expected file closer when log path is set")
	}

	logger.Info().Msg("visible message")
	logger.Debug().Msg("hidden message")

	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close file logger: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close stdout pipe writer: %v", err)
	}

	stdoutBytes, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read log directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one log file, got %d", len(entries))
	}

	logPath := filepath.Join(dir, entries[0].Name())
	fileBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	stdout := strings.TrimSpace(string(stdoutBytes))
	if stdout == "" {
		t.Fatal("expected stdout output")
	}
	if json.Valid([]byte(stdout)) {
		t.Fatalf("expected human-readable stdout, got JSON: %s", stdout)
	}
	if !strings.Contains(stdout, "visible message") {
		t.Fatalf("expected stdout to contain message, got: %s", stdout)
	}
	if strings.Contains(stdout, "hidden message") {
		t.Fatalf("did not expect filtered debug message on stdout, got: %s", stdout)
	}

	fileLines := bytes.Split(bytes.TrimSpace(fileBytes), []byte("\n"))
	if len(fileLines) != 1 {
		t.Fatalf("expected one JSON log line, got %d: %s", len(fileLines), string(fileBytes))
	}
	if !json.Valid(fileLines[0]) {
		t.Fatalf("expected JSON log output, got: %s", string(fileLines[0]))
	}
	if !strings.Contains(string(fileLines[0]), `"message":"visible message"`) {
		t.Fatalf("expected file log to contain message field, got: %s", string(fileLines[0]))
	}
	if strings.Contains(string(fileLines[0]), "hidden message") {
		t.Fatalf("did not expect filtered debug message in file, got: %s", string(fileLines[0]))
	}
}

func TestNew_RejectsInvalidLevel(t *testing.T) {
	logger, closer, err := New("invalid", "")
	if err == nil {
		t.Fatal("expected an error for invalid log level")
	}
	if !strings.Contains(err.Error(), "invalid log-level") {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = logger
	if closer != nil {
		t.Fatal("expected nil closer on error")
	}
}
