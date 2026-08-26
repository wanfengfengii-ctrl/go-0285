package application_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"precast-wall-grout-support-release/application"
	"precast-wall-grout-support-release/devices"
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

type simultaneousSaveRepository struct {
	persistence.Repository
	armed atomic.Bool
	calls atomic.Int32
	gate  chan struct{}
}

func (r *simultaneousSaveRepository) Save(state *persistence.State, expectedVersion int64) error {
	if r.armed.Load() {
		call := r.calls.Add(1)
		if call <= 2 {
			if call == 2 {
				close(r.gate)
			}
			<-r.gate
		}
	}
	return r.Repository.Save(state, expectedVersion)
}

func TestModel_ConcurrentFileRepositoryCommits(t *testing.T) {
	tests := []struct {
		name      string
		batches   [2]string
		operators [2]domain.Operator
	}{
		{
			name:      "successful commands survive snapshot recovery",
			batches:   [2]string{"MAT-001", "MAT-002"},
			operators: [2]domain.Operator{"inspector-a", "inspector-b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "events.log")
			snapshotPath := filepath.Join(dir, "snapshot.json")
			fileRepo, err := persistence.NewFileRepository(logPath, snapshotPath)
			if err != nil {
				t.Fatalf("open repository: %v", err)
			}

			initial := persistence.NewState()
			initial.Version = 1
			for i := 0; i < 25000; i++ {
				id := domain.TaskID(fmt.Sprintf("filler-%05d", i))
				initial.Tasks[id] = &domain.InspectionTask{ID: id, Stage: domain.StageLocked}
			}
			for _, id := range []domain.TaskID{"task-a", "task-b"} {
				initial.Tasks[id] = &domain.InspectionTask{
					ID:               id,
					Building:         "B1",
					Level:            "L1",
					WallPanel:        "W1-" + string(id),
					Stage:            domain.StageLocked,
					AggregateVersion: 1,
				}
			}
			if err := fileRepo.Save(initial, 0); err != nil {
				t.Fatalf("seed repository: %v", err)
			}

			coordinated := &simultaneousSaveRepository{
				Repository: fileRepo,
				gate:       make(chan struct{}),
			}
			svc := application.NewService(
				coordinated,
				application.NewManualClock(10),
				application.DefaultCatalog(),
				devices.NewScriptedAdapter(),
			)
			coordinated.armed.Store(true)

			commands := [2]application.Command{
				{Type: application.CommandMaterialCheck, TaskID: "task-a", Operator: tc.operators[0], OperationID: "material-a", MaterialBatch: tc.batches[0]},
				{Type: application.CommandMaterialCheck, TaskID: "task-b", Operator: tc.operators[1], OperationID: "material-b", MaterialBatch: tc.batches[1]},
			}
			errs := make(chan error, len(commands))
			var wg sync.WaitGroup
			for _, command := range commands {
				command := command
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := svc.Handle(context.Background(), command)
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatalf("concurrent command reported failure: %v", err)
				}
			}

			if err := fileRepo.Close(); err != nil {
				t.Fatalf("close repository: %v", err)
			}
			recoveredRepo, err := persistence.NewFileRepository(logPath, snapshotPath)
			if err != nil {
				t.Fatalf("reopen repository: %v", err)
			}
			if health := recoveredRepo.Health(); !health.Ready || !health.Recovered || health.Corrupt {
				t.Fatalf("repository not ready after recovery: %+v", health)
			}
			recovered := application.NewService(
				recoveredRepo,
				application.NewManualClock(20),
				application.DefaultCatalog(),
				devices.NewScriptedAdapter(),
			)
			for _, command := range commands {
				task, err := recovered.GetTask(command.TaskID)
				if err != nil {
					t.Fatalf("get recovered task %s: %v", command.TaskID, err)
				}
				if task.AggregateVersion != 2 {
					t.Errorf("task %s lost committed state: aggregate version = %d, want 2", command.TaskID, task.AggregateVersion)
				}
				evidence, err := recovered.ListEvidence(command.TaskID)
				if err != nil {
					t.Fatalf("list recovered evidence for %s: %v", command.TaskID, err)
				}
				if len(evidence) != 1 || evidence[0].OperationID != command.OperationID {
					t.Errorf("task %s evidence was not retained: %+v", command.TaskID, evidence)
				}
			}
		})
	}
}
