package application

import (
	"testing"

	"precast-wall-grout-support-release/domain"
)

func TestModel_LockValidatesSlurryPathsAgainstLockedTopology(t *testing.T) {
	tests := []struct {
		name        string
		slurryPaths [][]domain.PortID
		wantCode    domain.Code
	}{
		{
			name:        "rejects omitted inlet paths",
			slurryPaths: nil,
			wantCode:    domain.CodeInvalidTopology,
		},
		{
			name:        "rejects a missing locked inlet",
			slurryPaths: [][]domain.PortID{{"P1"}},
			wantCode:    domain.CodeInvalidTopology,
		},
		{
			name:        "rejects a duplicated inlet reference",
			slurryPaths: [][]domain.PortID{{"P1", "P1", "P3"}},
			wantCode:    domain.CodeInvalidTopology,
		},
		{
			name:        "rejects an outlet reference",
			slurryPaths: [][]domain.PortID{{"P1", "P2", "P3"}},
			wantCode:    domain.CodeInvalidTopology,
		},
		{
			name:        "rejects an order inconsistent with the port graph",
			slurryPaths: [][]domain.PortID{{"P3", "P1"}},
			wantCode:    domain.CodeInvalidTopology,
		},
		{
			name:        "accepts the topology ordered inlet sequence",
			slurryPaths: [][]domain.PortID{{"P1", "P3"}},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newTestEnv(t)
			taskID := domain.TaskID("slurry-path-" + string(rune('A'+i)))
			if _, err := svc.CreateTask(nil, taskID, "B1", "L1", "W1"); err != nil {
				t.Fatalf("create task: %v", err)
			}

			cmd := lockPlanCmd(taskID, domain.OperationID("lock-"+taskID))
			cmd.SlurryPaths = tt.slurryPaths
			_, err := svc.Handle(nil, cmd)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("valid lock rejected: %v", err)
				}
				task, getErr := svc.GetTask(taskID)
				if getErr != nil {
					t.Fatalf("get locked task: %v", getErr)
				}
				if task.Stage != domain.StageLocked {
					t.Fatalf("valid plan stage = %s, want %s", task.Stage, domain.StageLocked)
				}
				return
			}

			if !domain.IsCode(err, tt.wantCode) {
				t.Fatalf("lock error = %v, want %s", err, tt.wantCode)
			}
			task, getErr := svc.GetTask(taskID)
			if getErr != nil {
				t.Fatalf("get rejected task: %v", getErr)
			}
			if task.Stage != domain.StageCreated {
				t.Fatalf("rejected plan stage = %s, want %s", task.Stage, domain.StageCreated)
			}
		})
	}
}
