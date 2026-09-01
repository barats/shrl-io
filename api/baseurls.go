package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/store"
)

// listBaseURLs returns the Base URL Registry — every registered base URL,
// available to any authenticated user for the create-link select.
func (s *server) listBaseURLs(w http.ResponseWriter, r *http.Request) {
	names, err := s.linkSvc.ListBaseURLs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list base URLs")
		return
	}
	writeJSON(w, http.StatusOK, names)
}

func (s *server) createBaseURL(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req struct {
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	baseURL, err := domain.NormalizeAndValidateBaseURL(req.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	b := &domain.BaseURL{BaseURL: baseURL, RegisteredBy: currentUser(r).ID}
	if err := s.baseURLs.Create(r.Context(), b); err != nil {
		if errors.Is(err, store.ErrDuplicatedKey) {
			writeError(w, http.StatusConflict, "base URL already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to register base URL")
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *server) deleteBaseURL(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	// A base URL carries scheme, optional port, and path, so it arrives as a
	// query parameter rather than a path segment (which would need heavy
	// escaping for "/" and ":").
	baseURL, err := domain.NormalizeAndValidateBaseURL(r.URL.Query().Get("url"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Removing a base URL only unregisters it: existing Links keep serving,
	// they just can't be recreated or edited under this value.
	if err := s.baseURLs.Delete(r.Context(), baseURL); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove base URL")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
