package domain

// EventType discriminates the kinds of append-only evidence events.
type EventType string

const (
	EventMaterialCheck EventType = "material_check"
	EventPosition      EventType = "position"
	EventSeal          EventType = "seal"
	EventMix           EventType = "mix"
	EventSample        EventType = "sample"
	EventInletStart    EventType = "inlet_start"
	EventOutletStable  EventType = "outlet_stable"
	EventOutletSeal    EventType = "outlet_seal"
	EventPortSwitch    EventType = "port_switch"
	EventStrength      EventType = "strength"
	EventUltrasonic    EventType = "ultrasonic"
	EventEndoscope     EventType = "endoscope"
	EventLeak          EventType = "leak"
	EventRegrout       EventType = "regrout"
	EventReview        EventType = "review"
)

// EvidenceEvent is an immutable, append-only evidence record. Once written it
// can never be updated or deleted; late receipts for older generations are
// archived but never override the current conclusion.
type EvidenceEvent struct {
	ID            string
	TaskID        TaskID
	WallPanel     string
	SocketID      SocketID
	PortID        PortID
	SpecimenID    string
	Generation    Generation
	Type          EventType
	FixedValue    int64
	At            LogicalTime
	Operator      Operator
	OperationID   OperationID
	ContentDigest string
	Valid         bool
}

// GenerationState tracks the continuous prefix and conclusions of one
// generation. A re-grout generation is isolated from its parent and starts
// with an independent prefix.
type GenerationState struct {
	TaskID           TaskID
	Generation       Generation
	ParentGeneration Generation
	Reason           string
	Prefix           []PortID
	Step             int
	Defects          []SocketID
	RecheckSet       []SocketID
	StrengthOK       bool
	UltrasonicOK     bool
	EndoscopeOK      bool
	LeakOK           bool
	Isolated         bool
}

// DeviceCall records a scripted device attempt and its deterministic retry plan.
type DeviceCall struct {
	CallID        string
	Device        string
	TaskID        TaskID
	Target        string
	Generation    Generation
	RequestDigest string
	Attempts      []DeviceAttempt
	Closed        bool
}

// DeviceAttempt is one attempt outcome for a device call.
type DeviceAttempt struct {
	Sequence    int
	Outcome     DeviceOutcome
	NextRetryAt LogicalTime
}

// DeviceOutcome is the result kind of a device attempt.
type DeviceOutcome string

const (
	OutcomeRejected   DeviceOutcome = "rejected"
	OutcomeDisconnect DeviceOutcome = "disconnect"
	OutcomeTimeout    DeviceOutcome = "timeout"
	OutcomeMalformed  DeviceOutcome = "malformed"
	OutcomeSuccess    DeviceOutcome = "success"
)

// Review is a single reviewer signature bound to an evidence digest.
type Review struct {
	Reviewer       Operator
	EvidenceDigest string
	At             LogicalTime
}

// TerminalType is the single irreversible terminal decision kind.
type TerminalType string

const (
	TerminalRelease    TerminalType = "release"
	TerminalQuarantine TerminalType = "quarantine"
	TerminalCancel     TerminalType = "cancel"
)

// TerminalDecision is the unique, irreversible terminal slot outcome. Only a
// winning release produces a ReleaseCredential.
type TerminalDecision struct {
	Type              TerminalType
	At                LogicalTime
	ReleaseCredential string
}
