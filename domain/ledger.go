package domain

// MixBatch is a single mixing batch with its timing and consumption pool. All
// volumes are integer millilitres; masses are integer grams.
type MixBatch struct {
	ID           string
	TaskID       TaskID
	Generation   Generation
	InputGrams   int64
	WaterML      int64
	LossML       int64
	SampleML     int64
	WorkDeadline LogicalTime
}

// VolumeLedger tracks the conservation of a mix batch. It must always satisfy:
//
//	Available = InputVolume + WaterML - LossML - SampleML
//	Available = Unallocated + Reserved + Poured
type VolumeLedger struct {
	BatchID     string
	InputVolume int64
	WaterML     int64
	LossML      int64
	SampleML    int64
	Available   int64
	Unallocated int64
	Reserved    int64
	Poured      int64
	Version     int64
}

// CheckConservation verifies both ledger invariants and returns a
// MATERIAL_IMBALANCE error describing any violation.
func (l VolumeLedger) CheckConservation() error {
	avail := l.InputVolume + l.WaterML - l.LossML - l.SampleML
	if avail != l.Available {
		return NewError(CodeMaterialImbalance, "available volume does not equal inputs minus losses and samples")
	}
	if l.Available != l.Unallocated+l.Reserved+l.Poured {
		return NewError(CodeMaterialImbalance, "available volume does not equal unallocated plus reserved plus poured")
	}
	return nil
}

// Lease is a time-bounded hold on a leaseable resource. A lease is expired when
// now >= ExpiresAt.
type Lease struct {
	Key         ResourceKey
	Holder      TaskID
	OperationID OperationID
	AcquiredAt  LogicalTime
	ExpiresAt   LogicalTime
	Version     int64
}

// Expired reports whether the lease has expired at the given logical time.
func (l Lease) Expired(now LogicalTime) bool {
	return now >= l.ExpiresAt
}
