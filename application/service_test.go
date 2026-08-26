package application

import (
	"sync"
	"testing"

	"precast-wall-grout-support-release/domain"
)

func TestLockRejectsStaleCatalog(t *testing.T) {
	svc, _, _ := newTestEnv(t)
	if _, err := svc.CreateTask(nil, "T1", "B1", "L1", "W1"); err != nil {
		t.Fatalf("create: %v", err)
	}
	cmd := lockPlanCmd("T1", "lock-1")
	cmd.RuleDigest = "stale-digest"
	_, err := svc.Handle(nil, cmd)
	if !domain.IsCode(err, domain.CodeStaleRuleDigest) {
		t.Fatalf("expected STALE_RULE_DIGEST, got %v", err)
	}
	task, _ := svc.GetTask("T1")
	if task.Stage != domain.StageCreated {
		t.Fatalf("task should remain un-locked, got %s", task.Stage)
	}
}

func TestPourPrefixViolation(t *testing.T) {
	svc, _, _ := newTestEnv(t)
	createAndLock(t, svc, "T1")
	mixBatch(t, svc, "T1", "batch-1")
	mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "pump", ResourceType: domain.ResourcePump, ResourceID: "pump-1", LeaseTicks: 100})
	mustHandle(t, svc, Command{Type: CommandReserveSlurry, TaskID: "T1", Operator: "op", OperationID: "res", BatchID: "batch-1", VolumeML: 2000})

	// Sealing before any inlet start is out of order.
	_, err := svc.Handle(nil, Command{Type: CommandOutletSeal, TaskID: "T1", Operator: "op", OperationID: "seal-early", Generation: 0, BatchID: "batch-1", PortID: "P2"})
	if !domain.IsCode(err, domain.CodePortSealOutOfOrder) {
		t.Fatalf("expected PORT_SEAL_OUT_OF_ORDER, got %v", err)
	}
}

func TestWorkDeadlineSlurryExpired(t *testing.T) {
	svc, clock, _ := newTestEnv(t)
	createAndLock(t, svc, "T1")
	mixBatch(t, svc, "T1", "batch-1") // WorkDeadline = now(0)+100 = 100

	clock.Set(99)
	_, err := svc.Handle(nil, Command{Type: CommandReserveSlurry, TaskID: "T1", Operator: "op", OperationID: "r99", BatchID: "batch-1", VolumeML: 10})
	if err != nil {
		t.Fatalf("reserve before deadline should succeed, got %v", err)
	}

	clock.Set(100)
	_, err = svc.Handle(nil, Command{Type: CommandReserveSlurry, TaskID: "T1", Operator: "op", OperationID: "r100", BatchID: "batch-1", VolumeML: 10})
	if !domain.IsCode(err, domain.CodeSlurryExpired) {
		t.Fatalf("expected SLURRY_EXPIRED at deadline, got %v", err)
	}
}

func TestReserveOverclaimConcurrent(t *testing.T) {
	svc, _, _ := newTestEnv(t)
	createAndLock(t, svc, "T1")
	if _, err := svc.CreateTask(nil, "T2", "B1", "L1", "W1"); err != nil {
		t.Fatalf("create T2: %v", err)
	}
	mixBatch(t, svc, "T1", "batch-1") // available = 6800

	barrier := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	ids := []domain.TaskID{"T1", "T2"}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			_, errs[i] = svc.Handle(nil, Command{
				Type:        CommandReserveSlurry,
				TaskID:      ids[i],
				Operator:    "op",
				OperationID: domain.OperationID(ids[i] + "-res"),
				BatchID:     "batch-1",
				VolumeML:    4000,
			})
		}(i)
	}
	close(barrier)
	wg.Wait()

	success := 0
	overclaim := 0
	for _, e := range errs {
		if e == nil {
			success++
		} else if domain.IsCode(e, domain.CodeVolumeOverclaim) {
			overclaim++
		}
	}
	if success != 1 || overclaim != 1 {
		t.Fatalf("expected exactly one success and one overclaim, got success=%d overclaim=%d errs=%v", success, overclaim, errs)
	}
}

func TestLeaseCompetitionConcurrent(t *testing.T) {
	svc, _, _ := newTestEnv(t)
	createAndLock(t, svc, "T1")
	if _, err := svc.CreateTask(nil, "T2", "B1", "L1", "W1"); err != nil {
		t.Fatalf("create T2: %v", err)
	}

	barrier := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	ids := []domain.TaskID{"T1", "T2"}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			_, errs[i] = svc.Handle(nil, Command{
				Type:         CommandAcquireLease,
				TaskID:       ids[i],
				Operator:     "op",
				OperationID:  domain.OperationID(ids[i] + "-lease"),
				ResourceType: domain.ResourcePump,
				ResourceID:   "pump-1",
				LeaseTicks:   100,
			})
		}(i)
	}
	close(barrier)
	wg.Wait()

	success, conflict := 0, 0
	for _, e := range errs {
		if e == nil {
			success++
		} else if domain.IsCode(e, domain.CodeLeaseConflict) {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("expected one lease winner and one conflict, got success=%d conflict=%d errs=%v", success, conflict, errs)
	}
}

