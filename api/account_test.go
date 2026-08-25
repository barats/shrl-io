package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

// newRealUser creates a user whose password is a real bcrypt hash (the shared
// newUser helper uses a placeholder), returning their id.
func newRealUser(t *testing.T, s *server, username, password string) int64 {
	t.Helper()
	hash, err := domain.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := &domain.User{Username: username, PasswordHash: hash}
	if err := s.users.Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u.ID
}

// loginAs signs in with a real password and returns the bearer token.
func loginAs(t *testing.T, s *server, username, password string) string {
	t.Helper()
	rec := do(t, s, "POST", "/login", "", map[string]any{"username": username, "password": password})
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s = %d, body %s", username, rec.Code, rec.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	decode(t, rec, &out)
	return out.Token
}

func TestChangePassword(t *testing.T) {
	s := newTestServer(t)
	newRealUser(t, s, "alice", "correct-horse")
	tok := loginAs(t, s, "alice", "correct-horse")

	// wrong current password is rejected
	if rec := do(t, s, "POST", "/account/password", tok, map[string]any{
		"current_password": "wrong",
		"new_password":     "new-secret-123",
	}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong current = %d, want 401", rec.Code)
	}
	// a too-short new password is rejected
	if rec := do(t, s, "POST", "/account/password", tok, map[string]any{
		"current_password": "correct-horse",
		"new_password":     "short",
	}); rec.Code != http.StatusBadRequest {
		t.Fatalf("short new = %d, want 400", rec.Code)
	}
	// success keeps the current session alive
	if rec := do(t, s, "POST", "/account/password", tok, map[string]any{
		"current_password": "correct-horse",
		"new_password":     "new-secret-123",
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("change = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "GET", "/me", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("me after change = %d, want 200 (current session kept)", rec.Code)
	}
	// the old password no longer logs in; the new one does
	if rec := do(t, s, "POST", "/login", "", map[string]any{"username": "alice", "password": "correct-horse"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password login = %d, want 401", rec.Code)
	}
	if tok2 := loginAs(t, s, "alice", "new-secret-123"); tok2 == "" {
		t.Fatal("new password login failed")
	}
}

func TestChangePasswordRevokesOtherCredentials(t *testing.T) {
	s := newTestServer(t)
	newRealUser(t, s, "alice", "correct-horse")
	tok1 := loginAs(t, s, "alice", "correct-horse")
	tok2 := loginAs(t, s, "alice", "correct-horse")

	var keyOut struct {
		Secret string `json:"secret"`
	}
	decode(t, do(t, s, "POST", "/keys", tok1, map[string]any{"name": "ci"}), &keyOut)

	// tok2 and the key authenticate before the change
	if rec := do(t, s, "GET", "/me", tok2, nil); rec.Code != http.StatusOK {
		t.Fatalf("me tok2 before = %d", rec.Code)
	}
	if rec := do(t, s, "GET", "/me", keyOut.Secret, nil); rec.Code != http.StatusOK {
		t.Fatalf("me key before = %d", rec.Code)
	}

	if rec := do(t, s, "POST", "/account/password", tok1, map[string]any{
		"current_password": "correct-horse",
		"new_password":     "new-secret-123",
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("change = %d", rec.Code)
	}

	// the current session survives; the other token and the key are revoked
	if rec := do(t, s, "GET", "/me", tok1, nil); rec.Code != http.StatusOK {
		t.Fatalf("me tok1 after = %d, want 200", rec.Code)
	}
	if rec := do(t, s, "GET", "/me", tok2, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me tok2 after = %d, want 401", rec.Code)
	}
	if rec := do(t, s, "GET", "/me", keyOut.Secret, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me key after = %d, want 401", rec.Code)
	}
}

func TestAPIKeys(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)

	// a blank name is rejected
	if rec := do(t, s, "POST", "/keys", adminTok, map[string]any{"name": "  "}); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name = %d, want 400", rec.Code)
	}

	var keyOut struct {
		Secret string        `json:"secret"`
		Key    domain.APIKey `json:"api_key"`
	}
	rec := do(t, s, "POST", "/keys", adminTok, map[string]any{"name": "ci"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create key = %d, body %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &keyOut)
	if keyOut.Secret == "" || keyOut.Key.ID == 0 || keyOut.Key.Name != "ci" {
		t.Fatalf("create key response = %+v", keyOut)
	}

	// the key authenticates on the bearer path
	if rec := do(t, s, "GET", "/me", keyOut.Secret, nil); rec.Code != http.StatusOK {
		t.Fatalf("me via key = %d", rec.Code)
	}

	// the key is listed for its owner
	if rec := do(t, s, "GET", "/keys", adminTok, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ci") {
		t.Fatalf("list keys = %d, body %s", rec.Code, rec.Body.String())
	}

	// another user cannot revoke it
	_, aliceTok := newUser(t, s, "alice", false)
	if rec := do(t, s, "DELETE", "/keys/"+strconv.FormatInt(keyOut.Key.ID, 10), aliceTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user revoke = %d, want 404", rec.Code)
	}

	// the owner revokes it and it stops working
	if rec := do(t, s, "DELETE", "/keys/"+strconv.FormatInt(keyOut.Key.ID, 10), adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d", rec.Code)
	}
	if rec := do(t, s, "GET", "/me", keyOut.Secret, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me via revoked key = %d, want 401", rec.Code)
	}
}

func TestAdminResetForcesPasswordChange(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	aliceID, aliceTok := newUser(t, s, "alice", false)
	alice := strconv.FormatInt(aliceID, 10)

	// only an admin can reset; missing users are 404
	if rec := do(t, s, "POST", "/users/"+alice+"/reset", aliceTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin reset = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "POST", "/users/999/reset", adminTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("reset missing user = %d, want 404", rec.Code)
	}

	var resetOut struct {
		Password string `json:"password"`
	}
	rec := do(t, s, "POST", "/users/"+alice+"/reset", adminTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset = %d, body %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &resetOut)
	if len(resetOut.Password) < 10 {
		t.Fatalf("temp password = %q, too short", resetOut.Password)
	}

	// the old token is revoked
	if rec := do(t, s, "GET", "/me", aliceTok, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old token after reset = %d, want 401", rec.Code)
	}

	// login with the temp password works, but everything else is gated
	tempTok := loginAs(t, s, "alice", resetOut.Password)
	for _, path := range []string{"/links", "/teams", "/keys"} {
		if rec := do(t, s, "GET", path, tempTok, nil); rec.Code != http.StatusForbidden {
			t.Fatalf("gated %s = %d, want 403", path, rec.Code)
		}
	}
	if rec := do(t, s, "GET", "/me", tempTok, nil); rec.Code != http.StatusOK {
		t.Fatalf("me while gated = %d, want 200", rec.Code)
	}

	// changing the password lifts the gate
	if rec := do(t, s, "POST", "/account/password", tempTok, map[string]any{
		"current_password": resetOut.Password,
		"new_password":     "brand-new-pass-123",
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("change after reset = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "GET", "/links", tempTok, nil); rec.Code != http.StatusOK {
		t.Fatalf("links after change = %d, want 200", rec.Code)
	}
	if rec := do(t, s, "POST", "/login", "", map[string]any{"username": "alice", "password": resetOut.Password}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("temp password login after change = %d, want 401", rec.Code)
	}
	_ = loginAs(t, s, "alice", "brand-new-pass-123")
}
