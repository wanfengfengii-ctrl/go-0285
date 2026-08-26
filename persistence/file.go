package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"precast-wall-grout-support-release/domain"
)

// FileRepository persists State to an atomic snapshot file and appends framed
// evidence envelopes to a checksummed event log. It is the production store:
// Load replays and verifies the log on startup, and Save persists both the log
// and an atomic snapshot.
type FileRepository struct {
	mu       sync.Mutex
	logPath  string
	snapPath string
	version  int64
	frames   int
	corrupt  bool
	loaded   bool
}

// NewFileRepository opens a file-backed repository at the given paths and
// immediately loads and verifies the persisted state.
func NewFileRepository(logPath, snapPath string) (*FileRepository, error) {
	r := &FileRepository{logPath: logPath, snapPath: snapPath}
	if err := r.recover(); err != nil {
		return nil, err
	}
	return r, nil
}

// Load returns a deep copy of the committed state, verifying the event log on
// the first call after recovery.
func (r *FileRepository) Load() (*State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.corrupt {
		return nil, domain.NewError(domain.CodePersistenceCorrupt, "store is corrupt")
	}
	st, err := r.readSnapshot()
	if err != nil {
		return nil, err
	}
	return st, nil
}

// Save commits state when expectedVersion matches the committed version. It
// persists any new evidence envelopes and publishes the new snapshot.
func (r *FileRepository) Save(state *State, expectedVersion int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.corrupt {
		return domain.NewError(domain.CodePersistenceCorrupt, "store is corrupt")
	}
	if state.Version != expectedVersion+1 {
		return domain.NewError(domain.CodeConcurrentModification, "state version mismatch")
	}
	if r.version != expectedVersion {
		return domain.NewError(domain.CodeConcurrentModification, "concurrent modification detected")
	}
	if len(state.Evidence) < r.frames {
		return domain.NewError(domain.CodeConcurrentModification, "evidence log shrank")
	}
	if err := r.writeSnapshot(state); err != nil {
		return err
	}
	if err := r.appendEvidence(state.Evidence[r.frames:]); err != nil {
		return err
	}
	r.version = state.Version
	r.frames = len(state.Evidence)
	return nil
}

// Health reports readiness and recovery state.
func (r *FileRepository) Health() Health {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Health{
		Ready:        r.loaded && !r.corrupt,
		Recovered:    r.loaded,
		Corrupt:      r.corrupt,
		LogPath:      r.logPath,
		SnapshotPath: r.snapPath,
	}
}

// Close releases the repository (no-op for files, kept for interface parity).
func (r *FileRepository) Close() error {
	return nil
}

// recover loads the snapshot, replays and verifies the log, and truncates any
// uncommitted trailing log frames. Any corruption marks the store unready.
func (r *FileRepository) recover() error {
	st, err := r.readSnapshot()
	if err != nil {
		return err
	}
	committed := len(st.Evidence)
	// Verify committed log frames against the snapshot.
	if err := r.verifyLog(committed, st.Evidence); err != nil {
		r.corrupt = true
		r.loaded = true
		return nil // store is open but unready; PERSISTENCE_CORRUPT surfaces via Load.
	}
	r.version = st.Version
	r.frames = committed
	r.loaded = true
	return nil
}

func (r *FileRepository) readSnapshot() (*State, error) {
	data, err := os.ReadFile(r.snapPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewState(), nil
		}
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("snapshot corrupt: %w", err)
	}
	if st.Tasks == nil {
		return nil, fmt.Errorf("snapshot corrupt: nil maps")
	}
	return &st, nil
}

func (r *FileRepository) verifyLog(committed int, evidence []domain.EvidenceEvent) error {
	data, err := os.ReadFile(r.logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No log yet: only valid when no evidence has been committed.
			if committed != 0 {
				return fmt.Errorf("event log missing but %d evidence events committed", committed)
			}
			return nil
		}
		return err
	}
	offset := 0
	var prevSeq uint64
	for i := 0; offset < len(data); i++ {
		env, n, err := codec.Decode(data, offset)
		if err != nil {
			// Trailing partial frame may be an uncommitted tail; if beyond
			// committed evidence it is acceptable.
			if i >= committed {
				return r.truncateLog(offset)
			}
			return fmt.Errorf("event log corrupt at frame %d: %w", i, err)
		}
		if i > 0 && env.Sequence != prevSeq+1 {
			return fmt.Errorf("event log sequence gap at frame %d", i)
		}
		if int(env.Sequence) > committed {
			// Uncommitted trailing frame: truncate the log tail.
			return r.truncateLog(offset)
		}
		if evidence[env.Sequence-1].ID != env.Event.ID {
			return fmt.Errorf("event log diverges from snapshot at sequence %d", env.Sequence)
		}
		prevSeq = env.Sequence
		offset += n
	}
	if int(prevSeq) != committed {
		return fmt.Errorf("event log has %d frames but %d evidence committed", prevSeq, committed)
	}
	return nil
}

func (r *FileRepository) truncateLog(keepBytes int) error {
	return os.Truncate(r.logPath, int64(keepBytes))
}

func (r *FileRepository) appendEvidence(events []domain.EvidenceEvent) error {
	if len(events) == 0 {
		return nil
	}
	f, err := os.OpenFile(r.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for i, ev := range events {
		seq := uint64(r.frames + i + 1)
		env := EventEnvelope{
			Sequence:   seq,
			Length:     len(ev.ContentDigest),
			TaskID:     ev.TaskID,
			Generation: ev.Generation,
			Event:      ev,
		}
		frame, err := codec.Encode(env)
		if err != nil {
			return err
		}
		if _, err := f.Write(frame); err != nil {
			return err
		}
	}
	return f.Sync()
}

func (r *FileRepository) writeSnapshot(st *State) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := r.snapPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	f, err := os.OpenFile(tmp, os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if err := os.Rename(tmp, r.snapPath); err != nil {
		return err
	}
	return syncDir(filepath.Dir(r.snapPath))
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
