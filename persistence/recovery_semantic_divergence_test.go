package persistence_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

func TestModel_RecoveryRejectsCommittedEvidencePayloadDivergence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.EvidenceEvent)
	}{
		{
			name: "content digest differs",
			mutate: func(event *domain.EvidenceEvent) {
				event.ContentDigest = "snapshot-digest"
			},
		},
		{
			name: "operator differs",
			mutate: func(event *domain.EvidenceEvent) {
				event.Operator = "snapshot-operator"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "events.log")
			snapshotPath := filepath.Join(dir, "snapshot.json")

			repo, err := persistence.NewFileRepository(logPath, snapshotPath)
			if err != nil {
				t.Fatalf("create repository: %v", err)
			}
			state := persistence.NewState()
			state.Version = 1
			state.Evidence = []domain.EvidenceEvent{{
				ID:            "evt-1",
				TaskID:        "task-1",
				WallPanel:     "wall-1",
				Generation:    1,
				Type:          domain.EventMaterialCheck,
				At:            100,
				Operator:      "log-operator",
				OperationID:   "operation-1",
				ContentDigest: "log-digest",
				Valid:         true,
			}}
			state.NextSequence = 1
			state.NextEventID = 1
			if err := repo.Save(state, 0); err != nil {
				t.Fatalf("commit evidence: %v", err)
			}

			diverged := state.Clone()
			tt.mutate(&diverged.Evidence[0])
			snapshot, err := json.Marshal(diverged)
			if err != nil {
				t.Fatalf("encode divergent snapshot: %v", err)
			}
			if err := os.WriteFile(snapshotPath, snapshot, 0o644); err != nil {
				t.Fatalf("write divergent snapshot: %v", err)
			}

			recovered, err := persistence.NewFileRepository(logPath, snapshotPath)
			if err != nil {
				t.Fatalf("reopen repository: %v", err)
			}
			health := recovered.Health()
			if health.Ready || !health.Corrupt {
				t.Errorf("divergent committed evidence must be corrupt and unready, got %+v", health)
			}
			if _, err := recovered.Load(); !domain.IsCode(err, domain.CodePersistenceCorrupt) {
				t.Errorf("Load() error = %v, want PERSISTENCE_CORRUPT", err)
			}
		})
	}
}
