package persistence

import (
	"strconv"

	"precast-wall-grout-support-release/domain"
)

// LeaseKey encodes a resource lease map key as a JSON-serializable string.
func LeaseKey(t domain.ResourceType, id string) string {
	return string(t) + "/" + id
}

// GenKey encodes a task+generation map key as a JSON-serializable string.
func GenKey(taskID domain.TaskID, gen domain.Generation) string {
	return string(taskID) + ":" + strconv.FormatInt(int64(gen), 10)
}

// IdempotencyRecord captures the normalized content digest and prior result of
// a command so an equal operation id plus equal content returns the original
// result, while equal id plus different content returns IDEMPOTENCY_CONFLICT.
type IdempotencyRecord struct {
	OperationID   domain.OperationID
	ContentDigest string
	Result        []byte
}

// State is the complete, serializable snapshot of the backend. It is the unit
// of persistence: commands mutate a copy and commit it through a conditional
// write guarded by Version.
type State struct {
	Version int64

	Catalog domain.CatalogSnapshot

	Tasks       map[domain.TaskID]*domain.InspectionTask
	Plans       map[domain.TaskID]*domain.LockedPlan
	Batches     map[string]*domain.MixBatch
	Ledgers     map[string]*domain.VolumeLedger
	Leases      map[string]*domain.Lease
	Generations map[string]*domain.GenerationState
	DeviceCalls map[string]*domain.DeviceCall
	Reviews     map[domain.TaskID][]domain.Review
	Terminals   map[domain.TaskID]*domain.TerminalDecision
	Evidence    []domain.EvidenceEvent
	Idempotency map[domain.TaskID]map[domain.OperationID]IdempotencyRecord

	NextSequence uint64
	NextEventID  uint64
}

// NewState returns an empty, version-zero State with all maps initialized.
func NewState() *State {
	return &State{
		Tasks:       map[domain.TaskID]*domain.InspectionTask{},
		Plans:       map[domain.TaskID]*domain.LockedPlan{},
		Batches:     map[string]*domain.MixBatch{},
		Ledgers:     map[string]*domain.VolumeLedger{},
		Leases:      map[string]*domain.Lease{},
		Generations: map[string]*domain.GenerationState{},
		DeviceCalls: map[string]*domain.DeviceCall{},
		Reviews:     map[domain.TaskID][]domain.Review{},
		Terminals:   map[domain.TaskID]*domain.TerminalDecision{},
		Idempotency: map[domain.TaskID]map[domain.OperationID]IdempotencyRecord{},
	}
}

// Clone returns a deep copy of the state so an in-flight command can never
// corrupt a committed snapshot on failure.
func (s *State) Clone() *State {
	c := NewState()
	c.Version = s.Version
	c.Catalog = s.Catalog
	c.NextSequence = s.NextSequence
	c.NextEventID = s.NextEventID
	for k, v := range s.Tasks {
		cp := *v
		c.Tasks[k] = &cp
	}
	for k, v := range s.Plans {
		cp := *v
		c.Plans[k] = &cp
	}
	for k, v := range s.Batches {
		cp := *v
		c.Batches[k] = &cp
	}
	for k, v := range s.Ledgers {
		cp := *v
		c.Ledgers[k] = &cp
	}
	for k, v := range s.Leases {
		cp := *v
		c.Leases[k] = &cp
	}
	for k, v := range s.Generations {
		cp := *v
		c.Generations[k] = &cp
	}
	for k, v := range s.DeviceCalls {
		cp := *v
		c.DeviceCalls[k] = &cp
	}
	for k, v := range s.Reviews {
		c.Reviews[k] = append([]domain.Review(nil), v...)
	}
	for k, v := range s.Terminals {
		cp := *v
		c.Terminals[k] = &cp
	}
	for k, m := range s.Idempotency {
		inner := map[domain.OperationID]IdempotencyRecord{}
		for ok, ov := range m {
			inner[ok] = ov
		}
		c.Idempotency[k] = inner
	}
	c.Evidence = append([]domain.EvidenceEvent(nil), s.Evidence...)
	return c
}
