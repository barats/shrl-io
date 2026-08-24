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

// teamByID parses and loads the team from the {id} path segment.
func (s *server) teamByID(w http.ResponseWriter, r *http.Request) (*domain.Team, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return nil, false
	}
	t, err := s.teams.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return nil, false
	}
	return t, true
}

// requireTeamRead grants access to a team's details and links to its members
// and to admins (instance oversight); outsiders get 404 so team existence is
// not leaked.
func (s *server) requireTeamRead(w http.ResponseWriter, r *http.Request, teamID int64) bool {
	u := currentUser(r)
	if u.IsAdmin {
		return true
	}
	if _, err := s.teams.MemberRole(r.Context(), teamID, u.ID); err != nil {
		writeError(w, http.StatusNotFound, "team not found")
		return false
	}
	return true
}

// requireTeamMember grants team write access to members only.
func (s *server) requireTeamMember(w http.ResponseWriter, r *http.Request, teamID int64) bool {
	u := currentUser(r)
	if _, err := s.teams.MemberRole(r.Context(), teamID, u.ID); err != nil {
		writeError(w, http.StatusForbidden, "not a team member")
		return false
	}
	return true
}

// requireTeamOwner grants membership management to team owners only.
func (s *server) requireTeamOwner(w http.ResponseWriter, r *http.Request, teamID int64) bool {
	u := currentUser(r)
	role, err := s.teams.MemberRole(r.Context(), teamID, u.ID)
	if err != nil || role != domain.RoleOwner {
		writeError(w, http.StatusForbidden, "team owner required")
		return false
	}
	return true
}

func (s *server) createTeam(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 128 {
		writeError(w, http.StatusBadRequest, "team name must be 1-128 characters")
		return
	}
	t := &domain.Team{Name: req.Name, CreatedBy: currentUser(r).ID}
	if err := s.teams.Create(r.Context(), t); err != nil {
		if errors.Is(err, store.ErrDuplicatedKey) {
			writeError(w, http.StatusConflict, "team name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create team")
		return
	}
	// The creating admin becomes the first Team Owner.
	if err := s.teams.AddMember(r.Context(), t.ID, currentUser(r).ID, domain.RoleOwner); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add owner")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

// listTeams returns the caller's teams with their role; admins additionally
// see every team (instance oversight) with an empty role when not a member.
func (s *server) listTeams(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var teams []domain.Team
	var err error
	if u.IsAdmin {
		teams, err = s.teams.ListAll(r.Context())
	} else {
		teams, err = s.teams.ListForUser(r.Context(), u.ID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}
	items := make([]map[string]any, 0, len(teams))
	for _, t := range teams {
		role, err := s.teams.MemberRole(r.Context(), t.ID, u.ID)
		if err != nil {
			role = ""
		}
		items = append(items, map[string]any{
			"id":         t.ID,
			"name":       t.Name,
			"created_by": t.CreatedBy,
			"created_at": t.CreatedAt,
			"role":       role,
		})
	}
	writeJSON(w, http.StatusOK, items)
}

// getTeam returns a team with its members and roles.
func (s *server) getTeam(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	if !s.requireTeamRead(w, r, t.ID) {
		return
	}
	memberships, err := s.teams.ListMembers(r.Context(), t.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	members := make([]map[string]any, 0, len(memberships))
	for _, m := range memberships {
		u, err := s.users.GetByID(r.Context(), m.UserID)
		if err != nil {
			continue
		}
		members = append(members, map[string]any{
			"id":        u.ID,
			"username":  u.Username,
			"role":      m.Role,
			"joined_at": m.JoinedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         t.ID,
		"name":       t.Name,
		"created_by": t.CreatedBy,
		"created_at": t.CreatedAt,
		"members":    members,
	})
}

// listTeamLinks returns the links of a team, read-only for members.
func (s *server) listTeamLinks(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	if !s.requireTeamRead(w, r, t.ID) {
		return
	}
	links, err := s.links.ListByTeam(r.Context(), s.hostname(r), t.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list links")
		return
	}
	if links == nil {
		links = []domain.Link{}
	}
	writeJSON(w, http.StatusOK, links)
}

// createTeamLink creates a Link in the team's scope.
func (s *server) createTeamLink(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	if !s.requireTeamMember(w, r, t.ID) {
		return
	}
	s.createLinkInScope(w, r, &t.ID, currentUser(r).ID)
}

// addTeamMember adds an existing user to the team as a member.
func (s *server) addTeamMember(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	if !s.requireTeamOwner(w, r, t.ID) {
		return
	}
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	u, err := s.users.GetByUsername(r.Context(), req.Username)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err := s.teams.AddMember(r.Context(), t.ID, u.ID, domain.RoleMember); err != nil {
		if errors.Is(err, store.ErrDuplicatedKey) {
			writeError(w, http.StatusConflict, "user is already a member")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add member")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": u.ID, "username": u.Username, "role": domain.RoleMember})
}

// setTeamMemberRole promotes or demotes a member; a team always keeps at
// least one owner.
func (s *server) setTeamMemberRole(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	if !s.requireTeamOwner(w, r, t.ID) {
		return
	}
	targetID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil || targetID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req struct {
		Role domain.TeamRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role != domain.RoleOwner && req.Role != domain.RoleMember {
		writeError(w, http.StatusBadRequest, "role must be owner or member")
		return
	}
	cur, err := s.teams.MemberRole(r.Context(), t.ID, targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user is not a member")
		return
	}
	if cur == domain.RoleOwner && req.Role == domain.RoleMember {
		n, err := s.teams.CountOwners(r.Context(), t.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to count owners")
			return
		}
		if n <= 1 {
			writeError(w, http.StatusConflict, "a team must keep at least one owner")
			return
		}
	}
	if err := s.teams.SetRole(r.Context(), t.ID, targetID, req.Role); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update role")
		return
	}
	u, err := s.users.GetByID(r.Context(), targetID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load member")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": u.ID, "username": u.Username, "role": req.Role})
}

// removeTeamMember removes a member. An owner removes others; any member may
// remove themselves (leave). A team always keeps at least one owner.
func (s *server) removeTeamMember(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	targetID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil || targetID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	u := currentUser(r)
	if targetID == u.ID {
		if !s.requireTeamMember(w, r, t.ID) {
			return
		}
	} else if !s.requireTeamOwner(w, r, t.ID) {
		return
	}
	role, err := s.teams.MemberRole(r.Context(), t.ID, targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user is not a member")
		return
	}
	if role == domain.RoleOwner {
		n, err := s.teams.CountOwners(r.Context(), t.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to count owners")
			return
		}
		if n <= 1 {
			writeError(w, http.StatusConflict, "a team must keep at least one owner")
			return
		}
	}
	if _, err := s.teams.RemoveMember(r.Context(), t.ID, targetID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteTeam removes a team (admin only). Its links revert to Personal, the
// sole exception to the no-transfer rule; memberships are removed with it.
func (s *server) deleteTeam(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	t, ok := s.teamByID(w, r)
	if !ok {
		return
	}
	if err := s.links.TransferTeamLinksToPersonal(r.Context(), t.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to transfer links")
		return
	}
	if err := s.teams.Delete(r.Context(), t.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete team")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
