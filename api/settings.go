package main

import (
	"encoding/json"
	"net/http"

	"github.com/barats/shrl-io/internal/domain"
)

// getSettings returns the runtime-configurable instance settings.
func (s *server) getSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	n, err := s.settings.CodeLength(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code_length": n})
}

// updateCodeLength sets the per-instance Code Length (ADR 0013).
func (s *server) updateCodeLength(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req struct {
		CodeLength int `json:"code_length"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := domain.ValidateCodeLength(req.CodeLength); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.settings.SetCodeLength(r.Context(), req.CodeLength); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save setting")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code_length": req.CodeLength})
}
