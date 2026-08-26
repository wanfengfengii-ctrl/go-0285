package application

import (
	"context"
	"encoding/json"

	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

const maxRetries = 3

// Service is the command service that owns the full grouting lifecycle. It is
// built over a persistence.Repository and applies commands optimistically,
// retrying a bounded number of times on concurrent modification.
type Service struct {
	repo       persistence.Repository
	clock      Clock
	catalog    domain.CatalogSnapshot
	device     DeviceAdapter
	maxRetries int
}

// NewService constructs a Service with the given repository, clock, catalog and
// device adapter.
func NewService(repo persistence.Repository, clock Clock, catalog domain.CatalogSnapshot, device DeviceAdapter) *Service {
	return &Service{repo: repo, clock: clock, catalog: catalog, device: device, maxRetries: maxRetries}
}

// CreateTask creates a task in StageCreated and returns its id. It is
// idempotent by task id: an existing task is returned unchanged.
func (s *Service) CreateTask(ctx context.Context, id domain.TaskID, building, level, wallPanel string) (domain.InspectionTask, error) {
	_ = ctx
	state, err := s.repo.Load()
	if err != nil {
		return domain.InspectionTask{}, err
	}
	if t, ok := state.Tasks[id]; ok {
		return *t, nil
	}
	now := s.clock.Now()
	task := &domain.InspectionTask{
		ID:                id,
		Building:          building,
		Level:             level,
		WallPanel:         wallPanel,
		Stage:             domain.StageCreated,
		CurrentGeneration: 0,
		AggregateVersion:  0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	next := state.Clone()
	next.Tasks[id] = task
	next.Version = state.Version + 1
	if err := s.repo.Save(next, state.Version); err != nil {
		return domain.InspectionTask{}, err
	}
	return *task, nil
}

// Handle applies a single command. It computes the idempotency digest, checks
// the idempotency record, then applies the command to a cloned state and
// commits through a conditional write, retrying on concurrent modification.
func (s *Service) Handle(ctx context.Context, cmd Command) (Result, error) {
	_ = ctx
	digest, err := ContentDigest(cmd)
	if err != nil {
		return Result{}, err
	}
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		state, err := s.repo.Load()
		if err != nil {
			return Result{}, err
		}
		if rec, ok := state.Idempotency[cmd.TaskID][cmd.OperationID]; ok {
			if rec.ContentDigest != digest {
				return Result{}, domain.NewError(domain.CodeIdempotencyConflict, "operation id reused with different content")
			}
			var res Result
			if err := json.Unmarshal(rec.Result, &res); err != nil {
				return Result{}, err
			}
			return res, nil
		}
		next := state.Clone()
		now := s.clock.Now()
		res, err := s.apply(next, cmd, now)
		if err != nil {
			if domain.IsCode(err, domain.CodeDeviceRetryPending) {
				// A failed device attempt is still persisted with its retry
				// schedule; only the reading itself is withheld.
				next.Version = state.Version + 1
				if saveErr := s.repo.Save(next, state.Version); saveErr != nil {
					if domain.IsCode(saveErr, domain.CodeConcurrentModification) {
						continue
					}
					return Result{}, saveErr
				}
				return Result{}, err
			}
			return Result{}, err
		}
		next.Version = state.Version + 1
		s.recordIdempotency(next, cmd, digest, res)
		if err := s.repo.Save(next, state.Version); err != nil {
			if domain.IsCode(err, domain.CodeConcurrentModification) {
				continue
			}
			return Result{}, err
		}
		return res, nil
	}
	return Result{}, domain.NewError(domain.CodeConcurrentModification, "retries exhausted")
}

func (s *Service) recordIdempotency(state *persistence.State, cmd Command, digest string, res Result) {
	if state.Idempotency[cmd.TaskID] == nil {
		state.Idempotency[cmd.TaskID] = map[domain.OperationID]persistence.IdempotencyRecord{}
	}
	data, _ := json.Marshal(res)
	state.Idempotency[cmd.TaskID][cmd.OperationID] = persistence.IdempotencyRecord{
		OperationID:   cmd.OperationID,
		ContentDigest: digest,
		Result:        data,
	}
}

// apply dispatches a command to its handler. Each handler mutates state (a
// clone) and returns a Result; it never persists.
func (s *Service) apply(state *persistence.State, cmd Command, now domain.LogicalTime) (Result, error) {
	switch cmd.Type {
	case CommandLock:
		return s.applyLock(state, cmd, now)
	case CommandMaterialCheck:
		return s.applyMaterialCheck(state, cmd, now)
	case CommandPosition:
		return s.applyPosition(state, cmd, now)
	case CommandSeal:
		return s.applySeal(state, cmd, now)
	case CommandMix:
		return s.applyMix(state, cmd, now)
	case CommandSample:
		return s.applySample(state, cmd, now)
	case CommandAcquireLease:
		return s.applyAcquireLease(state, cmd, now)
	case CommandRenewLease:
		return s.applyRenewLease(state, cmd, now)
	case CommandReleaseLease:
		return s.applyReleaseLease(state, cmd, now)
	case CommandReserveSlurry:
		return s.applyReserveSlurry(state, cmd, now)
	case CommandInletStart:
		return s.applyInletStart(state, cmd, now)
	case CommandOutletStable:
		return s.applyOutletStable(state, cmd, now)
	case CommandOutletSeal:
		return s.applyOutletSeal(state, cmd, now)
	case CommandPortSwitch:
		return s.applyPortSwitch(state, cmd, now)
	case CommandStrength:
		return s.applyStrength(state, cmd, now)
	case CommandUltrasonic:
		return s.applyUltrasonic(state, cmd, now)
	case CommandEndoscope:
		return s.applyEndoscope(state, cmd, now)
	case CommandLeak:
		return s.applyLeak(state, cmd, now)
	case CommandRegrout:
		return s.applyRegrout(state, cmd, now)
	case CommandReview:
		return s.applyReview(state, cmd, now)
	case CommandTerminal:
		return s.applyTerminal(state, cmd, now)
	default:
		return Result{}, domain.NewError(domain.CodeEvidenceIncomplete, "unknown command type")
	}
}

func (s *Service) taskResult(task *domain.InspectionTask, now domain.LogicalTime) Result {
	res := Result{
		TaskID:           task.ID,
		Stage:            task.Stage,
		Generation:       task.CurrentGeneration,
		AggregateVersion: task.AggregateVersion,
		LogicalTime:      now,
	}
	if task.Terminal != nil && task.Terminal.Type == domain.TerminalRelease {
		res.ReleaseCredential = task.Terminal.ReleaseCredential
	}
	return res
}

func (s *Service) requireTask(state *persistence.State, id domain.TaskID) (*domain.InspectionTask, error) {
	t, ok := state.Tasks[id]
	if !ok {
		return nil, domain.NewError(domain.CodeEvidenceIncomplete, "task not found")
	}
	return t, nil
}

// appendEvidence appends an immutable evidence event to the state.
func (s *Service) appendEvidence(state *persistence.State, ev domain.EvidenceEvent) {
	state.NextEventID++
	ev.ID = formatEventID(state.NextEventID)
	state.Evidence = append(state.Evidence, ev)
}

func formatEventID(seq uint64) string {
	return "evt-" + itoa(seq)
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
