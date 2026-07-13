package e2e

import (
	"testing"

	"pulseorperish/internal/testkit/fshelpers"
)

// TestEnv holds temporary directories for a test run.
type TestEnv struct {
	DataDir  string
	StateDir string
}

// SetupTestEnv creates isolated temporary directories for test data.
// The returned cleanup function removes all temporary files when called.
func SetupTestEnv(t *testing.T) TestEnv {
	t.Helper()

	dataDir := t.TempDir()
	stateDir := t.TempDir()

	return TestEnv{
		DataDir:  dataDir,
		StateDir: stateDir,
	}
}

// CreateTestFile creates a single file in the given directory with dummy content.
// Returns the full path to the created file.
func CreateTestFile(t *testing.T, dir, name string) string {
	return fshelpers.CreateTestFile(t, dir, name)
}

// CreateTestFiles creates multiple test files in the given directory.
// Returns the list of full paths to the created files.
func CreateTestFiles(t *testing.T, dir string, count int) []string {
	return fshelpers.CreateTestFiles(t, dir, count)
}
