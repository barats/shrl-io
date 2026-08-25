package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/store"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	ctx := context.Background()
	for _, migrate := range []func(context.Context) error{
		store.NewLinkStore(db).Migrate,
		store.NewHostnameStore(db).Migrate,
		store.NewUserStore(db).Migrate,
		store.NewTeamStore(db).Migrate,
		store.NewInviteStore(db).Migrate,
		store.NewAnalyticsStore(db).Migrate,
	} {
		if err := migrate(ctx); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return db
}

// newTestServer builds a full server over sqlite + miniredis. The default
// hostname is registered so link creation works out of the box.
func newTestServer(t *testing.T) *server {
	t.Helper()
	db := testDB(t)
	rdb := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: rdb.Addr()})
	hs := store.NewHostnameStore(db)
	if err := hs.Create(context.Background(), &domain.Hostname{Name: "localhost"}); err != nil {
		t.Fatalf("register hostname: %v", err)
	}
	return &server{
		links:     store.NewLinkStore(db),
		analytics: store.NewAnalyticsStore(db),
		users:     store.NewUserStore(db),
		hostnames: hs,
		teams:     store.NewTeamStore(db),
		invites:   store.NewInviteStore(db),
		linkCache: cache.NewLinkCache(client),
		cfg:       config{defaultHostname: "localhost", retentionDays: 30},
	}
}

// newUser creates a user and a valid bearer token, returning id and token.
func newUser(t *testing.T, s *server, username string, isAdmin bool) (int64, string) {
	t.Helper()
	u := &domain.User{Username: username, PasswordHash: "hash", IsAdmin: isAdmin}
	if err := s.users.Create(context.Background(), u); err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	token := "tok-" + username
	tk := &domain.Token{UserID: u.ID, Hash: domain.HashToken(token), ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.users.CreateToken(context.Background(), tk); err != nil {
		t.Fatalf("create token %s: %v", username, err)
	}
	return u.ID, token
}

// do performs an authenticated request against the full auth-wrapped router.
func do(t *testing.T, s *server, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.auth(s.routes()).ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
}

func TestTeamLifecycleAndAccess(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, aliceTok := newUser(t, s, "alice", false)
	carolID, carolTok := newUser(t, s, "carol", false)
	_, bobTok := newUser(t, s, "bob", false)

	// only an admin can create a team
	if rec := do(t, s, "POST", "/teams", aliceTok, map[string]any{"name": "growth"}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin create team = %d, want 403", rec.Code)
	}
	rec := do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create team = %d, body %s", rec.Code, rec.Body.String())
	}
	var team domain.Team
	decode(t, rec, &team)
	if team.ID == 0 || team.CreatedBy == 0 {
		t.Fatalf("unexpected team: %+v", team)
	}
	if rec := do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate team name = %d, want 409", rec.Code)
	}

	// the creating admin is an owner
	var teams []map[string]any
	decode(t, do(t, s, "GET", "/teams", adminTok, nil), &teams)
	if len(teams) != 1 || teams[0]["role"] != "owner" {
		t.Fatalf("admin teams = %+v, want one with role owner", teams)
	}

	// an outsider cannot read the team
	if rec := do(t, s, "GET", "/teams/1", bobTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider get team = %d, want 404", rec.Code)
	}

	// owner adds alice and carol as members
	if rec := do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "alice"}); rec.Code != http.StatusCreated {
		t.Fatalf("add alice = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "carol"}); rec.Code != http.StatusCreated {
		t.Fatalf("add carol = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "alice"}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate member = %d, want 409", rec.Code)
	}
	if rec := do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "ghost"}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown user = %d, want 404", rec.Code)
	}

	// carol creates a link in the team
	rec = do(t, s, "POST", "/teams/1/links", carolTok, map[string]any{"hostname": "localhost", "destination": "https://example.com"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("carol create team link = %d, body %s", rec.Code, rec.Body.String())
	}
	var link domain.Link
	decode(t, rec, &link)
	if link.TeamID == nil || *link.TeamID != team.ID || link.CreatedBy != carolID {
		t.Fatalf("team link not scoped: %+v", link)
	}

	// members read team links; outsiders cannot
	if rec := do(t, s, "GET", "/teams/1/links", aliceTok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), link.Code) {
		t.Fatalf("member list team links = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "GET", "/teams/1/links", bobTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider list team links = %d, want 404", rec.Code)
	}
	if rec := do(t, s, "GET", "/links/"+link.Code, aliceTok, nil); rec.Code != 200 {
		t.Fatalf("member read team link = %d, want 200", rec.Code)
	}
	if rec := do(t, s, "GET", "/links/"+link.Code+"/analytics", aliceTok, nil); rec.Code != 200 {
		t.Fatalf("member read analytics = %d, want 200", rec.Code)
	}
	if rec := do(t, s, "GET", "/links/"+link.Code, bobTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider read team link = %d, want 404", rec.Code)
	}
	// outsider cannot create in the team
	if rec := do(t, s, "POST", "/teams/1/links", bobTok, map[string]any{"destination": "https://example.com"}); rec.Code != http.StatusForbidden {
		t.Fatalf("outsider create team link = %d, want 403", rec.Code)
	}

	// a plain member cannot manage another member's link
	if rec := do(t, s, "PATCH", "/links/"+link.Code, aliceTok, map[string]any{"destination": "https://new.example", "remark": "x"}); rec.Code != http.StatusForbidden {
		t.Fatalf("member patch link = %d, want 403", rec.Code)
	}
	// the creator can manage their own link
	if rec := do(t, s, "PATCH", "/links/"+link.Code, carolTok, map[string]any{"destination": "https://new.example", "remark": "x"}); rec.Code != 200 {
		t.Fatalf("creator patch link = %d, want 200", rec.Code)
	}
	// a team owner can manage any team link
	if rec := do(t, s, "PATCH", "/links/"+link.Code, adminTok, map[string]any{"destination": "https://owner.example"}); rec.Code != 200 {
		t.Fatalf("owner patch link = %d, want 200", rec.Code)
	}
	if rec := do(t, s, "POST", "/links/"+link.Code+"/disable", adminTok, nil); rec.Code != 200 {
		t.Fatalf("owner disable link = %d, want 200", rec.Code)
	}
}

