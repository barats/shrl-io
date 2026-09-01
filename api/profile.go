package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/store"
)

const minPasswordLen = 8

// changePassword lets a user replace their own password. It verifies the
// current password, then revokes every other login token and every API key —
// a password change is treated as a security event (ADR 0012). The current
// session survives; everything else must be re-issued.
func (s *server) changePassword(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !domain.VerifyPassword(u.PasswordHash, req.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}
	if len(req.NewPassword) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := domain.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "password hashing failed")
		return
	}
	if err := s.users.SetPassword(r.Context(), u.ID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}
	curHash := domain.HashToken(bearerToken(r))
	if err := s.users.DeleteTokensForUserExcept(r.Context(), u.ID, curHash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke tokens")
		return
	}
	if err := s.users.DeleteKeysForUser(r.Context(), u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke API keys")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// createKey issues a new API key for the current user and returns the
// plaintext secret exactly once.
func (s *server) createKey(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 64 {
		writeError(w, http.StatusBadRequest, "key name must be 1-64 characters")
		return
	}
	secret, err := domain.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "key generation failed")
		return
	}
	k := &domain.APIKey{UserID: u.ID, Name: req.Name, Hash: domain.HashToken(secret)}
	if err := s.users.CreateKey(r.Context(), k); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create key")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"api_key": k, "secret": secret})
}

// listKeys returns the current user's API keys, without hashes.
func (s *server) listKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.users.ListKeys(r.Context(), currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list keys")
		return
	}
	if keys == nil {
		keys = []domain.APIKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

// deleteKey revokes one of the current user's API keys.
func (s *server) deleteKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid key id")
		return
	}
	if err := s.users.DeleteKey(r.Context(), id, currentUser(r).ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "key not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to revoke key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
