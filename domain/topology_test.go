package domain

import "testing"

func testCatalog() CatalogSnapshot {
	return CatalogSnapshot{
		Version: 1,
		Compat: []SocketCompat{
			{RebarSpec: "HRB400-12", SleeveSpec: "GT-12"},
			{RebarSpec: "HRB400-16", SleeveSpec: "GT-16"},
		},
		MaterialCert: []MaterialCert{
			{BatchID: "MAT-001", Version: 1},
			{BatchID: "WAT-001", Version: 1},
		},
		WaterRules: WaterRule{MinRatioPPM: 300000, MaxRatioPPM: 500000},
		LossBounds: LossBounds{MaxLossRatioPPM: 50000, MinVolumeML: 1000, MaxVolumeML: 100000},
		WorkLimits: WorkLimits{MinWorkTicks: 10, MaxWorkTicks: 1000},
		Personnel:  []PersonnelQualification{{PersonID: "r1", Qualified: true, ValidFrom: 0, ValidUntil: 1 << 40}},
	}
}

func validPlan() LockedPlan {
	return LockedPlan{
		WallPosition: WallPosition{Building: "B1", Level: "L1", WallID: "W1"},
		Connections: []Connection{
			{RebarSpec: "HRB400-12", SleeveSpec: "GT-12", SocketID: "S1"},
			{RebarSpec: "HRB400-12", SleeveSpec: "GT-12", SocketID: "S2"},
		},
		PortNodes: []PortNode{
			{ID: "P1", Kind: PortInlet, SocketID: "S1"},
			{ID: "P2", Kind: PortOutlet, SocketID: "S1"},
			{ID: "P3", Kind: PortInlet, SocketID: "S2"},
			{ID: "P4", Kind: PortOutlet, SocketID: "S2"},
		},
		PortEdges: []PortEdge{
			{From: "P1", To: "P2"},
			{From: "P2", To: "P3"},
			{From: "P3", To: "P4"},
		},
		SlurryPaths:         [][]PortID{{"P1", "P3"}},
		MaterialBatch:       "MAT-001",
		WaterBatch:          "WAT-001",
		TheoreticalVolumeML: 5000,
		LossCeilingPPM:      40000,
		SpecimenPlan:        []Specimen{{ID: "SP1", CureTicks: 100}},
		UltrasonicLines:     []string{"L1"},
		EndoscopeHoles:      []string{"H1"},
		ReleaseThreshold:    30,
	}
}

func TestValidateLockPlanSuccess(t *testing.T) {
	issues := ValidateLockPlan("B1", "L1", "W1", validPlan(), testCatalog())
	if len(issues) != 0 {
		t.Fatalf("expected valid plan, got issues: %+v", issues)
	}
}

func TestValidateLockPlanWallMismatch(t *testing.T) {
	issues := ValidateLockPlan("B2", "L1", "W1", validPlan(), testCatalog())
	if len(issues) == 0 || issues[0].Code != CodeWallPanelMismatch {
		t.Fatalf("expected WALL_PANEL_MISMATCH, got %+v", issues)
	}
}

func TestValidateLockPlanSocketIncompatible(t *testing.T) {
	plan := validPlan()
	plan.Connections[0].SleeveSpec = "GT-16"
	issues := ValidateLockPlan("B1", "L1", "W1", plan, testCatalog())
	if len(issues) == 0 {
		t.Fatal("expected socket incompatible issue")
	}
	if issues[0].Code != CodeSocketIncompatible {
		t.Fatalf("expected SOCKET_INCOMPATIBLE, got %+v", issues)
	}
}

func TestValidateLockPlanDuplicatePort(t *testing.T) {
	plan := validPlan()
	plan.PortNodes[1].ID = "P1" // duplicate inlet id
	issues := ValidateLockPlan("B1", "L1", "W1", plan, testCatalog())
	if len(issues) == 0 {
		t.Fatal("expected duplicate port issue")
	}
	found := false
	for _, is := range issues {
		if is.Code == CodeInvalidTopology {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected INVALID_TOPOLOGY, got %+v", issues)
	}
}

func TestValidateLockPlanStableOrdering(t *testing.T) {
	plan := validPlan()
	plan.WallPosition.Building = "B2"        // wall mismatch
	plan.Connections[0].SleeveSpec = "GT-16" // socket incompatible
	plan.MaterialBatch = "UNKNOWN"           // uncertified batch
	issues := ValidateLockPlan("B1", "L1", "W1", plan, testCatalog())
	if len(issues) < 3 {
		t.Fatalf("expected at least 3 issues, got %d", len(issues))
	}
	for i := 1; i < len(issues); i++ {
		if issues[i].Key < issues[i-1].Key {
			t.Fatalf("issues not sorted by key: %q before %q", issues[i-1].Key, issues[i].Key)
		}
	}
}

func TestComputeRecheckSet(t *testing.T) {
	got := ComputeRecheckSet(validPlan(), []SocketID{"S1"})
	// S1 plus its adjacent S2 (edge P2->P3) plus successor S2 on the slurry path.
	if len(got) != 2 {
		t.Fatalf("expected recheck set {S1,S2}, got %v", got)
	}
	if got[0] != "S1" || got[1] != "S2" {
		t.Fatalf("expected sorted [S1 S2], got %v", got)
	}
}

func TestPourSequence(t *testing.T) {
	seq := PourSequence(validPlan())
	if len(seq) != 7 {
		t.Fatalf("expected 7 pour steps, got %d: %+v", len(seq), seq)
	}
	wantTypes := []EventType{
		EventInletStart, EventOutletStable, EventOutletSeal, EventPortSwitch,
		EventInletStart, EventOutletStable, EventOutletSeal,
	}
	for i, wt := range wantTypes {
		if seq[i].Type != wt {
			t.Fatalf("step %d: expected %s got %s", i, wt, seq[i].Type)
		}
	}
}
