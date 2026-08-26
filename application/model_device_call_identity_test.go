package application

import (
	"testing"

	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

type modelRetryAdapter struct {
	calls []domain.DeviceCall
}

func (a *modelRetryAdapter) Call(call domain.DeviceCall) (domain.DeviceOutcome, error) {
	a.calls = append(a.calls, call)
	if len(call.Attempts) == 0 {
		return domain.OutcomeDisconnect, nil
	}
	return domain.OutcomeSuccess, nil
}

func TestModel_DeviceCallIdentityIsolation(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "released channel is reusable by another task",
			run: func(t *testing.T) {
				svc, _, _ := newTestEnv(t)
				createAndLock(t, svc, "T1")
				createAndLock(t, svc, "T2")

				mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "acquire-t1", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 100})
				mustHandle(t, svc, Command{Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "scan-t1", Generation: 0, ResourceID: "L1", Value: 41})
				mustHandle(t, svc, Command{Type: CommandReleaseLease, TaskID: "T1", Operator: "op", OperationID: "release-t1", ResourceType: domain.ResourceChannel, ResourceID: "L1"})
				mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T2", Operator: "op", OperationID: "acquire-t2", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 100})
				mustHandle(t, svc, Command{Type: CommandUltrasonic, TaskID: "T2", Operator: "op", OperationID: "scan-t2", Generation: 0, ResourceID: "L1", Value: 42})

				for _, want := range []struct {
					task      domain.TaskID
					operation domain.OperationID
					value     int64
				}{{"T1", "scan-t1", 41}, {"T2", "scan-t2", 42}} {
					evidence, err := svc.ListEvidence(want.task)
					if err != nil {
						t.Fatalf("list evidence for %s: %v", want.task, err)
					}
					matches := 0
					for _, event := range evidence {
						if event.Type == domain.EventUltrasonic && event.Valid && event.TaskID == want.task && event.Generation == 0 && event.OperationID == want.operation && event.FixedValue == want.value {
							matches++
						}
					}
					if matches != 1 {
						t.Fatalf("task %s must have exactly one of its own valid ultrasonic evidence events, got %d", want.task, matches)
					}
				}
			},
		},
		{
			name: "released channel is reusable by a later generation",
			run: func(t *testing.T) {
				svc, _, _ := newTestEnv(t)
				createAndLock(t, svc, "T1")

				mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "acquire-g0", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 100})
				mustHandle(t, svc, Command{Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "scan-g0", Generation: 0, ResourceID: "L1", Value: 41})
				mustHandle(t, svc, Command{Type: CommandReleaseLease, TaskID: "T1", Operator: "op", OperationID: "release-g0", ResourceType: domain.ResourceChannel, ResourceID: "L1"})
				mustHandle(t, svc, Command{Type: CommandRegrout, TaskID: "T1", Operator: "op", OperationID: "regrout", Generation: 0, Reason: "void", Defects: []domain.SocketID{"S1"}})
				mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "acquire-g1", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 100})
				mustHandle(t, svc, Command{Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "scan-g1", Generation: 1, ResourceID: "L1", Value: 42})

				evidence, err := svc.ListEvidence("T1")
				if err != nil {
					t.Fatalf("list evidence: %v", err)
				}
				seen := map[domain.Generation]int{}
				for _, event := range evidence {
					if event.Type == domain.EventUltrasonic && event.Valid {
						seen[event.Generation]++
					}
				}
				if seen[0] != 1 || seen[1] != 1 {
					t.Fatalf("each generation must have its own valid ultrasonic evidence, got generation counts %v", seen)
				}
			},
		},
		{
			name: "one real call retains retry and successful close semantics",
			run: func(t *testing.T) {
				repo := persistence.NewMemoryStore("log", "snapshot")
				adapter := &modelRetryAdapter{}
				svc := NewService(repo, NewManualClock(0), DefaultCatalog(), adapter)
				createAndLock(t, svc, "T1")
				mustHandle(t, svc, Command{Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "acquire", ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 100})

				cmd := Command{Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "scan", Generation: 0, ResourceID: "L1", Value: 41}
				if _, err := svc.Handle(nil, cmd); !domain.IsCode(err, domain.CodeDeviceRetryPending) {
					t.Fatalf("first failed attempt must remain retryable, got %v", err)
				}
				if _, err := svc.Handle(nil, cmd); err != nil {
					t.Fatalf("retry of the same call must succeed, got %v", err)
				}
				if _, err := svc.Handle(nil, cmd); err != nil {
					t.Fatalf("replay after successful close must return the recorded result, got %v", err)
				}

				if len(adapter.calls) != 2 || adapter.calls[0].CallID != adapter.calls[1].CallID {
					t.Fatalf("retry must address one stable device call and closed replay must not call the device again: %+v", adapter.calls)
				}
				evidence, err := svc.ListEvidence("T1")
				if err != nil {
					t.Fatalf("list evidence: %v", err)
				}
				valid := 0
				for _, event := range evidence {
					if event.Type == domain.EventUltrasonic && event.Valid && event.Generation == 0 {
						valid++
					}
				}
				if valid != 1 {
					t.Fatalf("successful close must create exactly one valid reading, got %d", valid)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.run)
	}
}
