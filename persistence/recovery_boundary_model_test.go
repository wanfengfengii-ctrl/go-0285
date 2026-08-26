package persistence

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"precast-wall-grout-support-release/domain"
)

func TestModel_RecoveryPreservesCommittedLogWhenSnapshotIsUnavailableOrStale(t *testing.T) {
	cases := []struct {
		name           string
		prepareStorage func(t *testing.T, logPath, snapshotPath string, staleSnapshot []byte)
	}{
		{
			name: "missing snapshot",
			prepareStorage: func(t *testing.T, _, snapshotPath string, _ []byte) {
				t.Helper()
				if err := os.Remove(snapshotPath); err != nil {
					t.Fatalf("remove snapshot: %v", err)
				}
			},
		},
		{
			name: "snapshot behind committed log",
			prepareStorage: func(t *testing.T, _, snapshotPath string, staleSnapshot []byte) {
				t.Helper()
				if err := os.WriteFile(snapshotPath, staleSnapshot, 0o644); err != nil {
					t.Fatalf("restore stale snapshot: %v", err)
				}
			},
		},
		{
			name: "uncommitted partial tail",
			prepareStorage: func(t *testing.T, logPath, _ string, _ []byte) {
				t.Helper()
				f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0o644)
				if err != nil {
					t.Fatalf("open log for partial tail: %v", err)
				}
				if _, err := f.Write([]byte{0, 0, 0, 0, 0}); err != nil {
					f.Close()
					t.Fatalf("append partial tail: %v", err)
				}
				if err := f.Close(); err != nil {
					t.Fatalf("close log with partial tail: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "events.log")
			snapshotPath := filepath.Join(dir, "snapshot.json")

			repo, err := NewFileRepository(logPath, snapshotPath)
			if err != nil {
				t.Fatalf("open repository: %v", err)
			}

			state := NewState()
			state.Version = 1
			state.Tasks[domain.TaskID("T-committed")] = &domain.InspectionTask{
				ID:                domain.TaskID("T-committed"),
				Building:          "B1",
				Level:             "L1",
				WallPanel:         "W1",
				Stage:             domain.StageLocked,
				AggregateVersion:  1,
				CurrentGeneration: 0,
			}
			state.Evidence = append(state.Evidence, domain.EvidenceEvent{
				ID:            "evt-1",
				TaskID:        domain.TaskID("T-committed"),
				WallPanel:     "W1",
				Generation:    0,
				Type:          domain.EventMaterialCheck,
				Valid:         true,
				ContentDigest: "digest-1",
			})
			state.NextEventID = 1
			state.NextSequence = 1
			if err := repo.Save(state, 0); err != nil {
				t.Fatalf("save first commit: %v", err)
			}

			staleSnapshot, err := os.ReadFile(snapshotPath)
			if err != nil {
				t.Fatalf("read first snapshot: %v", err)
			}
			latest := state.Clone()
			latest.Version = 2
			latest.Evidence = append(latest.Evidence, domain.EvidenceEvent{
				ID:            "evt-2",
				TaskID:        domain.TaskID("T-committed"),
				WallPanel:     "W1",
				Generation:    0,
				Type:          domain.EventMaterialCheck,
				Valid:         true,
				ContentDigest: "digest-2",
			})
			latest.NextEventID = 2
			latest.NextSequence = 2
			if err := repo.Save(latest, 1); err != nil {
				t.Fatalf("save second commit: %v", err)
			}
			if err := repo.Close(); err != nil {
				t.Fatalf("close repository: %v", err)
			}

			committedLog, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("read committed log: %v", err)
			}
			tc.prepareStorage(t, logPath, snapshotPath, staleSnapshot)

			recovered, err := NewFileRepository(logPath, snapshotPath)
			if err != nil {
				t.Fatalf("reopen repository: %v", err)
			}
			health := recovered.Health()
			loaded, loadErr := recovered.Load()
			if health.Ready {
				if loadErr != nil {
					t.Fatalf("ready repository failed to load: %v", loadErr)
				}
				if loaded.Version != 2 || len(loaded.Evidence) != 2 || loaded.Tasks[domain.TaskID("T-committed")] == nil {
					t.Fatalf("ready recovery did not rebuild the latest committed state: version=%d evidence=%d task=%v", loaded.Version, len(loaded.Evidence), loaded.Tasks[domain.TaskID("T-committed")])
				}
			} else {
				if !health.Corrupt {
					t.Fatalf("repository neither recovered nor reported corruption: %+v", health)
				}
				if !domain.IsCode(loadErr, domain.CodePersistenceCorrupt) {
					t.Fatalf("unready recovery must return PERSISTENCE_CORRUPT, got %v", loadErr)
				}
			}

			logAfterRecovery, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("read log after recovery: %v", err)
			}
			if !bytes.Equal(logAfterRecovery, committedLog) {
				t.Fatalf("recovery modified complete committed frames: before=%d bytes after=%d bytes", len(committedLog), len(logAfterRecovery))
			}
		})
	}
}
