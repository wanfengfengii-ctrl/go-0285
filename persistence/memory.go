package persistence

import (
	"sync"

	"precast-wall-grout-support-release/domain"
)

// MemoryRepository is an in-memory Repository used by tests and by the
// foundation before a file store is wired. It enforces the same optimistic
// version contract as the file store so callers exercise identical semantics.
type MemoryRepository struct {
	mu       sync.Mutex
	state    *State
	corrupt  bool
	logPath  string
	snapPath string
}

// NewMemoryStore returns an empty, ready MemoryRepository for the given paths.
func NewMemoryStore(logPath, snapPath string) *MemoryRepository {
	return &MemoryRepository{state: NewState(), logPath: logPath, snapPath: snapPath}
}

// Load returns a deep copy of the committed state.
func (m *MemoryRepository) Load() (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.corrupt {
		return nil, domain.NewError(domain.CodePersistenceCorrupt, "store is corrupt")
	}
	return m.state.Clone(), nil
}

// Save commits the state when expectedVersion matches the committed version.
func (m *MemoryRepository) Save(state *State, expectedVersion int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state.Version != expectedVersion+1 {
		return domain.NewError(domain.CodeConcurrentModification, "state version mismatch")
	}
	if m.state.Version != expectedVersion {
		return domain.NewError(domain.CodeConcurrentModification, "concurrent modification detected")
	}
	m.state = state.Clone()
	return nil
}

// Health reports the current health summary.
func (m *MemoryRepository) Health() Health {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Health{
		Ready:        !m.corrupt,
		Recovered:    true,
		Corrupt:      m.corrupt,
		LogPath:      m.logPath,
		SnapshotPath: m.snapPath,
	}
}

// MarkCorrupt flags the store corrupt for recovery tests.
func (m *MemoryRepository) MarkCorrupt() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.corrupt = true
}

// Close resets the store.
func (m *MemoryRepository) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = NewState()
	return nil
}
