package monitor

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pulseorperish/internal/delete"
	"pulseorperish/internal/state"

	"github.com/rs/zerolog"
)

type flakyDeleter struct {
	calls int
	err   error
}

func (f *flakyDeleter) ClearDirectories(ctx context.Context, dirs []string) error {
	f.calls++
	if f.calls == 1 {
		return f.err
	}
	return nil
}

func (f *flakyDeleter) ClearDirectory(ctx context.Context, dir string) error {
	return f.ClearDirectories(ctx, []string{dir})
}

func TestRegisterProofUpdatesStatus(t *testing.T) {
	d := t.TempDir()
	st := state.NewStore(filepath.Join(d, "state"))
	del := delete.NewSafeDeleter(zerolog.Nop(), false, "rm", "", "info")
	svc := NewService(zerolog.Nop(), st, del, time.Minute, false, []string{filepath.Join(d, "data")})
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
	del := delete.NewSafeDeleter(zerolog.Nop(), false, "rm", "", "info")
	svc := NewService(zerolog.Nop(), st, del, time.Second, false, []string{filepath.Join(d, "data")})
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
	del := delete.NewSafeDeleter(zerolog.Nop(), false, "rm", "", "info")
	svc := NewService(zerolog.Nop(), st, del, time.Minute, false, []string{filepath.Join(d, "data")})
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

func TestLoadInitialStatePersistsStartupProofAcrossRestart(t *testing.T) {
	d := t.TempDir()
	storePath := filepath.Join(d, "state")
	del := delete.NewSafeDeleter(zerolog.Nop(), false, "rm", "", "info")

	first := NewService(zerolog.Nop(), state.NewStore(storePath), del, time.Minute, false, []string{filepath.Join(d, "data")})
	if err := first.LoadInitialState(); err != nil {
		t.Fatal(err)
	}

	firstSnap := first.Snapshot(first.startedAt)
	if firstSnap.LastProofAt.IsZero() {
		t.Fatal("expected startup proof to be persisted")
	}

	// Simulate a restart with a new service instance.
	second := NewService(zerolog.Nop(), state.NewStore(storePath), del, time.Minute, false, []string{filepath.Join(d, "data")})
	if err := second.LoadInitialState(); err != nil {
		t.Fatal(err)
	}

	secondSnap := second.Snapshot(second.startedAt)
	if !secondSnap.LastProofAt.Equal(firstSnap.LastProofAt) {
		t.Fatalf("expected persisted last proof across restart: first=%s second=%s", firstSnap.LastProofAt, secondSnap.LastProofAt)
	}

	wantNextDeletion := firstSnap.LastProofAt.Add(time.Minute)
	if !secondSnap.NextDeletion.Equal(wantNextDeletion) {
		t.Fatalf("expected stable next deletion from persisted proof: got=%s want=%s", secondSnap.NextDeletion, wantNextDeletion)
	}
}

func TestEvaluateRetriesAfterFailedDeletion(t *testing.T) {
	d := t.TempDir()
	st := state.NewStore(filepath.Join(d, "state"))
	deleter := &flakyDeleter{err: errors.New("temporary failure")}
	svc := NewService(zerolog.Nop(), st, deleter, time.Minute, false, []string{filepath.Join(d, "data")})
	if err := svc.LoadInitialState(); err != nil {
		t.Fatal(err)
	}

	svc.mu.Lock()
	svc.lastProofAt = time.Now().UTC().Add(-2 * time.Minute)
	svc.deleteArmedAt = time.Time{}
	svc.mu.Unlock()

	now := time.Now().UTC()
	svc.evaluate(context.Background(), now)
	if deleter.calls != 1 {
		t.Fatalf("expected one deletion attempt after first failure, got %d", deleter.calls)
	}

	svc.evaluate(context.Background(), now)
	if deleter.calls != 2 {
		t.Fatalf("expected retry after failed deletion, got %d calls", deleter.calls)
	}

	svc.mu.RLock()
	armed := svc.deleteArmedAt
	svc.mu.RUnlock()
	if armed.IsZero() {
		t.Fatal("expected deletion to be armed after successful retry")
	}
}
