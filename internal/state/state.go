package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Standard filesystem locations for state persistence.
const (
	DefaultStateDir  = "/var/lib/arkon"
	DefaultStateFile = "/var/lib/arkon/state.json"
	DefaultLockFile  = "/var/lib/arkon/state.lock"
)

// ErrLockNotFound is returned when attempting to release a lock ID that does not exist.
var ErrLockNotFound = errors.New("inhibition lock not found")

// State holds the runtime persisted state of Arkon.
type State struct {
	Locks []InhibitionLock `json:"locks"`
}

// InhibitionLock represents a temporary override preventing automated power transitions.
type InhibitionLock struct {
	ID        string    `json:"id"`
	Reason    string    `json:"reason"`
	CreatedBy string    `json:"created_by"`
	ExpiresAt time.Time `json:"expires_at"`
	Remaining string    `json:"remaining,omitempty"`
}

// Manager coordinates atomic access and advisory locking for Arkon state.
type Manager struct {
	dir       string
	statePath string
	lockPath  string
	now       func() time.Time
}

// Option customizes Manager behavior.
type Option func(*Manager)

// WithClock overrides the clock function for deterministic time testing.
func WithClock(clock func() time.Time) Option {
	return func(m *Manager) {
		m.now = clock
	}
}

// NewManager creates a state manager for the given directory.
// If dir is empty, DefaultStateDir is used.
func NewManager(dir string, opts ...Option) *Manager {
	if dir == "" {
		dir = DefaultStateDir
	}

	mgr := &Manager{
		dir:       dir,
		statePath: filepath.Join(dir, "state.json"),
		lockPath:  filepath.Join(dir, "state.lock"),
		now:       time.Now,
	}

	for _, opt := range opts {
		opt(mgr)
	}

	return mgr
}

// AcquireLock creates and persists an active inhibition lock.
func (m *Manager) AcquireLock(reason string, duration time.Duration, createdBy string) (*InhibitionLock, error) {
	if duration <= 0 {
		duration = 2 * time.Hour
	}
	if reason == "" {
		reason = "Manual maintenance window"
	}
	if createdBy == "" {
		createdBy = "operator"
	}

	var createdLock *InhibitionLock

	err := m.withFileLock(func() error {
		st, err := m.loadAndPruneState()
		if err != nil {
			return err
		}

		now := m.now()
		baseID := fmt.Sprintf("inh-%s", now.Format("20060102-150405"))
		lockID := baseID
		suffix := 1

		// Guarantee ID uniqueness even when multiple locks are created within one second
		for hasLockID(st.Locks, lockID) {
			lockID = fmt.Sprintf("%s-%d", baseID, suffix)
			suffix++
		}

		expiresAt := now.Add(duration)
		lock := InhibitionLock{
			ID:        lockID,
			Reason:    reason,
			CreatedBy: createdBy,
			ExpiresAt: expiresAt,
			Remaining: FormatRemaining(duration),
		}

		st.Locks = append(st.Locks, lock)

		if err := m.saveStateAtomic(st); err != nil {
			return err
		}

		createdLock = &lock
		return nil
	})

	return createdLock, err
}

// ReleaseLock removes the specified lock ID from runtime state.
func (m *Manager) ReleaseLock(id string) error {
	return m.withFileLock(func() error {
		st, err := m.loadAndPruneState()
		if err != nil {
			return err
		}

		index := -1
		for i, l := range st.Locks {
			if l.ID == id {
				index = i
				break
			}
		}

		if index == -1 {
			return fmt.Errorf("%w: %s", ErrLockNotFound, id)
		}

		st.Locks = append(st.Locks[:index], st.Locks[index+1:]...)
		return m.saveStateAtomic(st)
	})
}

// ListLocks returns all active inhibition locks, pruning any that expired.
func (m *Manager) ListLocks() ([]InhibitionLock, error) {
	var result []InhibitionLock

	err := m.withFileLock(func() error {
		st, err := m.loadAndPruneState()
		if err != nil {
			return err
		}

		now := m.now()
		result = make([]InhibitionLock, len(st.Locks))
		for i, l := range st.Locks {
			l.Remaining = FormatRemaining(l.ExpiresAt.Sub(now))
			result[i] = l
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []InhibitionLock{}
	}
	return result, nil
}

// PruneExpiredLocks removes locks whose expiration timestamp is at or before the given time.
func PruneExpiredLocks(locks []InhibitionLock, now time.Time) ([]InhibitionLock, int) {
	var active []InhibitionLock
	pruned := 0

	for _, l := range locks {
		if !now.Before(l.ExpiresAt) {
			pruned++
		} else {
			active = append(active, l)
		}
	}

	if active == nil {
		active = []InhibitionLock{}
	}
	return active, pruned
}

// loadAndPruneState reads state from disk and automatically cleans up expired locks.
// Must be called with lock held.
func (m *Manager) loadAndPruneState() (*State, error) {
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{Locks: []InhibitionLock{}}, nil
		}
		return nil, fmt.Errorf("failed to read state file %s: %w", m.statePath, err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	if st.Locks == nil {
		st.Locks = []InhibitionLock{}
	}

	now := m.now()
	active, prunedCount := PruneExpiredLocks(st.Locks, now)
	if prunedCount > 0 {
		st.Locks = active
		if err := m.saveStateAtomic(&st); err != nil {
			return nil, err
		}
	}

	return &st, nil
}

func hasLockID(locks []InhibitionLock, id string) bool {
	for _, l := range locks {
		if l.ID == id {
			return true
		}
	}
	return false
}

// FormatRemaining produces a concise human-readable duration string.
func FormatRemaining(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)

	if h > 0 {
		if m > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	if m > 0 {
		if s > 0 {
			return fmt.Sprintf("%dm %ds", m, s)
		}
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", s)
}
