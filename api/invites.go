package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/store"
)

// createInvite generates a single-use invite code for a team (owner only).
func (s *server) createInvite(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	if !s.requireTeamOwner(w, r, t.ID) {
		return
	}
	code, err := domain.GenerateInviteCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invite code generation failed")
		return
	}
	inv := &domain.InviteCode{TeamID: t.ID, Code: code, CreatedBy: currentUser(r).ID}
	if err := s.invites.Create(r.Context(), inv); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create invite code")
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}

// listInvites returns a team's outstanding invite codes (owner only).
func (s *server) listInvites(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	if !s.requireTeamOwner(w, r, t.ID) {
		return
	}
	invites, err := s.invites.ListOutstanding(r.Context(), t.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list invite codes")
		return
	}
	if invites == nil {
		invites = []domain.InviteCode{}
	}
	writeJSON(w, http.StatusOK, invites)
}

// revokeInvite deletes an outstanding invite code (owner only).
func (s *server) revokeInvite(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	if !s.requireTeamOwner(w, r, t.ID) {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	if code == "" {
		writeError(w, http.StatusBadRequest, "invite code is required")
		return
	}
	revoked, err := s.invites.Revoke(r.Context(), t.ID, code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke invite code")
		return
	}
	if !revoked {
		writeError(w, http.StatusNotFound, "invite code not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// joinTeam lets an authenticated user join a team by entering an invite code.
func (s *server) joinTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		writeError(w, http.StatusBadRequest, "invite code is required")
		return
	}
	teamID, err := s.invites.JoinByCode(r.Context(), code, currentUser(r).ID)
	if errors.Is(err, store.ErrInvalidInvite) {
		writeError(w, http.StatusNotFound, "invalid or already-used invite code")
		return
	}
	if errors.Is(err, store.ErrDuplicatedKey) {
		writeError(w, http.StatusConflict, "you are already a member of this team")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to join team")
		return
	}
	t, err := s.teams.Get(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load team")
		return
	}
	writeJSON(w, http.StatusOK, t)
}
