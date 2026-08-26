package application

import (
	"testing"

	"precast-wall-grout-support-release/domain"
)

func TestModel_ClosedDeviceCallReceiptsDoNotChangeEffectiveEvidence(t *testing.T) {
	tests := []struct {
		name     string
		receipts []struct {
			outcome domain.DeviceOutcome
			value   int64
		}
	}{
		{
			name: "late success with a conflicting reading",
			receipts: []struct {
				outcome domain.DeviceOutcome
				value   int64
			}{{outcome: domain.OutcomeSuccess, value: 0}},
		},
		{
			name: "failure after success",
			receipts: []struct {
				outcome domain.DeviceOutcome
				value   int64
			}{{outcome: domain.OutcomeTimeout, value: 0}},
		},
		{
			name: "repeated successful delivery",
			receipts: []struct {
				outcome domain.DeviceOutcome
				value   int64
			}{
				{outcome: domain.OutcomeSuccess, value: 50},
				{outcome: domain.OutcomeSuccess, value: 50},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newTestEnv(t)
			createAndLock(t, svc, "T1")
			mustHandle(t, svc, Command{
				Type: CommandAcquireLease, TaskID: "T1", Operator: "op", OperationID: "channel",
				ResourceType: domain.ResourceChannel, ResourceID: "L1", LeaseTicks: 1000,
			})

			mustHandle(t, svc, Command{
				Type: CommandUltrasonic, TaskID: "T1", Operator: "op", OperationID: "ultrasonic-first",
				Generation: 0, ResourceID: "L1", Value: 50,
			})

			beforeDigest, err := svc.EvidenceDigest("T1", 0)
			if err != nil {
				t.Fatalf("evidence digest after first success: %v", err)
			}
			beforeEvidence, err := svc.ListEvidence("T1")
			if err != nil {
				t.Fatalf("evidence after first success: %v", err)
			}
			validUltrasonic := 0
			for _, event := range beforeEvidence {
				if event.Type == domain.EventUltrasonic && event.Valid {
					validUltrasonic++
				}
			}
			if validUltrasonic != 1 {
				t.Fatalf("first success must create exactly one valid ultrasonic reading, got %d", validUltrasonic)
			}

			for _, receipt := range tt.receipts {
				_, _ = svc.RecordDeviceAttempt("ultrasonic:L1", receipt.outcome, receipt.value)
			}

			afterDigest, err := svc.EvidenceDigest("T1", 0)
			if err != nil {
				t.Fatalf("evidence digest after closed-call receipt: %v", err)
			}
			if afterDigest != beforeDigest {
				t.Fatalf("closed-call receipt changed review digest: before=%s after=%s", beforeDigest, afterDigest)
			}

			afterEvidence, err := svc.ListEvidence("T1")
			if err != nil {
				t.Fatalf("evidence after closed-call receipt: %v", err)
			}
			validUltrasonic = 0
			for _, event := range afterEvidence {
				if event.Type == domain.EventUltrasonic && event.Valid {
					validUltrasonic++
				}
			}
			if validUltrasonic != 1 {
				t.Fatalf("closed call must retain one effective ultrasonic reading, got %d", validUltrasonic)
			}

			generations, err := svc.ListGenerations("T1")
			if err != nil {
				t.Fatalf("generations after closed-call receipt: %v", err)
			}
			if len(generations) != 1 || !generations[0].UltrasonicOK {
				t.Fatalf("closed-call receipt changed current conclusion: %+v", generations)
			}
		})
	}
}
