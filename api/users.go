package main

import (
	"encoding/json"
	"errors"
	"net/http"
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
