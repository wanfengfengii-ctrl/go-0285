package application

import (
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

// applyMix creates a mix batch and its volume ledger from integer inputs. All
// arithmetic uses checked fixed-point helpers so any overflow or imbalance
// rejects the whole command before the ledger is created.
func (s *Service) applyMix(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	if cmd.BatchID == "" {
		return Result{}, domain.NewError(domain.CodeMaterialImbalance, "batch id is required")
	}
	if _, ok := state.Batches[cmd.BatchID]; ok {
		return Result{}, domain.NewError(domain.CodeGenerationConflict, "batch already exists")
	}
	if err := domain.NonNegative(cmd.InputGrams); err != nil {
		return Result{}, err
	}
	if err := domain.NonNegative(cmd.WaterML); err != nil {
		return Result{}, err
	}
	if err := domain.NonNegative(cmd.LossML); err != nil {
		return Result{}, err
	}
	if cmd.InputGrams <= 0 {
		return Result{}, domain.NewError(domain.CodeMaterialImbalance, "input mass must be positive")
	}
	// Grout density is fixed at 1 g/ml for the fixed-point ledger.
	inputVolume := cmd.InputGrams

	// Water ratio check against catalog water rules (ppm).
	waterPPM, err := domain.MulDiv(cmd.WaterML, domain.ScalePPM, cmd.InputGrams)
	if err != nil {
		return Result{}, err
	}
	if waterPPM < s.catalog.WaterRules.MinRatioPPM || waterPPM > s.catalog.WaterRules.MaxRatioPPM {
		return Result{}, domain.NewError(domain.CodeMaterialImbalance, "water ratio outside catalog bounds")
	}

	// Loss ratio must not exceed the locked plan's loss ceiling.
	plan := state.Plans[cmd.TaskID]
	if plan == nil {
		return Result{}, domain.NewError(domain.CodeInvalidTopology, "plan missing")
	}
	total := inputVolume + cmd.WaterML
	lossPPM, err := domain.MulDiv(cmd.LossML, domain.ScalePPM, total)
	if err != nil {
		return Result{}, err
	}
	if lossPPM > plan.LossCeilingPPM {
		return Result{}, domain.NewError(domain.CodeMaterialImbalance, "loss exceeds loss ceiling")
	}

	if cmd.WorkTicks < s.catalog.WorkLimits.MinWorkTicks || cmd.WorkTicks > s.catalog.WorkLimits.MaxWorkTicks {
		return Result{}, domain.NewError(domain.CodeMaterialImbalance, "work time outside catalog bounds")
	}

	available, err := domain.Sub(total, cmd.LossML)
	if err != nil {
		return Result{}, err
	}
	deadline, err := domain.Add(int64(now), cmd.WorkTicks)
	if err != nil {
		return Result{}, err
	}

	state.Batches[cmd.BatchID] = &domain.MixBatch{
		ID:           cmd.BatchID,
		TaskID:       cmd.TaskID,
		Generation:   task.CurrentGeneration,
		InputGrams:   cmd.InputGrams,
		WaterML:      cmd.WaterML,
		LossML:       cmd.LossML,
		WorkDeadline: domain.LogicalTime(deadline),
	}
	state.Ledgers[cmd.BatchID] = &domain.VolumeLedger{
		BatchID:     cmd.BatchID,
		InputVolume: inputVolume,
		WaterML:     cmd.WaterML,
		LossML:      cmd.LossML,
		Available:   available,
		Unallocated: available,
	}

	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		Generation:    task.CurrentGeneration,
		Type:          domain.EventMix,
		FixedValue:    available,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digestOf(cmd.BatchID),
		Valid:         true,
	})

	if task.Stage == domain.StageLocked {
		task.Stage = domain.StagePrepared
	}
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// applySample deducts a specimen sample from the batch's unallocated volume.
func (s *Service) applySample(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	ledger, ok := state.Ledgers[cmd.BatchID]
	if !ok {
		return Result{}, domain.NewError(domain.CodeMaterialImbalance, "batch not found")
	}
	if err := domain.NonNegative(cmd.SampleML); err != nil {
		return Result{}, err
	}
	if cmd.SampleML > ledger.Unallocated {
		return Result{}, domain.NewError(domain.CodeVolumeOverclaim, "sample exceeds unallocated volume")
	}
	newUnallocated, err := domain.Sub(ledger.Unallocated, cmd.SampleML)
	if err != nil {
		return Result{}, err
	}
	newSample, err := domain.Add(ledger.SampleML, cmd.SampleML)
	if err != nil {
		return Result{}, err
	}
	newAvailable, err := domain.Sub(ledger.Available, cmd.SampleML)
	if err != nil {
		return Result{}, err
	}
	ledger.Unallocated = newUnallocated
	ledger.SampleML = newSample
	ledger.Available = newAvailable
	ledger.Version++

	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		SpecimenID:    cmd.SpecimenID,
		Generation:    task.CurrentGeneration,
		Type:          domain.EventSample,
		FixedValue:    cmd.SampleML,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digestOf(cmd.BatchID + cmd.SpecimenID),
		Valid:         true,
	})
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}
