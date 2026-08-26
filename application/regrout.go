package application

import (
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

// applyRegrout isolates the current generation and, via a compare-and-swap on
// the Isolated flag, creates exactly one successor generation with a freshly
// computed recheck set. Concurrent re-grout requests for the same generation
// lose with GENERATION_CONFLICT.
func (s *Service) applyRegrout(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	plan := state.Plans[cmd.TaskID]
	if plan == nil {
		return Result{}, domain.NewError(domain.CodeInvalidTopology, "plan missing")
	}
	if cmd.Generation != task.CurrentGeneration {
		return Result{}, domain.NewError(domain.CodeGenerationConflict, "generation is not current")
	}
	cur := s.generation(state, cmd.TaskID, cmd.Generation)
	if cur == nil {
		return Result{}, domain.NewError(domain.CodeGenerationConflict, "generation state missing")
	}
	if cur.Isolated {
		return Result{}, domain.NewError(domain.CodeGenerationConflict, "generation already isolated")
	}
	if len(cmd.Defects) == 0 {
		return Result{}, domain.NewError(domain.CodeEvidenceIncomplete, "no defects supplied for re-grout")
	}

	recheck := domain.ComputeRecheckSet(*plan, cmd.Defects)
	nextGen := task.CurrentGeneration + 1
	cur.Isolated = true

	next := *cur
	next.TaskID = cmd.TaskID
	next.Generation = nextGen
	next.ParentGeneration = task.CurrentGeneration
	next.Reason = cmd.Reason
	next.RecheckSet = recheck
	next.Defects = append([]domain.SocketID(nil), cmd.Defects...)
	next.Isolated = false
	state.Generations[persistence.GenKey(cmd.TaskID, nextGen)] = &next
	task.CurrentGeneration = nextGen
	task.Stage = domain.StagePrepared

	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		Generation:    nextGen,
		Type:          domain.EventRegrout,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digestOf(cmd.Reason),
		Valid:         true,
	})
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}
