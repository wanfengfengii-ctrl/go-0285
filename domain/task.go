package domain

// InspectionTask is the aggregate root owning a task's stage, immutable locked
// plan, current generation, aggregate version and terminal slot. Commands are
// idempotency-checked, stage-guarded and appended through storage-side
// conditional writes so a single task serializes its commits.
type InspectionTask struct {
	ID                TaskID
	Building          string
	Level             string
	WallPanel         string
	LockDigest        string
	Stage             Stage
	CurrentGeneration Generation
	AggregateVersion  int64
	Terminal          *TerminalDecision
	CreatedAt         LogicalTime
	UpdatedAt         LogicalTime
}

// LockedPlan is the immutable plan fixed at lock time. Its fields may never be
// modified in place; changes require cancelling and creating a new task.
type LockedPlan struct {
	WallPosition        WallPosition
	Connections         []Connection
	PortNodes           []PortNode
	PortEdges           []PortEdge
	SlurryPaths         [][]PortID
	MaterialBatch       string
	WaterBatch          string
	TheoreticalVolumeML int64
	LossCeilingPPM      int64
	SpecimenPlan        []Specimen
	UltrasonicLines     []string
	EndoscopeHoles      []string
	ReleaseThreshold    int64
}

// WallPosition summarizes where a wall panel sits.
type WallPosition struct {
	Building string
	Level    string
	WallID   string
}

// Connection pairs one reinforcement bar with one compatible sleeve.
type Connection struct {
	RebarSpec  string
	SleeveSpec string
	SocketID   SocketID
}

// SocketID identifies a single sleeve on a wall.
type SocketID string

// PortID identifies a grouting (inlet) or outlet port.
type PortID string

// PortKind discriminates inlet vs outlet ports.
type PortKind string

const (
	PortInlet  PortKind = "inlet"
	PortOutlet PortKind = "outlet"
)

// PortNode describes a port node in the directed grouting topology.
type PortNode struct {
	ID       PortID
	Kind     PortKind
	SocketID SocketID
}

// PortEdge is a directed edge from one port to the next in the grouting order.
type PortEdge struct {
	From PortID
	To   PortID
}

// Specimen is a planned test specimen with its curing plan.
type Specimen struct {
	ID        string
	CureTicks int64
}
