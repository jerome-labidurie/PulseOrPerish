package state

import (
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
