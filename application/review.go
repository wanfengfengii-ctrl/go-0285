package application

import (
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

// applyReview records a reviewer signature bound to the current evidence
// digest. The reviewer must hold a valid, currently-active qualification and
// may only sign once.
func (s *Service) applyReview(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	if !s.qualified(cmd.Operator, now) {
		return Result{}, domain.NewError(domain.CodeReviewerConflict, "reviewer qualification invalid")
	}
	digest := s.evidenceDigest(state, cmd.TaskID, task.CurrentGeneration)
	if cmd.EvidenceDigest != digest {
		return Result{}, domain.NewError(domain.CodeEvidenceIncomplete, "review digest does not match current evidence")
	}
	for _, r := range state.Reviews[cmd.TaskID] {
		if r.Reviewer == cmd.Operator {
			return Result{}, domain.NewError(domain.CodeReviewerConflict, "reviewer already signed")
		}
	}
	state.Reviews[cmd.TaskID] = append(state.Reviews[cmd.TaskID], domain.Review{
		Reviewer:       cmd.Operator,
		EvidenceDigest: digest,
		At:             now,
	})
	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		Generation:    task.CurrentGeneration,
		Type:          domain.EventReview,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digest,
		Valid:         true,
	})
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// applyTerminal publishes the single irreversible terminal decision. Release,
// quarantine and cancel compete through the same terminal slot; only one
// transaction can occupy it. A release additionally requires every closure
// gate to pass and produces the unique release credential.
func (s *Service) applyTerminal(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if task.Terminal != nil {
		return Result{}, domain.NewError(domain.CodeTerminalConflict, "terminal slot already occupied")
	}
	switch cmd.TerminalType {
	case domain.TerminalRelease:
		if err := s.releaseGate(state, cmd.TaskID, task.CurrentGeneration); err != nil {
			return Result{}, err
		}
		credential := digestOf(string(task.ID) + itoa(uint64(task.CurrentGeneration)))
		task.Terminal = &domain.TerminalDecision{
			Type:              domain.TerminalRelease,
			At:                now,
			ReleaseCredential: credential,
		}
	case domain.TerminalQuarantine:
		task.Terminal = &domain.TerminalDecision{Type: domain.TerminalQuarantine, At: now}
	case domain.TerminalCancel:
		task.Terminal = &domain.TerminalDecision{Type: domain.TerminalCancel, At: now}
	default:
		return Result{}, domain.NewError(domain.CodeTerminalConflict, "unknown terminal type")
	}
	task.Stage = domain.StageTerminal
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// releaseGate verifies every closure condition for a support release: a
// complete pour prefix, conservation of every task ledger, all detection
// conclusions satisfied, and two distinct valid reviewer signatures bound to
// the current evidence digest.
func (s *Service) releaseGate(state *persistence.State, taskID domain.TaskID, gen domain.Generation) error {
	genState := s.generation(state, taskID, gen)
	if genState == nil {
		return domain.NewError(domain.CodeEvidenceIncomplete, "generation state missing")
	}
	plan := state.Plans[taskID]
	if plan == nil {
		return domain.NewError(domain.CodeInvalidTopology, "plan missing")
	}
	if genState.Step != len(domain.PourSequence(*plan)) {
		return domain.NewError(domain.CodeEvidenceIncomplete, "pour prefix incomplete")
	}
	for _, batch := range state.Batches {
		if batch.TaskID != taskID {
			continue
		}
		if l := state.Ledgers[batch.ID]; l != nil {
			if err := l.CheckConservation(); err != nil {
				return domain.NewError(domain.CodeMaterialImbalance, "ledger imbalance: "+err.Error())
			}
		}
	}
	if !genState.StrengthOK {
		return domain.NewError(domain.CodeEvidenceIncomplete, "strength evidence incomplete")
	}
	if !genState.UltrasonicOK {
		return domain.NewError(domain.CodeEvidenceIncomplete, "ultrasonic evidence incomplete")
	}
	if !genState.EndoscopeOK {
		return domain.NewError(domain.CodeEvidenceIncomplete, "endoscope evidence incomplete")
	}
	if !genState.LeakOK {
		return domain.NewError(domain.CodeEvidenceIncomplete, "leak evidence incomplete")
	}
	digest := s.evidenceDigest(state, taskID, gen)
	reviewers := map[domain.Operator]bool{}
	for _, r := range state.Reviews[taskID] {
		if r.EvidenceDigest == digest {
			reviewers[r.Reviewer] = true
		}
	}
	if len(reviewers) < 2 {
		return domain.NewError(domain.CodeReviewerConflict, "two distinct valid reviews required")
	}
	return nil
}

// qualified reports whether an operator holds a valid, currently-active
// qualification in the catalog.
func (s *Service) qualified(op domain.Operator, now domain.LogicalTime) bool {
	for _, p := range s.catalog.Personnel {
		if p.PersonID == string(op) && p.IsValid(now) {
			return true
		}
	}
	return false
}

// evidenceDigest computes the current evidence digest for a generation by
// hashing the ordered valid evidence events, excluding review signatures which
// are meta-records signed against the digest rather than part of it.
func (s *Service) evidenceDigest(state *persistence.State, taskID domain.TaskID, gen domain.Generation) string {
	var acc string
	for _, ev := range state.Evidence {
		if ev.TaskID == taskID && ev.Generation == gen && ev.Valid && ev.Type != domain.EventReview {
			acc += string(ev.Type) + ":" + ev.ID + ";"
		}
	}
	return digestOf(acc)
}
