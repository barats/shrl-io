package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	"github.com/barats/shrl-io/internal/service"
	"github.com/barats/shrl-io/internal/store"
)

// apiLink mirrors the Link JSON shape: created_by is a username and team_id
// the team Ref (ADR 0021).
type apiLink struct {
	Code      string  `json:"code"`
	CreatedBy string  `json:"created_by"`
	TeamID    *string `json:"team_id"`
	Disabled  bool    `json:"disabled"`
}

// createTeamRef creates a team as tok and returns its Ref.
func createTeamRef(t *testing.T, s *server, tok, name string) string {
	t.Helper()
	var out struct {
		ID string `json:"id"`
	}
	decode(t, do(t, s, "POST", "/teams", tok, map[string]any{"name": name}), &out)
	if out.ID == "" {
		t.Fatalf("team creation returned no ref")
	}
	return out.ID
}

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
		store.NewBaseURLStore(db).Migrate,
		store.NewUserStore(db).Migrate,
		store.NewTeamStore(db).Migrate,
		store.NewInviteStore(db).Migrate,
		store.NewAnalyticsStore(db).Migrate,
		store.NewSettingStore(db).Migrate,
	} {
		if err := migrate(ctx); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return db
}

// newTestServer builds a full server over sqlite + miniredis. The default
// base URL is registered so link creation works out of the box.
func newTestServer(t *testing.T) *server {
	t.Helper()
	db := testDB(t)
	rdb := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: rdb.Addr()})
	bs := store.NewBaseURLStore(db)
	if err := bs.Create(context.Background(), &domain.BaseURL{BaseURL: "http://localhost:8080"}); err != nil {
		t.Fatalf("register base URL: %v", err)
	}
	links := store.NewLinkStore(db)
	analytics := store.NewAnalyticsStore(db)
	teams := store.NewTeamStore(db)
	settings := store.NewSettingStore(db)
	linkCache := cache.NewLinkCache(client)
	return &server{
		links:     links,
		analytics: analytics,
		users:     store.NewUserStore(db),
		baseURLs:  bs,
		teams:     teams,
		invites:   store.NewInviteStore(db),
		settings:  settings,
		linkCache: linkCache,
		linkSvc:   service.NewLinkService(links, analytics, bs, teams, settings, linkCache, "http://localhost:8080", 30),
		cfg:       config{defaultBaseURL: "http://localhost:8080", retentionDays: 30, tokenTTL: time.Hour, internalSecret: "test-secret"},
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
	_, carolTok := newUser(t, s, "carol", false)
	_, bobTok := newUser(t, s, "bob", false)

	// only an admin can create a team
	if rec := do(t, s, "POST", "/teams", aliceTok, map[string]any{"name": "growth"}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin create team = %d, want 403", rec.Code)
	}
	rec := do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create team = %d, body %s", rec.Code, rec.Body.String())
	}
	var team struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedBy string `json:"created_by"`
	}
	decode(t, rec, &team)
	if len(team.ID) != domain.RefLength || team.Name != "growth" || team.CreatedBy != "admin" {
		t.Fatalf("unexpected team: %+v", team)
	}
	ref := team.ID
	if rec := do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "growth"}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate team name = %d, want 409", rec.Code)
	}

	// the creating admin is an owner
	var teams []map[string]any
	decode(t, do(t, s, "GET", "/teams", adminTok, nil), &teams)
	if len(teams) != 1 || teams[0]["role"] != "owner" || teams[0]["id"] != ref {
		t.Fatalf("admin teams = %+v, want one with role owner and ref %s", teams, ref)
	}

	// an outsider cannot read the team
	if rec := do(t, s, "GET", "/teams/"+ref, bobTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider get team = %d, want 404", rec.Code)
	}

	// owner adds alice and carol as members
	if rec := do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "alice"}); rec.Code != http.StatusCreated {
		t.Fatalf("add alice = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "carol"}); rec.Code != http.StatusCreated {
		t.Fatalf("add carol = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "alice"}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate member = %d, want 409", rec.Code)
	}
	if rec := do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "ghost"}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown user = %d, want 404", rec.Code)
	}

	// carol creates a link in the team
	rec = do(t, s, "POST", "/teams/"+ref+"/links", carolTok, map[string]any{"base_url": "http://localhost:8080", "destination": "https://example.com"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("carol create team link = %d, body %s", rec.Code, rec.Body.String())
	}
	var link apiLink
	decode(t, rec, &link)
	if link.TeamID == nil || *link.TeamID != ref || link.CreatedBy != "carol" {
		t.Fatalf("team link not scoped: %+v", link)
	}

	// members read team links; outsiders cannot
	if rec := do(t, s, "GET", "/teams/"+ref+"/links", aliceTok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), link.Code) {
		t.Fatalf("member list team links = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "GET", "/teams/"+ref+"/links", bobTok, nil); rec.Code != http.StatusNotFound {
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
	if rec := do(t, s, "POST", "/teams/"+ref+"/links", bobTok, map[string]any{"destination": "https://example.com"}); rec.Code != http.StatusForbidden {
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

	ref := createTeamRef(t, s, adminTok, "growth")
	do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "carol"})
	do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "alice"})

	// carol creates a team link and a personal link
	var teamLink apiLink
	decode(t, do(t, s, "POST", "/teams/"+ref+"/links", carolTok, map[string]any{"destination": "https://team.example"}), &teamLink)
	var personalLink apiLink
	decode(t, do(t, s, "POST", "/links", carolTok, map[string]any{"destination": "https://personal.example"}), &personalLink)

	// carol's personal list excludes the team link
	var personal []apiLink
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
	if rec := do(t, s, "GET", "/teams/"+ref+"/links", aliceTok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), teamLink.Code) {
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
	_, adminTok := newUser(t, s, "admin", true)
	_, aliceTok := newUser(t, s, "alice", false)
	_, bobTok := newUser(t, s, "bob", false)

	ref := createTeamRef(t, s, adminTok, "growth")
	do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "alice"})

	// a plain member cannot manage membership
	if rec := do(t, s, "POST", "/teams/"+ref+"/members", aliceTok, map[string]any{"username": "bob"}); rec.Code != http.StatusForbidden {
		t.Fatalf("member add = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "PATCH", "/teams/"+ref+"/members/admin", aliceTok, map[string]any{"role": "member"}); rec.Code != http.StatusForbidden {
		t.Fatalf("member demote = %d, want 403", rec.Code)
	}

	// the last owner cannot be demoted or removed
	if rec := do(t, s, "PATCH", "/teams/"+ref+"/members/admin", adminTok, map[string]any{"role": "member"}); rec.Code != http.StatusConflict {
		t.Fatalf("self demote last owner = %d, want 409", rec.Code)
	}
	if rec := do(t, s, "DELETE", "/teams/"+ref+"/members/admin", adminTok, nil); rec.Code != http.StatusConflict {
		t.Fatalf("remove last owner = %d, want 409", rec.Code)
	}

	// a member may leave voluntarily
	if rec := do(t, s, "DELETE", "/teams/"+ref+"/members/alice", aliceTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("member leave = %d, want 204", rec.Code)
	}

	// admin is no longer implicitly an owner: promote bob and demote admin to
	// member. Admin loses owner powers but keeps admin privilege: direct
	// member add is admin-only, not owner-only (ADR 0010).
	do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "bob"})
	if rec := do(t, s, "PATCH", "/teams/"+ref+"/members/bob", adminTok, map[string]any{"role": "owner"}); rec.Code != 200 {
		t.Fatalf("promote bob = %d", rec.Code)
	}
	if rec := do(t, s, "PATCH", "/teams/"+ref+"/members/admin", adminTok, map[string]any{"role": "member"}); rec.Code != 200 {
		t.Fatalf("demote admin = %d", rec.Code)
	}
	// a demoted admin can still add members directly...
	newUser(t, s, "dave", false)
	if rec := do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "dave"}); rec.Code != http.StatusCreated {
		t.Fatalf("demoted admin add member = %d, want 201", rec.Code)
	}
	if rec := do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "ghost"}); rec.Code != http.StatusNotFound {
		t.Fatalf("admin add unknown user = %d, want 404", rec.Code)
	}
	// ...but cannot promote/demote without the owner role
	if rec := do(t, s, "PATCH", "/teams/"+ref+"/members/admin", adminTok, map[string]any{"role": "owner"}); rec.Code != http.StatusForbidden {
		t.Fatalf("admin-not-owner promote = %d, want 403", rec.Code)
	}
	// admin can still read the team as instance oversight
	if rec := do(t, s, "GET", "/teams/"+ref, adminTok, nil); rec.Code != 200 {
		t.Fatalf("admin oversight read = %d, want 200", rec.Code)
	}
	// bob (now owner) can remove the admin from the team
	if rec := do(t, s, "DELETE", "/teams/"+ref+"/members/admin", bobTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("bob removes admin = %d, want 204", rec.Code)
	}
}

