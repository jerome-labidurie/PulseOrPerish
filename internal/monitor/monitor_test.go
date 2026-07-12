package monitor

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"pulseorperish/internal/delete"
	"pulseorperish/internal/state"

	"github.com/rs/zerolog"
)

func TestRegisterProofUpdatesStatus(t *testing.T) {
	d := t.TempDir()
	st := state.NewStore(filepath.Join(d, "state"))
	del := delete.NewSafeDeleter(zerolog.Nop(), false)
	svc := NewService(zerolog.Nop(), st, del, time.Minute, false, filepath.Join(d, "data"))
	if err := svc.LoadInitialState(); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RegisterProof("test"); err != nil {
		t.Fatal(err)
	}

	s := svc.Snapshot(time.Now().UTC())
	if s.LastProofAt.IsZero() {
		t.Fatal("expected last proof to be set")
	}
	if s.Overdue {
		t.Fatal("expected non overdue after proof")
	}
}

func TestRunDoesNotPanicOnCancelledContext(t *testing.T) {
	d := t.TempDir()
	st := state.NewStore(filepath.Join(d, "state"))
	del := delete.NewSafeDeleter(zerolog.Nop(), false)
	svc := NewService(zerolog.Nop(), st, del, time.Second, false, filepath.Join(d, "data"))
	if err := svc.LoadInitialState(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.Run(ctx)
}

func TestSnapshotNextDeletionStableWithoutProof(t *testing.T) {
	d := t.TempDir()
	st := state.NewStore(filepath.Join(d, "state"))
	del := delete.NewSafeDeleter(zerolog.Nop(), false)
	svc := NewService(zerolog.Nop(), st, del, time.Minute, false, filepath.Join(d, "data"))
	if err := svc.LoadInitialState(); err != nil {
		t.Fatal(err)
	}

	first := svc.Snapshot(time.Now().UTC())
	second := svc.Snapshot(time.Now().UTC().Add(5 * time.Second))

	want := svc.startedAt.Add(time.Minute)
	if !first.NextDeletion.Equal(want) {
		t.Fatalf("unexpected first next deletion: got=%s want=%s", first.NextDeletion, want)
	}
	if !second.NextDeletion.Equal(want) {
		t.Fatalf("unexpected second next deletion: got=%s want=%s", second.NextDeletion, want)
	}
}
