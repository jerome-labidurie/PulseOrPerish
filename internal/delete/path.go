package delete

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSafeDir validates and resolves a directory path to a canonical path.
// It rejects dangerous paths and returns the resolved target directory.
func ResolveSafeDir(dir string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(dir))
	if clean == "." || clean == "" || clean == "/" {
		return "", fmt.Errorf("refusing dangerous path %q", dir)
	}

	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", clean, err)
	}
	resolved = filepath.Clean(resolved)
	if resolved == "." || resolved == "" || resolved == "/" {
		return "", fmt.Errorf("refusing dangerous resolved path %q", resolved)
	}

	st, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat resolved path %q: %w", resolved, err)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("resolved path %q is not a directory", resolved)
	}

	return resolved, nil
}