func TestMemberRemovalKeepsLinksInTeam(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, carolTok := newUser(t, s, "carol", false)

	ref := createTeamRef(t, s, adminTok, "growth")
	do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "carol"})
	var link apiLink
	decode(t, do(t, s, "POST", "/teams/"+ref+"/links", carolTok, map[string]any{"destination": "https://example.com"}), &link)

	// owner removes carol; her link stays with the team
	if rec := do(t, s, "DELETE", "/teams/"+ref+"/members/carol", adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("remove carol = %d, want 204", rec.Code)
	}
	if rec := do(t, s, "GET", "/links/"+link.Code, carolTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("removed member read link = %d, want 404", rec.Code)
	}
	if rec := do(t, s, "GET", "/teams/"+ref+"/links", adminTok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), link.Code) {
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

	ref := createTeamRef(t, s, adminTok, "growth")
	do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "carol"})
	var link apiLink
	decode(t, do(t, s, "POST", "/teams/"+ref+"/links", carolTok, map[string]any{"destination": "https://example.com"}), &link)

	// only admin can delete a team
	if rec := do(t, s, "DELETE", "/teams/"+ref, carolTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin delete team = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "DELETE", "/teams/"+ref, adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("admin delete team = %d, body %s", rec.Code, rec.Body.String())
	}
	// the team is gone; its links reverted to Personal for their creators
	if rec := do(t, s, "GET", "/teams/"+ref, adminTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("get deleted team = %d, want 404", rec.Code)
	}
	var personal []apiLink
	decode(t, do(t, s, "GET", "/links", carolTok, nil), &personal)
	if len(personal) != 1 || personal[0].Code != link.Code || personal[0].TeamID != nil {
		t.Fatalf("reverted links = %+v, want %s personal", personal, link.Code)
	}
	if rec := do(t, s, "GET", "/links/"+link.Code, carolTok, nil); rec.Code != 200 {
		t.Fatalf("creator read reverted link = %d, want 200", rec.Code)
	}
}

