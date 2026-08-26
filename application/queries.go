package application

import (
	"sort"

	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

// GetTask returns the current inspection task aggregate.
func (s *Service) GetTask(id domain.TaskID) (*domain.InspectionTask, error) {
	state, err := s.repo.Load()
	if err != nil {
		return nil, err
	}
	t, ok := state.Tasks[id]
	if !ok {
		return nil, domain.NewError(domain.CodeEvidenceIncomplete, "task not found")
	}
	cp := *t
	return &cp, nil
}

// ListEvidence returns the immutable evidence graph for a task in stable order.
func (s *Service) ListEvidence(taskID domain.TaskID) ([]domain.EvidenceEvent, error) {
	state, err := s.repo.Load()
	if err != nil {
		return nil, err
	}
	out := make([]domain.EvidenceEvent, 0)
	for _, ev := range state.Evidence {
		if ev.TaskID == taskID {
			out = append(out, ev)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ListGenerations returns every generation state for a task in generation order.
func (s *Service) ListGenerations(taskID domain.TaskID) ([]domain.GenerationState, error) {
	state, err := s.repo.Load()
	if err != nil {
		return nil, err
	}
	out := make([]domain.GenerationState, 0)
	for _, g := range state.Generations {
		if g.TaskID == taskID {
			out = append(out, *g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Generation < out[j].Generation })
	return out, nil
}

// GetLedger returns a mix batch's volume ledger.
func (s *Service) GetLedger(batchID string) (*domain.VolumeLedger, error) {
	state, err := s.repo.Load()
	if err != nil {
		return nil, err
	}
	l, ok := state.Ledgers[batchID]
	if !ok {
		return nil, domain.NewError(domain.CodeMaterialImbalance, "ledger not found")
	}
	cp := *l
	return &cp, nil
}

// GetLease returns the current lease for a resource, or nil when none exists.
func (s *Service) GetLease(typ domain.ResourceType, id string) (*domain.Lease, error) {
	state, err := s.repo.Load()
	if err != nil {
		return nil, err
	}
	l, ok := state.Leases[persistence.LeaseKey(typ, id)]
	if !ok {
		return nil, nil
	}
	cp := *l
	return &cp, nil
}

// GetDecision returns the terminal decision for a task, or nil when absent.
func (s *Service) GetDecision(taskID domain.TaskID) (*domain.TerminalDecision, error) {
	state, err := s.repo.Load()
	if err != nil {
		return nil, err
	}
	t, ok := state.Tasks[taskID]
	if !ok {
		return nil, domain.NewError(domain.CodeEvidenceIncomplete, "task not found")
	}
	if t.Terminal == nil {
		return nil, nil
	}
	cp := *t.Terminal
	return &cp, nil
}

// Health returns the underlying repository health summary.
func (s *Service) Health() persistence.Health {
	return s.repo.Health()
}

// EvidenceDigest returns the current evidence digest for a task generation,
// used by reviewers to sign the fixed review digest.
func (s *Service) EvidenceDigest(taskID domain.TaskID, gen domain.Generation) (string, error) {
	state, err := s.repo.Load()
	if err != nil {
		return "", err
	}
	return s.evidenceDigest(state, taskID, gen), nil
}
