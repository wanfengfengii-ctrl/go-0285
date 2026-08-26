package application

import "precast-wall-grout-support-release/domain"

// CommandType discriminates the unified command endpoint. Each command is
// idempotency-checked by its OperationID and normalized content digest.
type CommandType string

const (
	CommandLock          CommandType = "lock"
	CommandMaterialCheck CommandType = "material_check"
	CommandPosition      CommandType = "position"
	CommandSeal          CommandType = "seal"
	CommandMix           CommandType = "mix"
	CommandSample        CommandType = "sample"
	CommandAcquireLease  CommandType = "acquire_lease"
	CommandRenewLease    CommandType = "renew_lease"
	CommandReleaseLease  CommandType = "release_lease"
	CommandReserveSlurry CommandType = "reserve_slurry"
	CommandInletStart    CommandType = "inlet_start"
	CommandOutletStable  CommandType = "outlet_stable"
	CommandOutletSeal    CommandType = "outlet_seal"
	CommandPortSwitch    CommandType = "port_switch"
	CommandStrength      CommandType = "strength"
	CommandUltrasonic    CommandType = "ultrasonic"
	CommandEndoscope     CommandType = "endoscope"
	CommandLeak          CommandType = "leak"
	CommandRegrout       CommandType = "regrout"
	CommandReview        CommandType = "review"
	CommandTerminal      CommandType = "terminal"
)

// Command is a single user-submitted command. Only the fields relevant to a
// given Type are read; the rest are ignored, mirroring a documented command
// envelope carried over the JSON API.
type Command struct {
	Type        CommandType        `json:"type"`
	TaskID      domain.TaskID      `json:"taskId,omitempty"`
	Operator    domain.Operator    `json:"operator"`
	OperationID domain.OperationID `json:"operationId"`
	Generation  domain.Generation  `json:"generation,omitempty"`

	// Lock payload.
	Building            string            `json:"building,omitempty"`
	Level               string            `json:"level,omitempty"`
	WallPanel           string            `json:"wallPanel,omitempty"`
	CatalogVersion      int64             `json:"catalogVersion,omitempty"`
	RuleDigest          string            `json:"ruleDigest,omitempty"`
	Connections         []ConnectionSpec  `json:"connections,omitempty"`
	PortNodes           []PortNodeSpec    `json:"portNodes,omitempty"`
	PortEdges           []PortEdgeSpec    `json:"portEdges,omitempty"`
	SlurryPaths         [][]domain.PortID `json:"slurryPaths,omitempty"`
	MaterialBatch       string            `json:"materialBatch,omitempty"`
	WaterBatch          string            `json:"waterBatch,omitempty"`
	TheoreticalVolumeML int64             `json:"theoreticalVolumeMl,omitempty"`
	LossCeilingPPM      int64             `json:"lossCeilingPpm,omitempty"`
	Specimens           []SpecimenSpec    `json:"specimens,omitempty"`
	UltrasonicLines     []string          `json:"ultrasonicLines,omitempty"`
	EndoscopeHoles      []string          `json:"endoscopeHoles,omitempty"`
	ReleaseThreshold    int64             `json:"releaseThreshold,omitempty"`

	// Mix / sample payload.
	BatchID    string `json:"batchId,omitempty"`
	InputGrams int64  `json:"inputGrams,omitempty"`
	WaterML    int64  `json:"waterMl,omitempty"`
	LossML     int64  `json:"lossMl,omitempty"`
	SampleML   int64  `json:"sampleMl,omitempty"`
	WorkTicks  int64  `json:"workTicks,omitempty"`

	// Lease / reserve payload.
	ResourceType domain.ResourceType `json:"resourceType,omitempty"`
	ResourceID   string              `json:"resourceId,omitempty"`
	LeaseTicks   int64               `json:"leaseTicks,omitempty"`
	VolumeML     int64               `json:"volumeMl,omitempty"`

	// Pour payload.
	PortID   domain.PortID   `json:"portId,omitempty"`
	SocketID domain.SocketID `json:"socketId,omitempty"`
	Pressure int64           `json:"pressure,omitempty"`

	// Detection payload.
	SpecimenID string `json:"specimenId,omitempty"`
	Value      int64  `json:"value,omitempty"`

	// Re-grout payload.
	Reason  string            `json:"reason,omitempty"`
	Defects []domain.SocketID `json:"defects,omitempty"`

	// Review / terminal payload.
	EvidenceDigest string              `json:"evidenceDigest,omitempty"`
	TerminalType   domain.TerminalType `json:"terminalType,omitempty"`
}

// ConnectionSpec, PortNodeSpec, PortEdgeSpec and SpecimenSpec are the raw
// lock-plan inputs mapped onto domain.LockedPlan at lock time.
type ConnectionSpec struct {
	RebarSpec  string          `json:"rebarSpec"`
	SleeveSpec string          `json:"sleeveSpec"`
	SocketID   domain.SocketID `json:"socketId"`
}

type PortNodeSpec struct {
	ID       domain.PortID   `json:"id"`
	Kind     domain.PortKind `json:"kind"`
	SocketID domain.SocketID `json:"socketId"`
}

type PortEdgeSpec struct {
	From domain.PortID `json:"from"`
	To   domain.PortID `json:"to"`
}

type SpecimenSpec struct {
	ID        string `json:"id"`
	CureTicks int64  `json:"cureTicks"`
}
