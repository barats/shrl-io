package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/store"
)

// listHostnames returns the Hostname Registry — every registered hostname,
// available to any authenticated user for the create-link select.
func (s *server) listHostnames(w http.ResponseWriter, r *http.Request) {
	names, err := s.linkSvc.ListHostnames(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list hostnames")
		return
	}
	writeJSON(w, http.StatusOK, names)
}

func (s *server) createHostname(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req struct {
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name, err := domain.NormalizeAndValidateHostname(req.Hostname)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h := &domain.Hostname{Name: name, RegisteredBy: currentUser(r).ID}
	if err := s.hostnames.Create(r.Context(), h); err != nil {
		if errors.Is(err, store.ErrDuplicatedKey) {
			writeError(w, http.StatusConflict, "hostname already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to register hostname")
		return
	}
	writeJSON(w, http.StatusCreated, h)
}

func (s *server) deleteHostname(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	name, err := domain.NormalizeAndValidateHostname(r.PathValue("hostname"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Removing a hostname only unregisters it: existing Links keep serving,
	// they just can't be recreated or edited under this name.
	if err := s.hostnames.Delete(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove hostname")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
