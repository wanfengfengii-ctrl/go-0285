package domain

import "testing"

func TestVolumeLedgerConservation(t *testing.T) {
	tests := []struct {
		name string
		l    VolumeLedger
		ok   bool
	}{
		{
			name: "balanced",
			l: VolumeLedger{
				InputVolume: 1000, WaterML: 200, LossML: 50, SampleML: 150,
				Available: 1000, Unallocated: 1000, Reserved: 0, Poured: 0,
			},
			ok: true,
		},
		{
			name: "reserved and poured balanced",
			l: VolumeLedger{
				InputVolume: 1000, WaterML: 200, LossML: 100, SampleML: 100,
				Available: 1000, Unallocated: 400, Reserved: 300, Poured: 300,
			},
			ok: true,
		},
		{
			name: "available mismatch",
			l: VolumeLedger{
				InputVolume: 1000, WaterML: 200, LossML: 50, SampleML: 150,
				Available: 999, Unallocated: 999, Reserved: 0, Poured: 0,
			},
			ok: false,
		},
		{
			name: "allocation mismatch",
			l: VolumeLedger{
				InputVolume: 1000, WaterML: 200, LossML: 0, SampleML: 0,
				Available: 1200, Unallocated: 1000, Reserved: 100, Poured: 0,
			},
			ok: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.l.CheckConservation()
			if tt.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.ok {
				if err == nil {
					t.Fatal("expected MATERIAL_IMBALANCE")
				}
				if !IsCode(err, CodeMaterialImbalance) {
					t.Fatalf("expected MATERIAL_IMBALANCE, got %v", err)
				}
			}
		})
	}
}

func TestLeaseExpiryBoundary(t *testing.T) {
	l := Lease{ExpiresAt: 10}
	if l.Expired(9) {
		t.Fatal("lease should not be expired before expiresAt")
	}
	// now >= expiresAt is expired, so now == expiresAt is already expired.
	if !l.Expired(10) {
		t.Fatal("lease should be expired at now == expiresAt")
	}
	if !l.Expired(11) {
		t.Fatal("lease should be expired after expiresAt")
	}
}
