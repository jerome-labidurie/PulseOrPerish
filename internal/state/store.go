// Package state manages the persistent heartbeat state used to track the last
// accepted proof of life.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const fileName = "heartbeat_state.json"

// HeartbeatState is the JSON-serialisable state persisted to disk.
type HeartbeatState struct {
	Version     int       `json:"version"`
	LastProofAt time.Time `json:"lastProofAt"`
	UpdatedBy   string    `json:"updatedBy"`
}

// Store persists and loads HeartbeatState as a JSON file under a directory.
// All operations are safe for concurrent use.
type Store struct {
	dir  string
	path string
	mu   sync.Mutex
}

// NewStore returns a Store that persists state inside dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir, path: filepath.Join(dir, fileName)}
}

// Load reads the persisted state from disk.
// If no state file exists yet, it returns a zero HeartbeatState with Version 1.
func (s *Store) Load() (HeartbeatState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return HeartbeatState{}, err
	}

	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return HeartbeatState{Version: 1}, nil
	}
	if err != nil {
		return HeartbeatState{}, err
	}

	var st HeartbeatState
	if err := json.Unmarshal(b, &st); err != nil {
		return HeartbeatState{}, err
	}
	if st.Version == 0 {
		st.Version = 1
	}
	return st, nil
}

// Save atomically writes st to disk using a temporary file and rename, then
// fsyncs the directory to ensure durability.
func (s *Store) Save(st HeartbeatState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	st.Version = 1
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}

	df, err := os.Open(s.dir)
	if err == nil {
		_ = df.Sync()
		_ = df.Close()
	}
	return nil
}
