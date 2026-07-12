// Package monitor implements the core proof-of-life loop: it tracks the last
// accepted heartbeat and triggers directory deletion when the deadline is exceeded.
package monitor

import (
	"context"
	"sync"
	"time"

	"pulseorperish/internal/delete"
	"pulseorperish/internal/state"

	"github.com/rs/zerolog"
)

// Status is a point-in-time snapshot of the monitor state, suitable for
// JSON serialisation in API responses.
type Status struct {
	LastProofAt   time.Time `json:"lastProofAt"`
	NextDeletion  time.Time `json:"nextDeletion"`
	TimeRemaining string    `json:"timeRemaining"`
	Overdue       bool      `json:"overdue"`
	DryRun        bool      `json:"dryRun"`
}

// Service orchestrates the heartbeat deadline enforcement. It persists proofs
// of life to a Store and periodically evaluates whether the deadline has been
// missed. All exported methods are safe for concurrent use.
type Service struct {
	log       zerolog.Logger
	store     *state.Store
	deleter   delete.Deleter
	startedAt time.Time
	interval  time.Duration
	dryRun    bool
	dataDir   string
	tick      time.Duration

	mu            sync.RWMutex
	lastProofAt   time.Time
	deleteArmedAt time.Time
}

// NewService creates a Service. The evaluation tick is derived from interval
// (interval/10, clamped between 1 minute and 24 hours).
func NewService(log zerolog.Logger, st *state.Store, d delete.Deleter, interval time.Duration, dryRun bool, dataDir string) *Service {
	tick := interval / 10
	if tick < time.Minute {
		tick = time.Minute
	}
	if tick > 24*time.Hour {
		tick = 24 * time.Hour
	}
	return &Service{
		log:       log,
		store:     st,
		deleter:   d,
		startedAt: time.Now().UTC(),
		interval:  interval,
		dryRun:    dryRun,
		dataDir:   dataDir,
		tick:      tick,
	}
}

// LoadInitialState reads the persisted heartbeat state from the store and
// initialises the in-memory fields. It must be called once before Run.
func (s *Service) LoadInitialState() error {
	st, err := s.store.Load()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.lastProofAt = st.LastProofAt.UTC()
	s.deleteArmedAt = time.Time{}
	s.mu.Unlock()
	return nil
}

// RegisterProof records a new proof of life from the given source, persists it
// to the store, and resets the deletion arm. It returns the timestamp of the
// accepted proof.
func (s *Service) RegisterProof(source string) (time.Time, error) {
	now := time.Now().UTC()
	st := state.HeartbeatState{LastProofAt: now, UpdatedBy: source}
	if err := s.store.Save(st); err != nil {
		return time.Time{}, err
	}

	s.mu.Lock()
	s.lastProofAt = now
	s.deleteArmedAt = time.Time{}
	s.mu.Unlock()

	s.log.Info().Str("source", source).Time("lastProofAt", now).Msg("proof accepted")
	return now, nil
}

// Snapshot returns a Status computed at the given instant.
func (s *Service) Snapshot(now time.Time) Status {
	s.mu.RLock()
	last := s.lastProofAt
	s.mu.RUnlock()

	next := s.deadlineFrom(last)
	remaining := next.Sub(now)
	overdue := remaining <= 0
	if overdue {
		remaining = 0
	}

	return Status{
		LastProofAt:   last,
		NextDeletion:  next,
		TimeRemaining: remaining.String(),
		Overdue:       overdue,
		DryRun:        s.dryRun,
	}
}

// Tick returns the interval between consecutive deadline evaluations.
func (s *Service) Tick() time.Duration {
	return s.tick
}

// Run starts the monitoring loop, evaluating the deadline at each tick until
// ctx is cancelled. It is intended to run in its own goroutine.
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UTC()
			s.evaluate(ctx, now)
		}
	}
}

func (s *Service) evaluate(ctx context.Context, now time.Time) {
	status := s.Snapshot(now)
	s.log.Debug().Time("tickAt", now).Time("nextDeletion", status.NextDeletion).Msg("monitor tick")

	if !status.Overdue {
		return
	}

	s.mu.Lock()
	alreadyTriggered := !s.deleteArmedAt.IsZero() && s.deleteArmedAt.Equal(status.NextDeletion)
	if !alreadyTriggered {
		s.deleteArmedAt = status.NextDeletion
	}
	s.mu.Unlock()
	if alreadyTriggered {
		return
	}

	s.log.Warn().Time("deadline", status.NextDeletion).Str("dataDir", s.dataDir).Msg("deadline exceeded, clearing directory")
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.deleter.ClearDirectory(cctx, s.dataDir); err != nil {
		s.log.Error().Err(err).Str("dataDir", s.dataDir).Msg("directory clearing failed")
		return
	}
	s.log.Warn().Str("dataDir", s.dataDir).Msg("directory content cleared")
}

func (s *Service) deadlineFrom(last time.Time) time.Time {
	if last.IsZero() {
		return s.startedAt.Add(s.interval)
	}
	return last.Add(s.interval)
}