func TestRegroutSingleSuccessorConcurrent(t *testing.T) {
	svc, _, _ := newTestEnv(t)
	createAndLock(t, svc, "T1")

	barrier := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			_, errs[i] = svc.Handle(nil, Command{
				Type:        CommandRegrout,
				TaskID:      "T1",
				Operator:    "op",
				OperationID: domain.OperationID([]string{"regrout-a", "regrout-b"}[i]),
				Generation:  0,
				Reason:      "strength insufficient",
				Defects:     []domain.SocketID{"S1"},
			})
		}(i)
	}
	close(barrier)
	wg.Wait()

	success, conflict := 0, 0
	for _, e := range errs {
		if e == nil {
			success++
		} else if domain.IsCode(e, domain.CodeGenerationConflict) {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("expected one regrout winner, got success=%d conflict=%d errs=%v", success, conflict, errs)
	}
	task, _ := svc.GetTask("T1")
	if task.CurrentGeneration != 1 {
		t.Fatalf("expected generation 1, got %d", task.CurrentGeneration)
	}
}

func TestIdempotencyAndConflict(t *testing.T) {
	svc, _, _ := newTestEnv(t)
	createAndLock(t, svc, "T1")
	cmd := Command{Type: CommandMaterialCheck, TaskID: "T1", Operator: "op", OperationID: "op-1", MaterialBatch: "MAT-001"}
	r1, err := svc.Handle(nil, cmd)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	r2, err := svc.Handle(nil, cmd)
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if r1.AggregateVersion != r2.AggregateVersion {
		t.Fatalf("idempotent replay changed version: %d vs %d", r1.AggregateVersion, r2.AggregateVersion)
	}
	// Same operation id, different content.
	cmd2 := cmd
	cmd2.MaterialBatch = "MAT-002"
	if _, err := svc.Handle(nil, cmd2); !domain.IsCode(err, domain.CodeIdempotencyConflict) {
		t.Fatalf("expected IDEMPOTENCY_CONFLICT, got %v", err)
	}
}

func TestDeviceFailureScript(t *testing.T) {
	svc, _, adapter := newTestEnv(t)
	createAndLock(t, svc, "T1")
	mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "chan", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 1000})

	adapter.SetScript("ultrasonic:L1",
		domain.OutcomeRejected,
		domain.OutcomeDisconnect,
		domain.OutcomeTimeout,
		domain.OutcomeMalformed,
		domain.OutcomeSuccess,
	)
	cmd := Command{Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "us-1", Generation: 0, ResourceID: "L1", Value: 50}
	for i := 0; i < 4; i++ {
		if _, err := svc.Handle(nil, cmd); !domain.IsCode(err, domain.CodeDeviceRetryPending) {
			t.Fatalf("attempt %d: expected DEVICE_RETRY_PENDING, got %v", i+1, err)
		}
	}
	if _, err := svc.Handle(nil, cmd); err != nil {
		t.Fatalf("final attempt should succeed, got %v", err)
	}
	ev, _ := svc.ListEvidence("T1")
	valid := 0
	for _, e := range ev {
		if e.Type == domain.EventUltrasonic && e.Valid {
			valid++
		}
	}
	if valid != 1 {
		t.Fatalf("expected exactly one valid ultrasonic reading, got %d", valid)
	}
	gens, _ := svc.ListGenerations("T1")
	if !gens[0].UltrasonicOK {
		t.Fatalf("expected ultrasonic conclusion OK")
	}
}

func TestTerminalReleaseGate(t *testing.T) {
	svc, _, _ := newTestEnv(t)
	createAndLock(t, svc, "T1")
	mixBatch(t, svc, "T1", "batch-1")

	// Missing everything: release must fail.
	_, err := svc.Handle(nil, Command{Type: CommandTerminal, TaskID: "T1", Operator: "op", OperationID: "term-1", TerminalType: domain.TerminalRelease})
	if !domain.IsCode(err, domain.CodeEvidenceIncomplete) {
		t.Fatalf("expected EVIDENCE_INCOMPLETE, got %v", err)
	}
}

func TestTerminalRaceConcurrent(t *testing.T) {
	svc, _, _ := newTestEnv(t)
	createAndLock(t, svc, "T1")

	types := []domain.TerminalType{domain.TerminalRelease, domain.TerminalQuarantine, domain.TerminalCancel}
	barrier := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			_, errs[i] = svc.Handle(nil, Command{
				Type:         CommandTerminal,
				TaskID:       "T1",
				Operator:     "op",
				OperationID:  domain.OperationID("term-" + string(types[i])),
				TerminalType: types[i],
			})
		}(i)
	}
	close(barrier)
	wg.Wait()

	// Only the non-release decisions can win without gates; release always loses
	// because the gates are not satisfied. Exactly one terminal may occupy the
	// slot; losers get TERMINAL_CONFLICT or EVIDENCE_INCOMPLETE.
	winners := 0
	for _, e := range errs {
		if e == nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("expected exactly one terminal winner, got %d errs=%v", winners, errs)
	}
	task, _ := svc.GetTask("T1")
	if task.Terminal == nil {
		t.Fatalf("expected a terminal decision")
	}
	if task.Terminal.Type == domain.TerminalRelease {
		t.Fatalf("release must not win without satisfied gates")
	}
}

func mustHandle(t *testing.T, svc *Service, cmd Command) {
	t.Helper()
	if _, err := svc.Handle(nil, cmd); err != nil {
		t.Fatalf("command %s failed: %v", cmd.Type, err)
	}
}
