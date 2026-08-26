package httpapi

import (
	"encoding/json"
	"net/http"

	"precast-wall-grout-support-release/application"
	"precast-wall-grout-support-release/domain"
)

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, domain.NewError(domain.CodeEvidenceIncomplete, "invalid request body"))
		return
	}
	task, err := s.svc.CreateTask(r.Context(), req.TaskID, req.Building, req.Level, req.WallPanel)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleLock(w http.ResponseWriter, r *http.Request) {
	cmd, err := decodeCommand(r)
	if err != nil {
		writeError(w, err)
		return
	}
	cmd.Type = application.CommandLock
	cmd.TaskID = domain.TaskID(r.PathValue("id"))
	res, err := s.svc.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	cmd, err := decodeCommand(r)
	if err != nil {
		writeError(w, err)
		return
	}
	cmd.TaskID = domain.TaskID(r.PathValue("id"))
	res, err := s.svc.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDeviceAttempt(w http.ResponseWriter, r *http.Request) {
	var req deviceAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, domain.NewError(domain.CodeEvidenceIncomplete, "invalid request body"))
		return
	}
	res, err := s.svc.RecordDeviceAttempt(r.PathValue("id"), req.Outcome, req.Value)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	task, err := s.svc.GetTask(domain.TaskID(r.PathValue("id")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleEvidence(w http.ResponseWriter, r *http.Request) {
	ev, err := s.svc.ListEvidence(domain.TaskID(r.PathValue("id")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ev)
}

func (s *Server) handleGenerations(w http.ResponseWriter, r *http.Request) {
	gens, err := s.svc.ListGenerations(domain.TaskID(r.PathValue("id")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, gens)
}

func (s *Server) handleLedger(w http.ResponseWriter, r *http.Request) {
	ledger, err := s.svc.GetLedger(r.URL.Query().Get("batch"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ledger)
}

func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	decision, err := s.svc.GetDecision(domain.TaskID(r.PathValue("id")))
	if err != nil {
		writeError(w, err)
		return
	}
	if decision == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"status": "no decision"})
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (s *Server) handleLease(w http.ResponseWriter, r *http.Request) {
	lease, err := s.svc.GetLease(domain.ResourceType(r.PathValue("type")), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if lease == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"status": "no lease"})
		return
	}
	writeJSON(w, http.StatusOK, lease)
}

// decodeCommand reads a Command body, rejecting a missing Idempotency-Key via a
// deterministic error.
func decodeCommand(r *http.Request) (application.Command, error) {
	var cmd application.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		return cmd, domain.NewError(domain.CodeEvidenceIncomplete, "invalid request body")
	}
	if r.Header.Get("Idempotency-Key") == "" {
		return cmd, domain.NewError(domain.CodeIdempotencyConflict, "Idempotency-Key header required")
	}
	cmd.OperationID = domain.OperationID(r.Header.Get("Idempotency-Key"))
	return cmd, nil
}
