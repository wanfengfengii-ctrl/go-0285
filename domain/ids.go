package domain

// Core scalar identifiers and value types shared across every package.

// TaskID identifies a single inspection task.
type TaskID string

// Generation identifies an immutable evidence generation. Generation zero is
// the initial locked generation; every re-grout creates exactly one successor.
type Generation int64

// LogicalTime is a monotonically non-decreasing tick from the logical clock.
type LogicalTime int64

// Operator identifies the actor that submitted a command or signed a review.
type Operator string

// OperationID is the idempotency key of a command; equal operation IDs with
// equal normalized content return the original result.
type OperationID string

// ResourceType discriminates the kinds of leaseable resources.
type ResourceType string

// Resource leaseable kinds referenced by the documented flows.
const (
	ResourcePump    ResourceType = "pump"
	ResourceMixer   ResourceType = "mixer"
	ResourceChannel ResourceType = "channel"
)

// ResourceKey uniquely identifies a leaseable resource by type and number.
type ResourceKey struct {
	Type ResourceType
	ID   string
}

// Stage is the lifecycle phase of an inspection task.
type Stage string

const (
	StageCreated  Stage = "created"
	StageLocked   Stage = "locked"
	StagePrepared Stage = "prepared"
	StagePoured   Stage = "poured"
	StageCured    Stage = "cured"
	StageReviewed Stage = "reviewed"
	StageTerminal Stage = "terminal"
	StageCanceled Stage = "canceled"
)
