package application

import (
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

// applyInletStart begins grouting at the next inlet port.
func (s *Service) applyInletStart(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	return s.pourEvent(state, cmd, now, domain.EventInletStart)
}

// applyOutletStable records a stable outlet with a pressure reading.
func (s *Service) applyOutletStable(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	return s.pourEvent(state, cmd, now, domain.EventOutletStable)
}

// applyOutletSeal seals an outlet port.
func (s *Service) applyOutletSeal(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	return s.pourEvent(state, cmd, now, domain.EventOutletSeal)
}

// applyPortSwitch switches to the next inlet port.
func (s *Service) applyPortSwitch(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	return s.pourEvent(state, cmd, now, domain.EventPortSwitch)
}

// pourEvent validates and appends one continuous-pour step. Only the next legal
// event can extend the prefix; out-of-order seals, duplicate ports, wrong
// generations, expired leases or slurry all reject without writing evidence.
func (s *Service) pourEvent(state *persistence.State, cmd Command, now domain.LogicalTime, want domain.EventType) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	if cmd.Generation != task.CurrentGeneration {
		return Result{}, domain.NewError(domain.CodeGenerationConflict, "generation is not current")
	}
	gen, ok := state.Generations[persistence.GenKey(cmd.TaskID, task.CurrentGeneration)]
	if !ok {
		return Result{}, domain.NewError(domain.CodeGenerationConflict, "generation state missing")
	}
	plan, ok := state.Plans[cmd.TaskID]
	if !ok {
		return Result{}, domain.NewError(domain.CodeInvalidTopology, "plan missing")
	}
	seq := domain.PourSequence(*plan)
	if gen.Step >= len(seq) {
		return Result{}, domain.NewError(domain.CodePrefixViolation, "pour prefix already complete")
	}
	next := seq[gen.Step]
	if next.Type != want {
		if want == domain.EventOutletSeal {
			return Result{}, domain.NewError(domain.CodePortSealOutOfOrder, "outlet must be stable before sealing")
		}
		return Result{}, domain.NewError(domain.CodePrefixViolation, "pour event out of order")
	}
	if cmd.PortID != next.PortID {
		if want == domain.EventOutletSeal {
			return Result{}, domain.NewError(domain.CodePortSealOutOfOrder, "sealing the wrong port")
		}
		return Result{}, domain.NewError(domain.CodePrefixViolation, "port does not match expected sequence")
	}
	// A valid pump lease held by this task is required to pour.
	if !s.hasActiveLease(state, cmd.TaskID, domain.ResourcePump, now) {
		return Result{}, domain.NewError(domain.CodeLeaseExpired, "no valid pump lease")
	}
	// Slurry must still be within its working time.
	batch, ok := state.Batches[cmd.BatchID]
	if !ok {
		return Result{}, domain.NewError(domain.CodeMaterialImbalance, "batch not found")
	}
	if now >= batch.WorkDeadline {
		return Result{}, domain.NewError(domain.CodeSlurryExpired, "mix batch working time expired")
	}
	// Inlet start consumes reserved volume.
	if want == domain.EventInletStart {
		ledger, ok := state.Ledgers[cmd.BatchID]
		if !ok {
			return Result{}, domain.NewError(domain.CodeMaterialImbalance, "ledger not found")
		}
		if err := domain.NonNegative(cmd.VolumeML); err != nil {
			return Result{}, err
		}
		if cmd.VolumeML > ledger.Reserved {
			return Result{}, domain.NewError(domain.CodeVolumeOverclaim, "pour exceeds reserved volume")
		}
		newReserved, err := domain.Sub(ledger.Reserved, cmd.VolumeML)
		if err != nil {
			return Result{}, err
		}
		newPoured, err := domain.Add(ledger.Poured, cmd.VolumeML)
		if err != nil {
			return Result{}, err
		}
		ledger.Reserved = newReserved
		ledger.Poured = newPoured
		ledger.Version++
		gen.Prefix = append(gen.Prefix, next.PortID)
	}

	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		SocketID:      next.SocketID,
		PortID:        next.PortID,
		Generation:    task.CurrentGeneration,
		Type:          want,
		FixedValue:    cmd.Pressure,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digestOf(string(next.PortID)),
		Valid:         true,
	})
	gen.Step++
	if task.Stage == domain.StagePrepared {
		task.Stage = domain.StagePoured
	}
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// hasActiveLease reports whether the task holds a non-expired lease of the
// given resource type at the given logical time.
func (s *Service) hasActiveLease(state *persistence.State, taskID domain.TaskID, typ domain.ResourceType, now domain.LogicalTime) bool {
	for _, l := range state.Leases {
		if l.Key.Type == typ && l.Holder == taskID && !l.Expired(now) {
			return true
		}
	}
	return false
}
