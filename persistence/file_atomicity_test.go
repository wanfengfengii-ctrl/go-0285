package persistence_test

import (
	"os"
	"path/filepath"
	"testing"

	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

func TestModel_FileRepositoryLogAppendFailureIsAtomic(t *testing.T) {
	cases := []struct {
		name    string
		restart bool
	}{
		{name: "failed save remains invisible to queries"},
		{name: "failed save remains recoverable after restart", restart: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "events.log")
			snapshotPath := filepath.Join(dir, "snapshot.json")

			repo, err := persistence.NewFileRepository(logPath, snapshotPath)
			if err != nil {
				t.Fatalf("open repository: %v", err)
			}

			// A directory at the event-log path deterministically models a
			// temporarily unwritable append target while leaving the snapshot
			// directory writable.
			if err := os.Mkdir(logPath, 0o755); err != nil {
				t.Fatalf("block event log: %v", err)
			}

			candidate := persistence.NewState()
			candidate.Version = 1
			candidate.Tasks[domain.TaskID("T1")] = &domain.InspectionTask{
				ID:               domain.TaskID("T1"),
				WallPanel:        "W1",
				AggregateVersion: 1,
			}
			candidate.Evidence = append(candidate.Evidence, domain.EvidenceEvent{
				ID:            "evt-1",
				TaskID:        domain.TaskID("T1"),
				WallPanel:     "W1",
				Type:          domain.EventMaterialCheck,
				ContentDigest: "material-check",
				Valid:         true,
			})
			candidate.NextEventID = 1
			candidate.NextSequence = 1

			if err := repo.Save(candidate, 0); err == nil {
				t.Fatal("save unexpectedly succeeded with an unwritable event log")
			}

			if err := os.Remove(logPath); err != nil {
				t.Fatalf("restore event-log target: %v", err)
			}

			observed := repo
			if tc.restart {
				if err := repo.Close(); err != nil {
					t.Fatalf("close repository: %v", err)
				}
				observed, err = persistence.NewFileRepository(logPath, snapshotPath)
				if err != nil {
					t.Fatalf("reopen repository: %v", err)
				}
				if health := observed.Health(); !health.Ready || health.Corrupt {
					t.Fatalf("repository not ready after failed save: %+v", health)
				}
			}

			state, err := observed.Load()
			if err != nil {
				t.Fatalf("load state after failed save: %v", err)
			}
			if state.Version != 0 || len(state.Evidence) != 0 || state.Tasks[domain.TaskID("T1")] != nil {
				t.Fatalf("failed save became visible: version=%d evidence=%d task=%v", state.Version, len(state.Evidence), state.Tasks[domain.TaskID("T1")])
			}
		})
	}
}
