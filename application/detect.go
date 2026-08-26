package application

import (
	"precast-wall-grout-support-release/devices"
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

// applyStrength records a cured specimen's strength result directly. A value
// below the locked release threshold marks the strength conclusion as failed
// and becomes a defect trigger for re-grout.
func (s *Service) applyStrength(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
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
	gen := s.generation(state, cmd.TaskID, task.CurrentGeneration)
	if gen == nil {
		return Result{}, domain.NewError(domain.CodeGenerationConflict, "generation state missing")
	}
	plan := state.Plans[cmd.TaskID]
	if plan == nil {
		return Result{}, domain.NewError(domain.CodeInvalidTopology, "plan missing")
	}
	if cmd.Value < plan.ReleaseThreshold {
		gen.StrengthOK = false
	} else {
		gen.StrengthOK = true
	}
	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		SpecimenID:    cmd.SpecimenID,
		Generation:    task.CurrentGeneration,
		Type:          domain.EventStrength,
		FixedValue:    cmd.Value,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digestOf(cmd.SpecimenID),
		Valid:         true,
	})
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// applyUltrasonic drives an ultrasonic void detection through the device
// adapter. It requires a valid channel lease for the scanned line.
func (s *Service) applyUltrasonic(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	return s.detect(state, cmd, now, "ultrasonic", domain.EventUltrasonic)
}

// applyEndoscope drives an endoscope grout-fill detection through the device
// adapter. It requires a valid channel lease for the inspected hole.
func (s *Service) applyEndoscope(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	return s.detect(state, cmd, now, "endoscope", domain.EventEndoscope)
}

// applyLeak records a seal-leak inspection result directly; a detected leak
// marks the leak conclusion failed and triggers re-grout.
func (s *Service) applyLeak(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
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
	gen := s.generation(state, cmd.TaskID, task.CurrentGeneration)
	if gen == nil {
		return Result{}, domain.NewError(domain.CodeGenerationConflict, "generation state missing")
	}
	leaked := cmd.Value > 0
	if leaked {
		gen.LeakOK = false
	} else {
		gen.LeakOK = true
	}
	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		SocketID:      cmd.SocketID,
		Generation:    task.CurrentGeneration,
		Type:          domain.EventLeak,
		FixedValue:    cmd.Value,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digestOf(string(cmd.SocketID)),
		Valid:         true,
	})
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// detect implements the shared device-driven detection flow. A failed device
// attempt is persisted with a deterministic retry schedule and never produces a
// reading; only a validated success receipt closes the call and records
// evidence for the current generation.
func (s *Service) detect(state *persistence.State, cmd Command, now domain.LogicalTime, deviceName string, evType domain.EventType) (Result, error) {
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
	// A valid channel lease for the line/hole is required.
	lease, ok := state.Leases[persistence.LeaseKey(domain.ResourceChannel, cmd.ResourceID)]
	if !ok || lease.Holder != cmd.TaskID || lease.Expired(now) {
		return Result{}, domain.NewError(domain.CodeLeaseExpired, "no valid detection channel lease")
	}

	callID := deviceName + ":" + cmd.ResourceID
	call, exists := state.DeviceCalls[callID]
	if !exists {
		call = &domain.DeviceCall{
			CallID:        callID,
			Device:        deviceName,
			TaskID:        cmd.TaskID,
			Target:        cmd.ResourceID,
			Generation:    cmd.Generation,
			RequestDigest: digestOf(cmd.ResourceID),
		}
		state.DeviceCalls[callID] = call
	}
	if call.Closed {
		return Result{}, domain.NewError(domain.CodeDeviceRetryPending, "device call already closed")
	}

	outcome, _ := s.device.Call(*call)
	return s.recordOutcome(state, cmd, now, call, evType, outcome)
}

