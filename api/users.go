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

func (s *server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	u := currentUser(r)
	if u == nil || !u.IsAdmin {
		writeError(w, http.StatusForbidden, "admin required")
		return false
	}
	return true
}

func (s *server) listUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	users, err := s.users.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []domain.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *server) createUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Username) > 64 {
		writeError(w, http.StatusBadRequest, "username must be 1-64 characters")
		return
	}
	password := req.Password
	if password == "" {
		p, err := domain.GeneratePassword(20)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "password generation failed")
			return
		}
		password = p
	}
	hash, err := domain.HashPassword(password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "password hashing failed")
		return
	}
	u := &domain.User{Username: req.Username, PasswordHash: hash, IsAdmin: req.IsAdmin}
	if err := s.users.Create(r.Context(), u); err != nil {
		if errors.Is(err, store.ErrDuplicatedKey) {
			writeError(w, http.StatusConflict, "username already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"user":     u,
		"password": password, // shown once to the creating admin
	})
}

// resetUserPassword gives a user a generated temporary password shown once,
// flags it for forced change on their next login, and revokes their tokens and
// keys. There is no SMTP in the stack, so reset is admin-mediated (ADR 0012).
func (s *server) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if _, err := s.users.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	password, err := domain.GeneratePassword(20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "password generation failed")
		return
	}
	hash, err := domain.HashPassword(password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "password hashing failed")
		return
	}
	if err := s.users.SetPassword(r.Context(), id, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set password")
		return
	}
	if err := s.users.RequirePasswordChange(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to flag password change")
		return
	}
	if err := s.users.DeleteTokensForUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke tokens")
		return
	}
	if err := s.users.DeleteKeysForUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke keys")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"password": password})
}

// deleteUser removes a user (admin only). Their Personal Links, bearer
// tokens, and memberships are removed; Team Links they created stay with the
// Team (the fixed-team rule), leaving created_by as a dangling id. A user who
// is the sole owner of a team cannot be deleted (the last-owner rule).
func (s *server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if _, err := s.users.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	// last-owner rule: a user who is the only owner of any team cannot go
	teams, err := s.teams.ListForUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}
	for _, t := range teams {
		role, err := s.teams.MemberRole(r.Context(), t.ID, id)
		if err == nil && role == domain.RoleOwner {
			n, err := s.teams.CountOwners(r.Context(), t.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to count owners")
				return
			}
			if n <= 1 {
				writeError(w, http.StatusConflict, "cannot delete: user is the only owner of team "+t.Name)
				return
			}
		}
	}
	// evict the user's personal links from the redirect cache
	links, err := s.links.ListPersonalByCreator(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list links")
		return
	}
	if err := s.users.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	for _, l := range links {
		s.linkCache.Delete(r.Context(), l.Code)
	}
	w.WriteHeader(http.StatusNoContent)
}
