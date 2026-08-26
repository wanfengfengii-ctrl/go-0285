// Package persistence implements the append-log + atomic-snapshot persistence
// boundary. State is committed through conditional writes guarded by an
// optimistic version; the event log is a checksummed, sequence-framed append
// log that is independently replayed and verified on startup so corruption,
// truncation and sequence gaps surface as PERSISTENCE_CORRUPT instead of being
// silently skipped.
package persistence

import "precast-wall-grout-support-release/domain"

// EventEnvelope is the durable framing of a single committed evidence record:
// its sequence number, payload length and checksum protect against truncation
// and corruption during replay.
type EventEnvelope struct {
	Sequence   uint64
	Length     int
	Checksum   uint32
	TaskID     domain.TaskID
	Generation domain.Generation
	Event      domain.EvidenceEvent
}

// Health summarizes event log, snapshot and recovery state for /healthz and
// /readyz.
type Health struct {
	Ready        bool
	Recovered    bool
	Corrupt      bool
	LogPath      string
	SnapshotPath string
}

// Repository is the persistence boundary implemented by the file-backed event
// log plus atomic snapshot store and the in-memory test store. Save performs a
// conditional write: when expectedVersion does not match the committed version
// the caller read, it returns a CONCURRENT_MODIFICATION error and changes
// nothing.
type Repository interface {
	Load() (*State, error)
	Save(state *State, expectedVersion int64) error
	Health() Health
	Close() error
}
