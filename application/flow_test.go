package application

import (
	"testing"

	"precast-wall-grout-support-release/domain"
)

// TestFullReleaseHappyPath exercises the complete lifecycle: lock, mix, sample,
// lease, reserve, continuous pour, four detections, dual review and terminal
// release producing a unique credential.
func TestFullReleaseHappyPath(t *testing.T) {
	svc, _, _ := newTestEnv(t)
	createAndLock(t, svc, "T1")
	mixBatch(t, svc, "T1", "batch-1")

	mustHandle(t, svc, Command{Type: CommandSample, TaskID: "T1", Operator: "op", OperationID: "sp", BatchID: "batch-1", SampleML: 100, SpecimenID: "SP1"})
	mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "pump", ResourceType: domain.ResourcePump, ResourceID: "pump-1", LeaseTicks: 1000})
	mustHandle(t, svc, Command{Type: CommandReserveSlurry, TaskID: "T1", Operator: "op", OperationID: "res", BatchID: "batch-1", VolumeML: 2000})

	// Continuous pour across two sockets.
	mustHandle(t, svc, Command{Type: CommandInletStart, TaskID: "T1", Operator: "op", OperationID: "p1", Generation: 0, BatchID: "batch-1", PortID: "P1", VolumeML: 1000})
	mustHandle(t, svc, Command{Type: CommandOutletStable, TaskID: "T1", Operator: "op", OperationID: "p2", Generation: 0, BatchID: "batch-1", PortID: "P2", Pressure: 50})
	mustHandle(t, svc, Command{Type: CommandOutletSeal, TaskID: "T1", Operator: "op", OperationID: "p3", Generation: 0, BatchID: "batch-1", PortID: "P2"})
	mustHandle(t, svc, Command{Type: CommandPortSwitch, TaskID: "T1", Operator: "op", OperationID: "p4", Generation: 0, BatchID: "batch-1", PortID: "P3"})
	mustHandle(t, svc, Command{Type: CommandInletStart, TaskID: "T1", Operator: "op", OperationID: "p5", Generation: 0, BatchID: "batch-1", PortID: "P3", VolumeML: 1000})
	mustHandle(t, svc, Command{Type: CommandOutletStable, TaskID: "T1", Operator: "op", OperationID: "p6", Generation: 0, BatchID: "batch-1", PortID: "P4", Pressure: 50})
	mustHandle(t, svc, Command{Type: CommandOutletSeal, TaskID: "T1", Operator: "op", OperationID: "p7", Generation: 0, BatchID: "batch-1", PortID: "P4"})

	// Detections.
	mustHandle(t, svc, Command{Type: CommandStrength, TaskID: "T1", Operator: "op", OperationID: "s", Generation: 0, SpecimenID: "SP1", Value: 40})
	mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "ch1", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 1000})
	mustHandle(t, svc, Command{Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "u", Generation: 0, ResourceID: "L1", Value: 40})
	mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "ch2", ResourceType: domain.ResourceChannel, ResourceID: "H1", LeaseTicks: 1000})
	mustHandle(t, svc, Command{Type: CommandEndoscope, TaskID: "T1", Operator: "op", OperationID: "e", Generation: 0, ResourceID: "H1", Value: 40})
	mustHandle(t, svc, Command{Type: CommandLeak, TaskID: "T1", Operator: "op", OperationID: "lk", Generation: 0, SocketID: "S1", Value: 0})

	// Dual review bound to the current evidence digest.
	digest, err := svc.EvidenceDigest("T1", 0)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	mustHandle(t, svc, Command{Type: CommandReview, TaskID: "T1", Operator: "reviewer-a", OperationID: "rv1", EvidenceDigest: digest})
	mustHandle(t, svc, Command{Type: CommandReview, TaskID: "T1", Operator: "reviewer-b", OperationID: "rv2", EvidenceDigest: digest})

	res, err := svc.Handle(nil, Command{Type: CommandTerminal, TaskID: "T1", Operator: "op", OperationID: "term", TerminalType: domain.TerminalRelease})
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if res.ReleaseCredential == "" {
		t.Fatalf("expected a release credential")
	}
	task, _ := svc.GetTask("T1")
	if task.Terminal == nil || task.Terminal.Type != domain.TerminalRelease {
		t.Fatalf("expected release terminal, got %+v", task.Terminal)
	}
	if task.Terminal.ReleaseCredential != res.ReleaseCredential {
		t.Fatalf("credential mismatch")
	}
}

// TestLateReceiptDoesNotOverrideGeneration verifies a late device receipt for
// an older generation is archived without changing the current conclusion.
func TestLateReceiptDoesNotOverrideGeneration(t *testing.T) {
	svc, _, adapter := newTestEnv(t)
	createAndLock(t, svc, "T1")

	// Acquire a channel lease and open an ultrasonic call on generation 0 that
	// first fails (leaving the call open and pending).
	mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "ch", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 1000})
	adapter.SetScript("ultrasonic:L1", domain.OutcomeDisconnect)
	if _, err := svc.Handle(nil, Command{Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "us", Generation: 0, ResourceID: "L1", Value: 50}); !domain.IsCode(err, domain.CodeDeviceRetryPending) {
		t.Fatalf("expected pending attempt, got %v", err)
	}

	// Re-grout advances the current generation to 1.
	mustHandle(t, svc, Command{Type: CommandRegrout, TaskID: "T1", Operator: "op", OperationID: "rg", Generation: 0, Reason: "defect", Defects: []domain.SocketID{"S1"}})

	// The late receipt for the old generation now arrives.
	adapter.SetScript("ultrasonic:L1", domain.OutcomeSuccess)
	if _, err := svc.RecordDeviceAttempt("ultrasonic:L1", domain.OutcomeSuccess, 50); err != nil {
		t.Fatalf("late receipt should be archived, got %v", err)
	}

	// The current generation (1) must not have been marked OK by the late receipt.
	gens, _ := svc.ListGenerations("T1")
	for _, g := range gens {
		if g.Generation == 1 && g.UltrasonicOK {
			t.Fatalf("late receipt must not mark the new generation OK")
		}
	}
}
