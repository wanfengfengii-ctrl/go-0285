package application

import (
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

// applyLock validates and fixes the immutable lock plan. Any mismatch rejects
// the whole command with reasons ordered by stable domain key.
func (s *Service) applyLock(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if task.Stage != domain.StageCreated {
		return Result{}, domain.NewError(domain.CodeGenerationConflict, "task is already locked")
	}
	if cmd.CatalogVersion != s.catalog.Version || cmd.RuleDigest != domain.CatalogDigest(s.catalog) {
		return Result{}, domain.NewError(domain.CodeStaleRuleDigest, "catalog version or rule digest is stale")
	}

	plan := domain.LockedPlan{
		WallPosition:        domain.WallPosition{Building: cmd.Building, Level: cmd.Level, WallID: cmd.WallPanel},
		Connections:         make([]domain.Connection, 0, len(cmd.Connections)),
		PortNodes:           make([]domain.PortNode, 0, len(cmd.PortNodes)),
		PortEdges:           make([]domain.PortEdge, 0, len(cmd.PortEdges)),
		SlurryPaths:         cmd.SlurryPaths,
		MaterialBatch:       cmd.MaterialBatch,
		WaterBatch:          cmd.WaterBatch,
		TheoreticalVolumeML: cmd.TheoreticalVolumeML,
		LossCeilingPPM:      cmd.LossCeilingPPM,
		SpecimenPlan:        make([]domain.Specimen, 0, len(cmd.Specimens)),
		UltrasonicLines:     cmd.UltrasonicLines,
		EndoscopeHoles:      cmd.EndoscopeHoles,
		ReleaseThreshold:    cmd.ReleaseThreshold,
	}
	for _, c := range cmd.Connections {
		plan.Connections = append(plan.Connections, domain.Connection{RebarSpec: c.RebarSpec, SleeveSpec: c.SleeveSpec, SocketID: c.SocketID})
	}
	for _, n := range cmd.PortNodes {
		plan.PortNodes = append(plan.PortNodes, domain.PortNode{ID: n.ID, Kind: n.Kind, SocketID: n.SocketID})
	}
	for _, e := range cmd.PortEdges {
		plan.PortEdges = append(plan.PortEdges, domain.PortEdge{From: e.From, To: e.To})
	}
	for _, sp := range cmd.Specimens {
		plan.SpecimenPlan = append(plan.SpecimenPlan, domain.Specimen{ID: sp.ID, CureTicks: sp.CureTicks})
	}

	if issues := domain.ValidateLockPlan(task.Building, task.Level, task.WallPanel, plan, s.catalog); len(issues) > 0 {
		err := domain.NewError(issues[0].Code, "lock plan rejected")
		reasons := make([]string, 0, len(issues))
		for _, is := range issues {
			reasons = append(reasons, is.Msg)
		}
		return Result{}, err.WithReasons(reasons...)
	}

	task.LockDigest = cmd.RuleDigest
	task.Stage = domain.StageLocked
	task.UpdatedAt = now
	task.AggregateVersion++
	state.Plans[cmd.TaskID] = &plan
	state.Generations[persistence.GenKey(cmd.TaskID, 0)] = &domain.GenerationState{
		TaskID:           cmd.TaskID,
		Generation:       0,
		ParentGeneration: -1,
		Prefix:           []domain.PortID{},
	}
	return s.taskResult(task, now), nil
}

// applyMaterialCheck appends a material certificate verification event.
func (s *Service) applyMaterialCheck(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		Generation:    task.CurrentGeneration,
		Type:          domain.EventMaterialCheck,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digestOf(cmd.MaterialBatch),
		Valid:         true,
	})
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// applyPosition appends a wall positioning evidence event.
func (s *Service) applyPosition(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		Generation:    task.CurrentGeneration,
		Type:          domain.EventPosition,
		At:            now,
		Operator:      cmd.Operator,
		OperationID:   cmd.OperationID,
		ContentDigest: digestOf(string(task.ID)),
		Valid:         true,
	})
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// applySeal appends a joint sealing evidence event.
func (s *Service) applySeal(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	s.appendEvidence(state, domain.EvidenceEvent{
		TaskID:        cmd.TaskID,
		WallPanel:     task.WallPanel,
		SocketID:      cmd.SocketID,
		Generation:    task.CurrentGeneration,
		Type:          domain.EventSeal,
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

// guardActive rejects commands against a task that has reached a terminal state.
func (s *Service) guardActive(task *domain.InspectionTask) error {
	if task.Terminal != nil {
		return domain.NewError(domain.CodeTerminalConflict, "task is terminal")
	}
	return nil
}
