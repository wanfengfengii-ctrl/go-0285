package application

import (
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

// applyAcquireLease atomically acquires a lease on a resource. An existing,
// non-expired lease held by anyone (including this task) is a conflict.
func (s *Service) applyAcquireLease(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	key := domain.ResourceKey{Type: cmd.ResourceType, ID: cmd.ResourceID}
	kstr := persistence.LeaseKey(cmd.ResourceType, cmd.ResourceID)
	if existing, ok := state.Leases[kstr]; ok && !existing.Expired(now) {
		return Result{}, domain.NewError(domain.CodeLeaseConflict, "resource already leased")
	}
	expires, err := domain.Add(int64(now), cmd.LeaseTicks)
	if err != nil {
		return Result{}, err
	}
	state.Leases[kstr] = &domain.Lease{
		Key:         key,
		Holder:      cmd.TaskID,
		OperationID: cmd.OperationID,
		AcquiredAt:  now,
		ExpiresAt:   domain.LogicalTime(expires),
	}
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// applyRenewLease extends a lease held by this task.
func (s *Service) applyRenewLease(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	kstr := persistence.LeaseKey(cmd.ResourceType, cmd.ResourceID)
	lease, ok := state.Leases[kstr]
	if !ok {
		return Result{}, domain.NewError(domain.CodeLeaseConflict, "no such lease")
	}
	if lease.Holder != cmd.TaskID {
		return Result{}, domain.NewError(domain.CodeLeaseConflict, "lease held by another task")
	}
	if lease.Expired(now) {
		return Result{}, domain.NewError(domain.CodeLeaseExpired, "lease expired")
	}
	expires, err := domain.Add(int64(lease.ExpiresAt), cmd.LeaseTicks)
	if err != nil {
		return Result{}, err
	}
	lease.ExpiresAt = domain.LogicalTime(expires)
	lease.Version++
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// applyReleaseLease releases a lease held by this task.
func (s *Service) applyReleaseLease(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	kstr := persistence.LeaseKey(cmd.ResourceType, cmd.ResourceID)
	lease, ok := state.Leases[kstr]
	if !ok {
		return Result{}, domain.NewError(domain.CodeLeaseConflict, "no such lease")
	}
	if lease.Holder != cmd.TaskID {
		return Result{}, domain.NewError(domain.CodeLeaseConflict, "lease held by another task")
	}
	delete(state.Leases, kstr)
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}

// applyReserveSlurry reserves a volume of the mix batch's unallocated pool for
// pouring. Two tasks racing on the same batch serialise through the ledger
// version; only one reservation can succeed for the remaining volume.
func (s *Service) applyReserveSlurry(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	task, err := s.requireTask(state, cmd.TaskID)
	if err != nil {
		return Result{}, err
	}
	if err := s.guardActive(task); err != nil {
		return Result{}, err
	}
	batch, ok := state.Batches[cmd.BatchID]
	if !ok {
		return Result{}, domain.NewError(domain.CodeMaterialImbalance, "batch not found")
	}
	ledger, ok := state.Ledgers[cmd.BatchID]
	if !ok {
		return Result{}, domain.NewError(domain.CodeMaterialImbalance, "ledger not found")
	}
	if now >= batch.WorkDeadline {
		return Result{}, domain.NewError(domain.CodeSlurryExpired, "mix batch working time expired")
	}
	if err := domain.NonNegative(cmd.VolumeML); err != nil {
		return Result{}, err
	}
	if cmd.VolumeML > ledger.Unallocated {
		return Result{}, domain.NewError(domain.CodeVolumeOverclaim, "reservation exceeds unallocated volume")
	}
	newUnallocated, err := domain.Sub(ledger.Unallocated, cmd.VolumeML)
	if err != nil {
		return Result{}, err
	}
	newReserved, err := domain.Add(ledger.Reserved, cmd.VolumeML)
	if err != nil {
		return Result{}, err
	}
	ledger.Unallocated = newUnallocated
	ledger.Reserved = newReserved
	ledger.Version++
	task.UpdatedAt = now
	task.AggregateVersion++
	return s.taskResult(task, now), nil
}
