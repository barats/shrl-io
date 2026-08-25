package main

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

func TestInviteCodeFlow(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, aliceTok := newUser(t, s, "alice", false)
	_, bobTok := newUser(t, s, "bob", false)

	if rec := do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"}); rec.Code != http.StatusCreated {
		t.Fatalf("create team = %d", rec.Code)
	}

	// only an owner can generate an invite code
	if rec := do(t, s, "POST", "/teams/1/invites", aliceTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("member generate invite = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "POST", "/teams/1/invites", bobTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("outsider generate invite = %d, want 403", rec.Code)
	}
	rec := do(t, s, "POST", "/teams/1/invites", adminTok, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("owner generate invite = %d, body %s", rec.Code, rec.Body.String())
	}
	var invite struct {
		Code string `json:"code"`
	}
	decode(t, rec, &invite)
	if len(invite.Code) != 8 {
		t.Fatalf("invite code = %q, want 8 chars", invite.Code)
	}

	// only the owner can list or revoke invites
	if rec := do(t, s, "GET", "/teams/1/invites", bobTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("outsider list invites = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "GET", "/teams/1/invites", aliceTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("member list invites = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "GET", "/teams/1/invites", adminTok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), invite.Code) {
		t.Fatalf("owner list invites = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "DELETE", "/teams/1/invites/"+invite.Code, bobTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("outsider revoke invite = %d, want 403", rec.Code)
	}

	// alice joins via the code
	if rec := do(t, s, "POST", "/teams/join", aliceTok, map[string]any{"code": invite.Code}); rec.Code != 200 {
		t.Fatalf("alice join = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "GET", "/teams", aliceTok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "growth") {
		t.Fatalf("alice teams after join = %d, body %s", rec.Code, rec.Body.String())
	}

	// the used code cannot join bob; a garbage code is rejected too
	if rec := do(t, s, "POST", "/teams/join", bobTok, map[string]any{"code": invite.Code}); rec.Code != http.StatusNotFound {
		t.Fatalf("bob reuse code = %d, want 404", rec.Code)
	}
	if rec := do(t, s, "POST", "/teams/join", bobTok, map[string]any{"code": "GARBAGE"}); rec.Code != http.StatusNotFound {
		t.Fatalf("bob garbage code = %d, want 404", rec.Code)
	}

	// an existing member re-entering a fresh code gets 409, and the code stays
	// usable by someone else
	var invite2 struct{ Code string `json:"code"` }
	decode(t, do(t, s, "POST", "/teams/1/invites", adminTok, nil), &invite2)
	if rec := do(t, s, "POST", "/teams/join", aliceTok, map[string]any{"code": invite2.Code}); rec.Code != http.StatusConflict {
		t.Fatalf("member rejoin = %d, want 409", rec.Code)
	}
	if rec := do(t, s, "POST", "/teams/join", bobTok, map[string]any{"code": invite2.Code}); rec.Code != 200 {
		t.Fatalf("bob join after member-reject = %d, want 200", rec.Code)
	}

	// revoking an outstanding invite kills it; revoking an unknown one is 404
	var invite3 struct{ Code string `json:"code"` }
	decode(t, do(t, s, "POST", "/teams/1/invites", adminTok, nil), &invite3)
	if rec := do(t, s, "DELETE", "/teams/1/invites/"+invite3.Code, adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke invite = %d, want 204", rec.Code)
	}
	if rec := do(t, s, "POST", "/teams/join", bobTok, map[string]any{"code": invite3.Code}); rec.Code != http.StatusNotFound {
		t.Fatalf("join revoked code = %d, want 404", rec.Code)
	}
	if rec := do(t, s, "DELETE", "/teams/1/invites/ZZZ99999", adminTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("revoke unknown = %d, want 404", rec.Code)
	}
}

func TestAddMemberIsAdminOnly(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	aliceID, aliceTok := newUser(t, s, "alice", false)
	_, bobTok := newUser(t, s, "bob", false)

	do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"})
	// admin direct-adds alice, then promotes her to owner
	if rec := do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "alice"}); rec.Code != http.StatusCreated {
		t.Fatalf("admin add alice = %d", rec.Code)
	}
	if rec := do(t, s, "PATCH", "/teams/1/members/"+strconv.FormatInt(aliceID, 10), adminTok, map[string]any{"role": "owner"}); rec.Code != 200 {
		t.Fatalf("promote alice = %d", rec.Code)
	}
	// alice is an owner but not an admin: direct add is forbidden; she must
	// use invite codes instead
	if rec := do(t, s, "POST", "/teams/1/members", aliceTok, map[string]any{"username": "bob"}); rec.Code != http.StatusForbidden {
		t.Fatalf("owner non-admin direct add = %d, want 403", rec.Code)
	}
	var inv struct{ Code string `json:"code"` }
	decode(t, do(t, s, "POST", "/teams/1/invites", aliceTok, nil), &inv)
	if rec := do(t, s, "POST", "/teams/join", bobTok, map[string]any{"code": inv.Code}); rec.Code != 200 {
		t.Fatalf("bob join through owner code = %d", rec.Code)
	}
}

func TestUserDeletion(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	carolID, carolTok := newUser(t, s, "carol", false)

	// carol creates a personal link
	var personal domain.Link
	decode(t, do(t, s, "POST", "/links", carolTok, map[string]any{"destination": "https://personal.example"}), &personal)

	// only admin can delete a user; missing users are 404
	if rec := do(t, s, "DELETE", "/users/"+strconv.FormatInt(carolID, 10), carolTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("self delete = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "DELETE", "/users/999", adminTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing user = %d, want 404", rec.Code)
	}

	// carol is in a team (admin direct-add) with a team link
	do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"})
	do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "carol"})
	var teamLink domain.Link
	decode(t, do(t, s, "POST", "/teams/1/links", carolTok, map[string]any{"destination": "https://team.example"}), &teamLink)

	if rec := do(t, s, "DELETE", "/users/"+strconv.FormatInt(carolID, 10), adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("admin delete user = %d, body %s", rec.Code, rec.Body.String())
	}
	// carol's personal link is gone; her team link stays with the team; her
	// membership is removed
	if rec := do(t, s, "GET", "/links/"+personal.Code, adminTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted personal link = %d, want 404", rec.Code)
	}
	if rec := do(t, s, "GET", "/teams/1/links", adminTok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), teamLink.Code) {
		t.Fatalf("team links after delete = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "GET", "/teams/1", adminTok, nil); rec.Code != 200 || strings.Contains(rec.Body.String(), "carol") {
		t.Fatalf("team members after delete = %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestCannotDeleteLastTeamOwner(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	aliceID, _ := newUser(t, s, "alice", false)

	do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"})
	do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "alice"})
	// make alice the sole owner
	if rec := do(t, s, "PATCH", "/teams/1/members/2", adminTok, map[string]any{"role": "owner"}); rec.Code != 200 {
		t.Fatalf("promote alice = %d", rec.Code)
	}
	if rec := do(t, s, "PATCH", "/teams/1/members/1", adminTok, map[string]any{"role": "member"}); rec.Code != 200 {
		t.Fatalf("demote admin = %d", rec.Code)
	}
	// admin (instance privilege) cannot delete the sole owner of a team
	if rec := do(t, s, "DELETE", "/users/"+strconv.FormatInt(aliceID, 10), adminTok, nil); rec.Code != http.StatusConflict {
		t.Fatalf("delete sole owner = %d, want 409", rec.Code)
	}
}
