package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSaveLoad(t *testing.T) {
	d := t.TempDir()
	s := NewStore(d)
	now := time.Now().UTC().Truncate(time.Second)
	if err := s.Save(HeartbeatState{LastProofAt: now, UpdatedBy: "test"}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !got.LastProofAt.Equal(now) {
		t.Fatalf("unexpected last proof: %v", got.LastProofAt)
	}
}

func TestMissingStateReturnsZero(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "state"))
	st, err := s.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !st.LastProofAt.IsZero() {
		t.Fatal("expected zero lastProofAt")
	}
}

func TestSaveAlwaysSetsVersionOne(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Save(HeartbeatState{LastProofAt: time.Now().UTC()}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if st.Version != 1 {
		t.Fatalf("expected version 1, got %d", st.Version)
	}
}

func TestLoadMigratesVersionZeroToOne(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	// Write a state file without a version field (simulates old data).
	raw := map[string]any{"lastProofAt": time.Now().UTC(), "updatedBy": "legacy"}
	b, _ := json.Marshal(raw)
	if err := os.WriteFile(filepath.Join(dir, "heartbeat_state.json"), b, 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if st.Version != 1 {
		t.Fatalf("expected version migrated to 1, got %d", st.Version)
	}
}