func TestTeamRename(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, aliceTok := newUser(t, s, "alice", false)
	_, carolTok := newUser(t, s, "carol", false)

	ref := createTeamRef(t, s, adminTok, "growth")
	do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "alice"})
	do(t, s, "POST", "/teams", adminTok, map[string]any{"name": "dupe"})

	// the owner renames
	if rec := do(t, s, "PATCH", "/teams/"+ref, adminTok, map[string]any{"name": "Growth v2"}); rec.Code != http.StatusOK {
		t.Fatalf("owner rename = %d, body %s", rec.Code, rec.Body.String())
	}

	// a plain member cannot rename
	if rec := do(t, s, "PATCH", "/teams/"+ref, aliceTok, map[string]any{"name": "nope"}); rec.Code != http.StatusForbidden {
		t.Fatalf("member rename = %d, want 403", rec.Code)
	}
	// an outsider gets 404 so team existence is not leaked
	if rec := do(t, s, "PATCH", "/teams/"+ref, carolTok, map[string]any{"name": "nope"}); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider rename = %d, want 404", rec.Code)
	}
	// a duplicate name conflicts
	if rec := do(t, s, "PATCH", "/teams/"+ref, adminTok, map[string]any{"name": "dupe"}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate rename = %d, want 409", rec.Code)
	}
	// a blank name is rejected
	if rec := do(t, s, "PATCH", "/teams/"+ref, adminTok, map[string]any{"name": "   "}); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank rename = %d, want 400", rec.Code)
	}
	// the renamed name is persisted and readable
	if rec := do(t, s, "GET", "/teams/"+ref, adminTok, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Growth v2") {
		t.Fatalf("get renamed team = %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestTeamDashboardEndpoints(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, aliceTok := newUser(t, s, "alice", false)
	_, bobTok := newUser(t, s, "bob", false)

	ref := createTeamRef(t, s, adminTok, "growth")
	do(t, s, "POST", "/teams/"+ref+"/members", adminTok, map[string]any{"username": "alice"})
	if rec := do(t, s, "POST", "/teams/"+ref+"/links", aliceTok, map[string]any{"destination": "https://example.com"}); rec.Code != http.StatusCreated {
		t.Fatalf("alice create team link = %d, body %s", rec.Code, rec.Body.String())
	}

	// a member reads the full team dashboard; personal isolation holds.
	var d service.Dashboard
	if rec := do(t, s, "GET", "/teams/"+ref+"/dashboard", aliceTok, nil); rec.Code != 200 {
		t.Fatalf("member team dashboard = %d, body %s", rec.Code, rec.Body.String())
	} else {
		decode(t, rec, &d)
		if d.TotalLinks != 1 || d.ActiveLinks != 1 || d.DisabledLinks != 0 {
			t.Errorf("cards = total %d active %d disabled %d, want 1/1/0", d.TotalLinks, d.ActiveLinks, d.DisabledLinks)
		}
	}

	// an admin not in the team can still read it (instance oversight)
	if rec := do(t, s, "GET", "/teams/"+ref+"/dashboard", adminTok, nil); rec.Code != 200 {
		t.Fatalf("admin oversight team dashboard = %d, want 200", rec.Code)
	}
	// an outsider gets 404 so team existence is not leaked
	if rec := do(t, s, "GET", "/teams/"+ref+"/dashboard", bobTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider team dashboard = %d, want 404", rec.Code)
	}

	// breakdowns and top-links follow the same access rules
	if rec := do(t, s, "GET", "/teams/"+ref+"/stats/breakdowns?dimension=country", aliceTok, nil); rec.Code != 200 {
		t.Fatalf("member team breakdowns = %d, want 200", rec.Code)
	}
	if rec := do(t, s, "GET", "/teams/"+ref+"/stats/top-links?metric=visitors", aliceTok, nil); rec.Code != 200 {
		t.Fatalf("member team top links = %d, want 200", rec.Code)
	}
	if rec := do(t, s, "GET", "/teams/"+ref+"/stats/breakdowns?dimension=country", bobTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider team breakdowns = %d, want 404", rec.Code)
	}
	if rec := do(t, s, "GET", "/teams/"+ref+"/stats/top-links", bobTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider team top links = %d, want 404", rec.Code)
	}
	// invalid dimension is rejected for members too
	if rec := do(t, s, "GET", "/teams/"+ref+"/stats/breakdowns?dimension=bogus", aliceTok, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid dimension = %d, want 400", rec.Code)
	}
}

// TestTeamRefIsolation pins the isolation boundary behind opaque team refs
// (ADR 0021): guessing a ref (or reusing a legacy numeric id) yields the same
// 404 as a nonexistent team, with no team name or existence leak, and no
// numeric ids appear in team payloads.
func TestTeamRefIsolation(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, bobTok := newUser(t, s, "bob", false)

	ref := createTeamRef(t, s, adminTok, "secret team")
	otherRef := createTeamRef(t, s, adminTok, "other")
	if ref == otherRef {
		t.Fatalf("two teams share the ref %s", ref)
	}

	// an outsider and a fake ref produce the identical 404 body
	outsider := do(t, s, "GET", "/teams/"+ref, bobTok, nil)
	fake := do(t, s, "GET", "/teams/zzzzzzzzzz", bobTok, nil)
	legacy := do(t, s, "GET", "/teams/1", bobTok, nil)
	for _, rec := range []*httptest.ResponseRecorder{outsider, fake, legacy} {
		if rec.Code != http.StatusNotFound || strings.TrimSpace(rec.Body.String()) != `{"error":"team not found"}` {
			t.Fatalf("get team = %d, body %s, want 404 team not found", rec.Code, rec.Body.String())
		}
	}
	if outsider.Body.String() != fake.Body.String() || outsider.Body.String() != legacy.Body.String() {
		t.Fatalf("404 bodies diverge: %s | %s | %s", outsider.Body.String(), fake.Body.String(), legacy.Body.String())
	}

	// the same holds for the nested read endpoints
	for _, path := range []string{"/links", "/dashboard", "/stats", "/stats/breakdowns", "/stats/top-links"} {
		if rec := do(t, s, "GET", "/teams/"+ref+path, bobTok, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("outsider GET %s = %d, want 404", path, rec.Code)
		}
	}

	// team payloads carry no numeric ids: members are usernames, created_by
	// is a username
	var detail struct {
		ID        string `json:"id"`
		CreatedBy string `json:"created_by"`
		Members   []map[string]any
	}
	decode(t, do(t, s, "GET", "/teams/"+ref, adminTok, nil), &detail)
	if detail.ID != ref || detail.CreatedBy != "admin" {
		t.Fatalf("team detail = %+v, want ref %s created by admin", detail, ref)
	}
	if len(detail.Members) != 1 {
		t.Fatalf("members = %+v, want 1", detail.Members)
	}
	for _, m := range detail.Members {
		if _, has := m["id"]; has {
			t.Fatalf("member payload exposes numeric id: %+v", m)
		}
		if m["username"] == "" {
			t.Fatalf("member payload missing username: %+v", m)
		}
	}
}
