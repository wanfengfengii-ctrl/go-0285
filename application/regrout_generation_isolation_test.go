package application

import (
	"testing"

	"precast-wall-grout-support-release/domain"
)

func TestModel_RegroutGenerationIsolation(t *testing.T) {
	setup := func(t *testing.T) *Service {
		t.Helper()
		svc, _, _ := newTestEnv(t)
		createAndLock(t, svc, "T1")
		mixBatch(t, svc, "T1", "batch-1")

		mustHandle(t, svc, Command{Type: CommandSample, TaskID: "T1", Operator: "op", OperationID: "sample", BatchID: "batch-1", SampleML: 100, SpecimenID: "SP1"})
		mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "pump", ResourceType: domain.ResourcePump, ResourceID: "pump-1", LeaseTicks: 1000})
		mustHandle(t, svc, Command{Type: CommandReserveSlurry, TaskID: "T1", Operator: "op", OperationID: "reserve", BatchID: "batch-1", VolumeML: 2000})
		mustHandle(t, svc, Command{Type: CommandInletStart, TaskID: "T1", Operator: "op", OperationID: "pour-1", Generation: 0, BatchID: "batch-1", PortID: "P1", VolumeML: 1000})
		mustHandle(t, svc, Command{Type: CommandOutletStable, TaskID: "T1", Operator: "op", OperationID: "pour-2", Generation: 0, BatchID: "batch-1", PortID: "P2", Pressure: 50})
		mustHandle(t, svc, Command{Type: CommandOutletSeal, TaskID: "T1", Operator: "op", OperationID: "pour-3", Generation: 0, BatchID: "batch-1", PortID: "P2"})
		mustHandle(t, svc, Command{Type: CommandPortSwitch, TaskID: "T1", Operator: "op", OperationID: "pour-4", Generation: 0, BatchID: "batch-1", PortID: "P3"})
		mustHandle(t, svc, Command{Type: CommandInletStart, TaskID: "T1", Operator: "op", OperationID: "pour-5", Generation: 0, BatchID: "batch-1", PortID: "P3", VolumeML: 1000})
		mustHandle(t, svc, Command{Type: CommandOutletStable, TaskID: "T1", Operator: "op", OperationID: "pour-6", Generation: 0, BatchID: "batch-1", PortID: "P4", Pressure: 50})
		mustHandle(t, svc, Command{Type: CommandOutletSeal, TaskID: "T1", Operator: "op", OperationID: "pour-7", Generation: 0, BatchID: "batch-1", PortID: "P4"})

		mustHandle(t, svc, Command{Type: CommandStrength, TaskID: "T1", Operator: "op", OperationID: "strength", Generation: 0, SpecimenID: "SP1", Value: 40})
		mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "ultrasonic-channel", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 1000})
		mustHandle(t, svc, Command{Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "ultrasonic", Generation: 0, ResourceID: "L1", Value: 40})
		mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "endoscope-channel", ResourceType: domain.ResourceChannel, ResourceID: "H1", LeaseTicks: 1000})
		mustHandle(t, svc, Command{Type: CommandEndoscope, TaskID: "T1", Operator: "op", OperationID: "endoscope", Generation: 0, ResourceID: "H1", Value: 40})
		mustHandle(t, svc, Command{Type: CommandLeak, TaskID: "T1", Operator: "op", OperationID: "leak", Generation: 0, SocketID: "S1", Value: 0})

		mustHandle(t, svc, Command{
			Type:        CommandRegrout,
			TaskID:      "T1",
			Operator:    "op",
			OperationID: "regrout",
			Generation:  0,
			Reason:      "local defect",
			Defects:     []domain.SocketID{"S1"},
		})
		return svc
	}

	tests := []struct {
		name  string
		check func(*testing.T, *Service)
	}{
		{
			name: "successor retains provenance but no parent completion state",
			check: func(t *testing.T, svc *Service) {
				generations, err := svc.ListGenerations("T1")
				if err != nil {
					t.Fatalf("list generations: %v", err)
				}
				if len(generations) != 2 {
					t.Fatalf("expected parent and one successor, got %d generations", len(generations))
				}
				parent, successor := generations[0], generations[1]
				if !parent.Isolated {
					t.Fatal("parent generation must be isolated after re-grout")
				}
				if successor.Generation != 1 || successor.ParentGeneration != 0 || successor.Reason != "local defect" {
					t.Fatalf("successor provenance mismatch: %+v", successor)
				}
				if len(successor.Defects) != 1 || successor.Defects[0] != "S1" || len(successor.RecheckSet) == 0 {
					t.Fatalf("successor defect and recheck scope mismatch: %+v", successor)
				}
				if len(successor.Prefix) != 0 || successor.Step != 0 {
					t.Fatalf("successor inherited parent pour progress: prefix=%v step=%d", successor.Prefix, successor.Step)
				}
				if successor.StrengthOK || successor.UltrasonicOK || successor.EndoscopeOK || successor.LeakOK {
					t.Fatalf("successor inherited parent inspection conclusions: %+v", successor)
				}
			},
		},
		{
			name: "new signatures alone cannot release an unworked successor",
			check: func(t *testing.T, svc *Service) {
				digest, err := svc.EvidenceDigest("T1", 1)
				if err != nil {
					t.Fatalf("successor evidence digest: %v", err)
				}
				mustHandle(t, svc, Command{Type: CommandReview, TaskID: "T1", Operator: "reviewer-a", OperationID: "review-a", EvidenceDigest: digest})
				mustHandle(t, svc, Command{Type: CommandReview, TaskID: "T1", Operator: "reviewer-b", OperationID: "review-b", EvidenceDigest: digest})

				result, err := svc.Handle(nil, Command{Type: CommandTerminal, TaskID: "T1", Operator: "op", OperationID: "release", TerminalType: domain.TerminalRelease})
				if !domain.IsCode(err, domain.CodeEvidenceIncomplete) {
					t.Fatalf("expected EVIDENCE_INCOMPLETE before successor pour and inspections, got result=%+v err=%v", result, err)
				}
				decision, getErr := svc.GetDecision("T1")
				if getErr != nil {
					t.Fatalf("get decision: %v", getErr)
				}
				if decision != nil || result.ReleaseCredential != "" {
					t.Fatalf("unworked successor received release state: decision=%+v credential=%q", decision, result.ReleaseCredential)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, setup(t))
		})
	}
}
