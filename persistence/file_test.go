package persistence

import (
	"os"
	"path/filepath"
	"testing"

	"precast-wall-grout-support-release/domain"
)

func sampleState() *State {
	st := NewState()
	st.Version = 1
	st.Tasks[domain.TaskID("T1")] = &domain.InspectionTask{
		ID:                "T1",
		Building:          "B1",
		Level:             "L1",
		WallPanel:         "W1",
		Stage:             domain.StageLocked,
		CurrentGeneration: 0,
		AggregateVersion:  1,
	}
	st.NextEventID = 1
	st.Evidence = append(st.Evidence, domain.EvidenceEvent{
		ID:            "evt-1",
		TaskID:        "T1",
		WallPanel:     "W1",
		Generation:    0,
		Type:          domain.EventMaterialCheck,
		Valid:         true,
		ContentDigest: "abc",
	})
	st.NextSequence = 1
	return st
}

func TestFileRepositoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "events.log")
	snapPath := filepath.Join(dir, "snapshot.bin")

	repo, err := NewFileRepository(logPath, snapPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	st := sampleState()
	if err := repo.Save(st, 0); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := repo.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Evidence) != 1 || loaded.Tasks["T1"] == nil {
		t.Fatalf("state not round-tripped: %+v", loaded)
	}
}

func TestFileRepositoryRecovery(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "events.log")
	snapPath := filepath.Join(dir, "snapshot.bin")

	repo, err := NewFileRepository(logPath, snapPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := repo.Save(sampleState(), 0); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reopen as a fresh process would.
	repo2, err := NewFileRepository(logPath, snapPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	loaded, err := repo2.Load()
	if err != nil {
		t.Fatalf("load after reopen: %v", err)
	}
	if len(loaded.Evidence) != 1 {
		t.Fatalf("evidence not recovered, got %d", len(loaded.Evidence))
	}
	if loaded.Version != 1 {
		t.Fatalf("version not recovered, got %d", loaded.Version)
	}
}

func TestFileRepositoryVersionConflict(t *testing.T) {
	dir := t.TempDir()
	repo, err := NewFileRepository(filepath.Join(dir, "l"), filepath.Join(dir, "s"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := repo.Save(sampleState(), 0); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Stale expected version must be rejected.
	st := sampleState()
	st.Version = 2
	err = repo.Save(st, 0)
	if !domain.IsCode(err, domain.CodeConcurrentModification) {
		t.Fatalf("expected CONCURRENT_MODIFICATION, got %v", err)
	}
}

func TestFileRepositoryCorruption(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "events.log")
	snapPath := filepath.Join(dir, "snapshot.bin")

	repo, err := NewFileRepository(logPath, snapPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := repo.Save(sampleState(), 0); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Corrupt the event log by overwriting bytes.
	if err := os.WriteFile(logPath, []byte("garbage-corrupting-the-log"), 0o644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	repo2, err := NewFileRepository(logPath, snapPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if repo2.Health().Corrupt != true {
		t.Fatalf("expected corrupt store, got %+v", repo2.Health())
	}
	if _, err := repo2.Load(); !domain.IsCode(err, domain.CodePersistenceCorrupt) {
		t.Fatalf("expected PERSISTENCE_CORRUPT, got %v", err)
	}
}