func TestPersonalIsolation(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, carolTok := newUser(t, s, "carol", false)
	_, aliceTok := newUser(t, s, "alice", false)

	do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"})
	do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "carol"})
	do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "alice"})

	// carol creates a team link and a personal link
	var teamLink domain.Link
	decode(t, do(t, s, "POST", "/teams/1/links", carolTok, map[string]any{"destination": "https://team.example"}), &teamLink)
	var personalLink domain.Link
	decode(t, do(t, s, "POST", "/links", carolTok, map[string]any{"destination": "https://personal.example"}), &personalLink)

	// carol's personal list excludes the team link
	var personal []domain.Link
	decode(t, do(t, s, "GET", "/links", carolTok, nil), &personal)
	if len(personal) != 1 || personal[0].Code != personalLink.Code {
		t.Fatalf("carol personal list = %+v, want only %s", personal, personalLink.Code)
	}

	// a teammate cannot see carol's personal link; neither can the admin
	if rec := do(t, s, "GET", "/links/"+personalLink.Code, aliceTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("teammate read personal link = %d, want 404", rec.Code)
	}
	if rec := do(t, s, "GET", "/links/"+personalLink.Code, adminTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("admin read personal link = %d, want 404", rec.Code)
	}

	// team links stay visible to teammates through the team dashboard
	if rec := do(t, s, "GET", "/teams/1/links", aliceTok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), teamLink.Code) {
		t.Fatalf("alice team links = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "GET", "/links/"+teamLink.Code, aliceTok, nil); rec.Code != 200 {
		t.Fatalf("alice read team link = %d, want 200", rec.Code)
	}
	if rec := do(t, s, "GET", "/links", carolTok, nil); rec.Code != 200 {
		t.Fatalf("carol list = %d", rec.Code)
	}
}

