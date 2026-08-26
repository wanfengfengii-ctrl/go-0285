package application

import (
	"reflect"
	"testing"

	"precast-wall-grout-support-release/domain"
)

func TestModel_SlurryReservationIsolation(t *testing.T) {
	tests := []struct {
		name      string
		pourer    domain.TaskID
		wantError domain.Code
	}{
		{
			name:      "another task cannot consume the reservation",
			pourer:    "T2",
			wantError: domain.CodeVolumeOverclaim,
		},
		{
			name:   "reserving task can consume its reservation",
			pourer: "T1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newTestEnv(t)
			createAndLock(t, svc, "T1")
			createAndLock(t, svc, "T2")
			mixBatch(t, svc, "T1", "shared-batch")
			mustHandle(t, svc, Command{
				Type:        CommandReserveSlurry,
				TaskID:      "T1",
				Operator:    "op",
				OperationID: "reserve-t1",
				BatchID:     "shared-batch",
				VolumeML:    1000,
			})
			mustHandle(t, svc, Command{
				Type:         CommandAcquireLease,
				TaskID:       tt.pourer,
				Operator:     "op",
				OperationID:  "lease-" + domain.OperationID(tt.pourer),
				ResourceType: domain.ResourcePump,
				ResourceID:   "pump-" + string(tt.pourer),
				LeaseTicks:   100,
			})

			ledgerBefore, err := svc.GetLedger("shared-batch")
			if err != nil {
				t.Fatalf("get ledger before inlet_start: %v", err)
			}
			generationsBefore, err := svc.ListGenerations(tt.pourer)
			if err != nil {
				t.Fatalf("list generations before inlet_start: %v", err)
			}
			evidenceBefore, err := svc.ListEvidence(tt.pourer)
			if err != nil {
				t.Fatalf("list evidence before inlet_start: %v", err)
			}

			_, err = svc.Handle(nil, Command{
				Type:        CommandInletStart,
				TaskID:      tt.pourer,
				Operator:    "op",
				OperationID: "inlet-" + domain.OperationID(tt.pourer),
				Generation:  0,
				BatchID:     "shared-batch",
				PortID:      "P1",
				VolumeML:    1000,
			})

			ledgerAfter, getErr := svc.GetLedger("shared-batch")
			if getErr != nil {
				t.Fatalf("get ledger after inlet_start: %v", getErr)
			}
			generationsAfter, getErr := svc.ListGenerations(tt.pourer)
			if getErr != nil {
				t.Fatalf("list generations after inlet_start: %v", getErr)
			}
			evidenceAfter, getErr := svc.ListEvidence(tt.pourer)
			if getErr != nil {
				t.Fatalf("list evidence after inlet_start: %v", getErr)
			}

			if tt.wantError != "" {
				if !domain.IsCode(err, tt.wantError) {
					t.Fatalf("inlet_start error = %v, want %s", err, tt.wantError)
				}
				if !reflect.DeepEqual(ledgerAfter, ledgerBefore) {
					t.Fatalf("rejected inlet_start changed ledger: before=%+v after=%+v", ledgerBefore, ledgerAfter)
				}
				if !reflect.DeepEqual(generationsAfter, generationsBefore) {
					t.Fatalf("rejected inlet_start changed pour prefix: before=%+v after=%+v", generationsBefore, generationsAfter)
				}
				if !reflect.DeepEqual(evidenceAfter, evidenceBefore) {
					t.Fatalf("rejected inlet_start changed evidence: before=%+v after=%+v", evidenceBefore, evidenceAfter)
				}
				return
			}

			if err != nil {
				t.Fatalf("owner inlet_start: %v", err)
			}
			if ledgerAfter.Unallocated != 5800 || ledgerAfter.Reserved != 0 || ledgerAfter.Poured != 1000 {
				t.Fatalf("owner consumption produced ledger %+v", ledgerAfter)
			}
			if len(generationsAfter) != 1 || generationsAfter[0].Step != 1 || !reflect.DeepEqual(generationsAfter[0].Prefix, []domain.PortID{"P1"}) {
				t.Fatalf("owner inlet_start produced generations %+v", generationsAfter)
			}
			if len(evidenceAfter) != len(evidenceBefore)+1 || evidenceAfter[len(evidenceAfter)-1].Type != domain.EventInletStart {
				t.Fatalf("owner inlet_start produced evidence %+v", evidenceAfter)
			}
		})
	}
}