// recordOutcome processes a single device outcome for a call. Failures append a
// retry attempt and leave the call open; success validates and records evidence
// or archives a late receipt for an older generation.
func (s *Service) recordOutcome(state *persistence.State, cmd Command, now domain.LogicalTime, call *domain.DeviceCall, evType domain.EventType, outcome domain.DeviceOutcome) (Result, error) {
	task := state.Tasks[cmd.TaskID]
	seq := len(call.Attempts) + 1
	if outcome != domain.OutcomeSuccess {
		call.Attempts = append(call.Attempts, domain.DeviceAttempt{
			Sequence:    seq,
			Outcome:     outcome,
			NextRetryAt: devices.NextRetryAt(now, seq),
		})
		task.UpdatedAt = now
		task.AggregateVersion++
		return Result{}, domain.NewError(domain.CodeDeviceRetryPending, "device attempt "+string(outcome))
	}
	// Success: close the call and record a validated reading.
	call.Attempts = append(call.Attempts, domain.DeviceAttempt{Sequence: seq, Outcome: domain.OutcomeSuccess})
	call.Closed = true

	gen := s.generation(state, cmd.TaskID, cmd.Generation)
	late := gen == nil || cmd.Generation != task.CurrentGeneration
	if late {
		// Late receipt for an older generation is archived without changing the
		// current conclusion.
		s.appendEvidence(state, domain.EvidenceEvent{
			TaskID:        cmd.TaskID,
			WallPanel:     task.WallPanel,
			Generation:    cmd.Generation,
			Type:          evType,
			FixedValue:    cmd.Value,
			At:            now,
			Operator:      cmd.Operator,
			OperationID:   cmd.OperationID,
			ContentDigest: digestOf(cmd.ResourceID),
			Valid:         false,
		})
		task.UpdatedAt = now
		task.AggregateVersion++
		return s.taskResult(task, now), nil
	}

	if evType == domain.EventUltrasonic {
		gen.UltrasonicOK = cmd.Value >= s.releaseThreshold(state, cmd.TaskID)
	} else if evType == domain.EventEndoscope {
		gen.EndoscopeOK = cmd.Value >= s.releaseThreshold(state, cmd.TaskID)
	}
	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		Generation:    cmd.Generation,
		Type:          evType,
		FixedValue:    cmd.Value,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digestOf(cmd.ResourceID),
		Valid:         true,
	})
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

func (s *Service) releaseThreshold(state *persistence.State, taskID domain.TaskID) int64 {
	if plan := state.Plans[taskID]; plan != nil {
		return plan.ReleaseThreshold
	}
	return 0
}

func (s *Service) generation(state *persistence.State, taskID domain.TaskID, gen domain.Generation) *domain.GenerationState {
	return state.Generations[persistence.GenKey(taskID, gen)]
}

// eventTypeForDevice maps a device name to its evidence event type.
func eventTypeForDevice(deviceName string) domain.EventType {
	if deviceName == "ultrasonic" {
		return domain.EventUltrasonic
	}
	return domain.EventEndoscope
}

// RecordDeviceAttempt processes an asynchronous device receipt for an existing
// device call. It is the entry point behind the device-calls attempt endpoint;
// the synchronous detection command shares the same recordOutcome path.
func (s *Service) RecordDeviceAttempt(callID string, outcome domain.DeviceOutcome, value int64) (Result, error) {
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		state, err := s.repo.Load()
		if err != nil {
			return Result{}, err
		}
		call, ok := state.DeviceCalls[callID]
		if !ok {
			return Result{}, domain.NewError(domain.CodeDeviceRetryPending, "unknown device call")
		}
		next := state.Clone()
		now := s.clock.Now()
		cmd := Command{
			TaskID:     call.TaskID,
			Generation: call.Generation,
			ResourceID: call.Target,
			Value:      value,
			Operator:   "device",
		}
		res, err := s.recordOutcome(next, cmd, now, next.DeviceCalls[callID], eventTypeForDevice(call.Device), outcome)
		if err != nil && domain.IsCode(err, domain.CodeDeviceRetryPending) {
			next.Version = state.Version + 1
			if saveErr := s.repo.Save(next, state.Version); saveErr != nil {
				if domain.IsCode(saveErr, domain.CodeConcurrentModification) {
					continue
				}
				return Result{}, saveErr
			}
			return Result{}, err
		}
		if err != nil {
			return Result{}, err
		}
		next.Version = state.Version + 1
		if saveErr := s.repo.Save(next, state.Version); saveErr != nil {
			if domain.IsCode(saveErr, domain.CodeConcurrentModification) {
				continue
			}
			return Result{}, saveErr
		}
		return res, nil
	}
	return Result{}, domain.NewError(domain.CodeConcurrentModification, "retries exhausted")
}
