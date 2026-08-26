// Package httpapi exposes the JSON HTTP API, unified validation, stable error
// mapping, health/readiness reporting and graceful shutdown wiring.
package httpapi

import (
	"encoding/json"
	"net/http"

	"precast-wall-grout-support-release/application"
)

// Server bundles the HTTP routes for the backend over the application service.
type Server struct {
	svc *application.Service
	mux *http.ServeMux
}

// NewServer constructs the HTTP handler tree.
func NewServer(svc *application.Service) *Server {
	s := &Server{svc: svc, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/readyz", s.handleReady)

	s.mux.HandleFunc("POST /api/v1/tasks", s.handleCreateTask)
	s.mux.HandleFunc("POST /api/v1/tasks/{id}/lock", s.handleLock)
	s.mux.HandleFunc("POST /api/v1/tasks/{id}/commands", s.handleCommand)
	s.mux.HandleFunc("POST /api/v1/device-calls/{id}/attempts", s.handleDeviceAttempt)

	s.mux.HandleFunc("GET /api/v1/tasks/{id}", s.handleGetTask)
	s.mux.HandleFunc("GET /api/v1/tasks/{id}/evidence", s.handleEvidence)
	s.mux.HandleFunc("GET /api/v1/tasks/{id}/generations", s.handleGenerations)
	s.mux.HandleFunc("GET /api/v1/tasks/{id}/ledger", s.handleLedger)
	s.mux.HandleFunc("GET /api/v1/tasks/{id}/decision", s.handleDecision)
	s.mux.HandleFunc("GET /api/v1/resources/{type}/{id}/lease", s.handleLease)
}

// Handler returns the root handler for http.Server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	h := s.svc.Health()
	status := http.StatusOK
	if !h.Ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, h)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
