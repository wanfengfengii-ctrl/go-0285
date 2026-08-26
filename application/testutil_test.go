package application

import (
	"testing"

	"precast-wall-grout-support-release/devices"
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

// newTestEnv builds a service over an in-memory repository with a manual clock,
// the default catalog and a scripted device adapter.
func newTestEnv(t *testing.T) (*Service, *ManualClock, *devices.ScriptedAdapter) {
	t.Helper()
	repo := persistence.NewMemoryStore("l", "s")
	clock := NewManualClock(0)
	adapter := devices.NewScriptedAdapter()
	svc := NewService(repo, clock, DefaultCatalog(), adapter)
	return svc, clock, adapter
}

// lockPlanCmd returns a lock command for a standard two-socket wall that is
// compatible with DefaultCatalog.
func lockPlanCmd(id domain.TaskID, opID domain.OperationID) Command {
	return Command{
		Type:           CommandLock,
		TaskID:         id,
		Operator:       "op",
		OperationID:    opID,
		Building:       "B1",
		Level:          "L1",
		WallPanel:      "W1",
		CatalogVersion: 1,
		RuleDigest:     domain.CatalogDigest(DefaultCatalog()),
		Connections: []ConnectionSpec{
			{RebarSpec: "HRB400-12", SleeveSpec: "GT-12", SocketID: "S1"},
			{RebarSpec: "HRB400-12", SleeveSpec: "GT-12", SocketID: "S2"},
		},
		PortNodes: []PortNodeSpec{
			{ID: "P1", Kind: domain.PortInlet, SocketID: "S1"},
			{ID: "P2", Kind: domain.PortOutlet, SocketID: "S1"},
			{ID: "P3", Kind: domain.PortInlet, SocketID: "S2"},
			{ID: "P4", Kind: domain.PortOutlet, SocketID: "S2"},
		},
		PortEdges: []PortEdgeSpec{
			{From: "P1", To: "P2"},
			{From: "P2", To: "P3"},
			{From: "P3", To: "P4"},
		},
		SlurryPaths:         [][]domain.PortID{{"P1", "P3"}},
		MaterialBatch:       "MAT-001",
		WaterBatch:          "WAT-001",
		TheoreticalVolumeML: 5000,
		LossCeilingPPM:      40000,
		Specimens:           []SpecimenSpec{{ID: "SP1", CureTicks: 100}},
		UltrasonicLines:     []string{"L1"},
		EndoscopeHoles:      []string{"H1"},
		ReleaseThreshold:    30,
	}
}

// createAndLock creates a task and locks it with the standard plan.
func createAndLock(t *testing.T, svc *Service, id domain.TaskID) {
	t.Helper()
	if _, err := svc.CreateTask(nil, id, "B1", "L1", "W1"); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if _, err := svc.Handle(nil, lockPlanCmd(id, "lock-"+domain.OperationID(id))); err != nil {
		t.Fatalf("lock task: %v", err)
	}
}

// mixBatch mixes a standard batch for a locked task.
func mixBatch(t *testing.T, svc *Service, id domain.TaskID, batch string) {
	t.Helper()
	_, err := svc.Handle(nil, Command{
		Type:        CommandMix,
		TaskID:      id,
		Operator:    "op",
		OperationID: domain.OperationID("mix-" + batch),
		BatchID:     batch,
		InputGrams:  5000,
		WaterML:     2000,
		LossML:      200,
		WorkTicks:   100,
	})
	if err != nil {
		t.Fatalf("mix: %v", err)
	}
}
