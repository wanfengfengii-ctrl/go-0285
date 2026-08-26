package application

import (
	"testing"

	"precast-wall-grout-support-release/devices"
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

func TestModel_ReviewAuditPersistence(t *testing.T) {
	prepare := func(t *testing.T) (*Service, *persistence.MemoryRepository, string) {
		t.Helper()
		repo := persistence.NewMemoryStore("review-audit.log", "review-audit.snapshot")
		svc := NewService(repo, NewManualClock(0), DefaultCatalog(), devices.NewScriptedAdapter())
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
		mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "ultrasonic-lease", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 1000})
		mustHandle(t, svc, Command{Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "ultrasonic", Generation: 0, ResourceID: "L1", Value: 40})
		mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "endoscope-lease", ResourceType: domain.ResourceChannel, ResourceID: "H1", LeaseTicks: 1000})
		mustHandle(t, svc, Command{Type: CommandEndoscope, TaskID: "T1", Operator: "op", OperationID: "endoscope", Generation: 0, ResourceID: "H1", Value: 40})
		mustHandle(t, svc, Command{Type: CommandLeak, TaskID: "T1", Operator: "op", OperationID: "leak", Generation: 0, SocketID: "S1", Value: 0})
		digest, err := svc.EvidenceDigest("T1", 0)
		if err != nil {
			t.Fatalf("evidence digest: %v", err)
		}
		return svc, repo, digest
	}

	reviewEvents := func(state *persistence.State) []domain.EvidenceEvent {
		var events []domain.EvidenceEvent
		for _, event := range state.Evidence {
			if event.TaskID == "T1" && event.Type == domain.EventReview {
				events = append(events, event)
			}
		}
		return events
	}

	cases := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "successful review atomically persists its audit event",
			run: func(t *testing.T) {
				svc, repo, digest := prepare(t)
				before, err := repo.Load()
				if err != nil {
					t.Fatalf("load before review: %v", err)
				}
				cmd := Command{Type: CommandReview, TaskID: "T1", Operator: "reviewer-a", OperationID: "review-a", EvidenceDigest: digest}
				if _, err := svc.Handle(nil, cmd); err != nil {
					t.Fatalf("review: %v", err)
				}
				after, err := repo.Load()
				if err != nil {
					t.Fatalf("load after review: %v", err)
				}
				events := reviewEvents(after)
				if after.Version != before.Version+1 || len(after.Reviews["T1"]) != 1 || len(events) != 1 {
					t.Fatalf("review and audit event must share one committed version: versions %d->%d reviews=%d events=%d", before.Version, after.Version, len(after.Reviews["T1"]), len(events))
				}
				review, event := after.Reviews["T1"][0], events[0]
				if review.Reviewer != cmd.Operator || review.EvidenceDigest != digest || !event.Valid || event.Generation != 0 || event.Operator != cmd.Operator || event.OperationID != cmd.OperationID || event.ContentDigest != digest || event.At != review.At {
					t.Fatalf("review audit binding mismatch: review=%+v event=%+v", review, event)
				}
				gotDigest, err := svc.EvidenceDigest("T1", 0)
				if err != nil || gotDigest != digest {
					t.Fatalf("review changed signed business evidence digest: got %q want %q err=%v", gotDigest, digest, err)
				}
			},
		},
		{
			name: "rejected reviewers leave neither half persisted",
			run: func(t *testing.T) {
				for _, rejection := range []struct {
					name     string
					operator domain.Operator
					prime    bool
				}{
					{name: "duplicate", operator: "reviewer-a", prime: true},
					{name: "invalid qualification", operator: "unqualified-reviewer"},
				} {
					t.Run(rejection.name, func(t *testing.T) {
						svc, repo, digest := prepare(t)
						if rejection.prime {
							mustHandle(t, svc, Command{Type: CommandReview, TaskID: "T1", Operator: rejection.operator, OperationID: "first-review", EvidenceDigest: digest})
						}
						before, _ := repo.Load()
						_, err := svc.Handle(nil, Command{Type: CommandReview, TaskID: "T1", Operator: rejection.operator, OperationID: "rejected-review", EvidenceDigest: digest})
						if !domain.IsCode(err, domain.CodeReviewerConflict) {
							t.Fatalf("expected reviewer conflict, got %v", err)
						}
						after, _ := repo.Load()
						if after.Version != before.Version || len(after.Reviews["T1"]) != len(before.Reviews["T1"]) || len(reviewEvents(after)) != len(reviewEvents(before)) || after.NextEventID != before.NextEventID {
							t.Fatalf("rejected review changed persistent state")
						}
					})
				}
			},
		},
		{
			name: "orphan review signatures cannot satisfy release gate",
			run: func(t *testing.T) {
				svc, repo, digest := prepare(t)
				state, _ := repo.Load()
				state.Reviews["T1"] = []domain.Review{
					{Reviewer: "reviewer-a", EvidenceDigest: digest, At: 1},
					{Reviewer: "reviewer-b", EvidenceDigest: digest, At: 1},
				}
				state.Version++
				if err := repo.Save(state, state.Version-1); err != nil {
					t.Fatalf("persist simulated partial signatures: %v", err)
				}
				res, err := svc.Handle(nil, Command{Type: CommandTerminal, TaskID: "T1", Operator: "op", OperationID: "release", TerminalType: domain.TerminalRelease})
				if err == nil || res.ReleaseCredential != "" {
					t.Fatalf("release consumed reviews without audit events: result=%+v err=%v", res, err)
				}
				decision, getErr := svc.GetDecision("T1")
				if getErr != nil || decision != nil {
					t.Fatalf("rejected release persisted a terminal decision: decision=%+v err=%v", decision, getErr)
				}
			},
		},
		{
			name: "complete dual review produces one release credential",
			run: func(t *testing.T) {
				svc, repo, digest := prepare(t)
				mustHandle(t, svc, Command{Type: CommandReview, TaskID: "T1", Operator: "reviewer-a", OperationID: "review-a", EvidenceDigest: digest})
				mustHandle(t, svc, Command{Type: CommandReview, TaskID: "T1", Operator: "reviewer-b", OperationID: "review-b", EvidenceDigest: digest})
				state, _ := repo.Load()
				if len(state.Reviews["T1"]) != 2 || len(reviewEvents(state)) != 2 {
					t.Fatalf("dual review incomplete: reviews=%d events=%d", len(state.Reviews["T1"]), len(reviewEvents(state)))
				}
				cmd := Command{Type: CommandTerminal, TaskID: "T1", Operator: "op", OperationID: "release", TerminalType: domain.TerminalRelease}
				first, err := svc.Handle(nil, cmd)
				if err != nil || first.ReleaseCredential == "" {
					t.Fatalf("release failed: result=%+v err=%v", first, err)
				}
				replay, err := svc.Handle(nil, cmd)
				if err != nil || replay.ReleaseCredential != first.ReleaseCredential {
					t.Fatalf("idempotent release did not return the unique credential: first=%q replay=%q err=%v", first.ReleaseCredential, replay.ReleaseCredential, err)
				}
				if _, err := svc.Handle(nil, Command{Type: CommandTerminal, TaskID: "T1", Operator: "op", OperationID: "second-release", TerminalType: domain.TerminalRelease}); !domain.IsCode(err, domain.CodeTerminalConflict) {
					t.Fatalf("second release should conflict, got %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, tc.run)
	}
}
