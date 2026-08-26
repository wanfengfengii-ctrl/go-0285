package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"precast-wall-grout-support-release/application"
	"precast-wall-grout-support-release/devices"
	"precast-wall-grout-support-release/domain"
	"precast-wall-grout-support-release/persistence"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	svc := application.NewService(persistence.NewMemoryStore("l", "s"), application.NewLogicalClock(), application.DefaultCatalog(), devices.NewScriptedAdapter())
	return NewServer(svc)
}

func TestHealthz(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d want 200", rr.Code)
	}
}

func TestReadyzWhenReady(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d want 200", rr.Code)
	}
}

func TestCreateAndGetTask(t *testing.T) {
	s := newTestServer(t)
	body := []byte(`{"taskId":"T1","building":"B1","level":"L1","wallPanel":"W1"}`)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: got %d body %s", rr.Code, rr.Body.String())
	}

	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/T1", nil))
	if rr2.Code != http.StatusOK {
		t.Fatalf("get: got %d", rr2.Code)
	}
	var task domain.InspectionTask
	if err := json.Unmarshal(rr2.Body.Bytes(), &task); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if task.ID != "T1" || task.Stage != domain.StageCreated {
		t.Fatalf("unexpected task %+v", task)
	}
}

func TestCommandRequiresIdempotencyKey(t *testing.T) {
	s := newTestServer(t)
	body := []byte(`{"type":"material_check","materialBatch":"MAT-001"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/T1/commands", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 without idempotency key, got %d", rr.Code)
	}
}

func TestLockEndpoint(t *testing.T) {
	s := newTestServer(t)
	// Create task first.
	s.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader([]byte(`{"taskId":"T1","building":"B1","level":"L1","wallPanel":"W1"}`))))

	lock := application.Command{
		Type:           application.CommandLock,
		Operator:       "op",
		Building:       "B1",
		Level:          "L1",
		WallPanel:      "W1",
		CatalogVersion: 1,
		RuleDigest:     domain.CatalogDigest(application.DefaultCatalog()),
		Connections: []application.ConnectionSpec{
			{RebarSpec: "HRB400-12", SleeveSpec: "GT-12", SocketID: "S1"},
		},
		PortNodes: []application.PortNodeSpec{
			{ID: "P1", Kind: domain.PortInlet, SocketID: "S1"},
			{ID: "P2", Kind: domain.PortOutlet, SocketID: "S1"},
		},
		PortEdges:           []application.PortEdgeSpec{{From: "P1", To: "P2"}},
		SlurryPaths:         [][]domain.PortID{{"P1"}},
		MaterialBatch:       "MAT-001",
		WaterBatch:          "WAT-001",
		TheoreticalVolumeML: 5000,
		LossCeilingPPM:      40000,
		ReleaseThreshold:    30,
	}
	body, _ := json.Marshal(lock)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/T1/lock", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", "lock-1")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("lock: got %d body %s", rr.Code, rr.Body.String())
	}
}