func TestTeamMembershipPowers(t *testing.T) {
	s := newTestServer(t)
	adminID, adminTok := newUser(t, s, "admin", true)
	aliceID, aliceTok := newUser(t, s, "alice", false)
	bobID, bobTok := newUser(t, s, "bob", false)

	do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"})
	do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "alice"})

	// a plain member cannot manage membership
	if rec := do(t, s, "POST", "/teams/1/members", aliceTok, map[string]any{"username": "bob"}); rec.Code != http.StatusForbidden {
		t.Fatalf("member add = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "PATCH", "/teams/1/members/"+strconv.FormatInt(adminID, 10), aliceTok, map[string]any{"role": "member"}); rec.Code != http.StatusForbidden {
		t.Fatalf("member demote = %d, want 403", rec.Code)
	}

	// the last owner cannot be demoted or removed
	if rec := do(t, s, "PATCH", "/teams/1/members/"+strconv.FormatInt(adminID, 10), adminTok, map[string]any{"role": "member"}); rec.Code != http.StatusConflict {
		t.Fatalf("self demote last owner = %d, want 409", rec.Code)
	}
	if rec := do(t, s, "DELETE", "/teams/1/members/"+strconv.FormatInt(adminID, 10), adminTok, nil); rec.Code != http.StatusConflict {
		t.Fatalf("remove last owner = %d, want 409", rec.Code)
	}

	// a member may leave voluntarily
	if rec := do(t, s, "DELETE", "/teams/1/members/"+strconv.FormatInt(aliceID, 10), aliceTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("member leave = %d, want 204", rec.Code)
	}

	// admin is no longer implicitly an owner: promote bob and demote admin to
	// member. Admin loses owner powers but keeps admin privilege: direct
	// member add is admin-only, not owner-only (ADR 0010).
	do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "bob"})
	if rec := do(t, s, "PATCH", "/teams/1/members/"+strconv.FormatInt(bobID, 10), adminTok, map[string]any{"role": "owner"}); rec.Code != 200 {
		t.Fatalf("promote bob = %d", rec.Code)
	}
	if rec := do(t, s, "PATCH", "/teams/1/members/"+strconv.FormatInt(adminID, 10), adminTok, map[string]any{"role": "member"}); rec.Code != 200 {
		t.Fatalf("demote admin = %d", rec.Code)
	}
	// a demoted admin can still add members directly...
	newUser(t, s, "dave", false)
	if rec := do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "dave"}); rec.Code != http.StatusCreated {
		t.Fatalf("demoted admin add member = %d, want 201", rec.Code)
	}
	if rec := do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "ghost"}); rec.Code != http.StatusNotFound {
		t.Fatalf("admin add unknown user = %d, want 404", rec.Code)
	}
	// ...but cannot promote/demote without the owner role
	if rec := do(t, s, "PATCH", "/teams/1/members/"+strconv.FormatInt(adminID, 10), adminTok, map[string]any{"role": "owner"}); rec.Code != http.StatusForbidden {
		t.Fatalf("admin-not-owner promote = %d, want 403", rec.Code)
	}
	// admin can still read the team as instance oversight
	if rec := do(t, s, "GET", "/teams/1", adminTok, nil); rec.Code != 200 {
		t.Fatalf("admin oversight read = %d, want 200", rec.Code)
	}
	// bob (now owner) can remove the admin from the team
	if rec := do(t, s, "DELETE", "/teams/1/members/"+strconv.FormatInt(adminID, 10), bobTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("bob removes admin = %d, want 204", rec.Code)
	}
}

func TestMemberRemovalKeepsLinksInTeam(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, carolTok := newUser(t, s, "carol", false)

	do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"})
	do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "carol"})
	var link domain.Link
	decode(t, do(t, s, "POST", "/teams/1/links", carolTok, map[string]any{"destination": "https://example.com"}), &link)

	// owner removes carol; her link stays with the team
	if rec := do(t, s, "DELETE", "/teams/1/members/2", adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("remove carol = %d, want 204", rec.Code)
	}
	if rec := do(t, s, "GET", "/links/"+link.Code, carolTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("removed member read link = %d, want 404", rec.Code)
	}
	if rec := do(t, s, "GET", "/teams/1/links", adminTok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), link.Code) {
		t.Fatalf("team links after removal = %d, body %s", rec.Code, rec.Body.String())
	}
	// an owner can clean up the orphaned link
	if rec := do(t, s, "DELETE", "/links/"+link.Code, adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("owner delete orphan link = %d, want 204", rec.Code)
	}
}

func TestTeamDeletion(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, carolTok := newUser(t, s, "carol", false)

	do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"})
	do(t, s, "POST", "/teams/1/members", adminTok, map[string]any{"username": "carol"})
	var link domain.Link
	decode(t, do(t, s, "POST", "/teams/1/links", carolTok, map[string]any{"destination": "https://example.com"}), &link)

	// only admin can delete a team
	if rec := do(t, s, "DELETE", "/teams/1", carolTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin delete team = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "DELETE", "/teams/1", adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("admin delete team = %d, body %s", rec.Code, rec.Body.String())
	}
	// the team is gone; its links reverted to Personal for their creators
	if rec := do(t, s, "GET", "/teams/1", adminTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("get deleted team = %d, want 404", rec.Code)
	}
	var personal []domain.Link
	decode(t, do(t, s, "GET", "/links", carolTok, nil), &personal)
	if len(personal) != 1 || personal[0].Code != link.Code || personal[0].TeamID != nil {
		t.Fatalf("reverted links = %+v, want %s personal", personal, link.Code)
	}
	if rec := do(t, s, "GET", "/links/"+link.Code, carolTok, nil); rec.Code != 200 {
		t.Fatalf("creator read reverted link = %d, want 200", rec.Code)
	}
}
