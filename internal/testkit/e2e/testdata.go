package e2e

import (
	"testing"
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
