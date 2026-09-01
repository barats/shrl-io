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
	"github.com/barats/shrl-io/internal/ratelimit"
	"github.com/barats/shrl-io/internal/service"
	"github.com/barats/shrl-io/internal/store"
)

func newTestServer(t *testing.T) *server {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	ctx := context.Background()
	for _, migrate := range []func(context.Context) error{
		store.NewLinkStore(db).Migrate,
		store.NewBaseURLStore(db).Migrate,
		store.NewUserStore(db).Migrate,
		store.NewTeamStore(db).Migrate,
		store.NewAnalyticsStore(db).Migrate,
		store.NewSettingStore(db).Migrate,
	} {
		if err := migrate(ctx); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	bs := store.NewBaseURLStore(db)
	if err := bs.Create(ctx, &domain.BaseURL{BaseURL: "http://localhost:8080"}); err != nil {
		t.Fatalf("register base URL: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	links := store.NewLinkStore(db)
	analytics := store.NewAnalyticsStore(db)
	users := store.NewUserStore(db)
	teams := store.NewTeamStore(db)
	settings := store.NewSettingStore(db)
	linkCache := cache.NewLinkCache(client)
	svc := service.NewLinkService(links, analytics, bs, teams, settings, linkCache, "http://localhost:8080", 30)
	return &server{
		users: users,
		teams: teams,
		svc:   svc,
		rl:    ratelimit.New(client),
		cfg: config{
			defaultBaseURL: "http://localhost:8080",
			retentionDays:  30,
			ipLimit:        60,
			keyReadLimit:   300,
			keyWriteLimit:  30,
			failLimit:      10,
			rateWindow:     time.Minute,
		},
	}
}

// newKeyUser creates a user with an API key, returning the id and the
// plaintext key.
func newKeyUser(t *testing.T, s *server, username string) (int64, string) {
	t.Helper()
	u := &domain.User{Username: username, PasswordHash: "hash"}
	if err := s.users.Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	secret := "key-" + username
	k := &domain.APIKey{UserID: u.ID, Name: "ci", Hash: domain.HashToken(secret)}
	if err := s.users.CreateKey(context.Background(), k); err != nil {
		t.Fatalf("create key: %v", err)
	}
	return u.ID, secret
}

func do(t *testing.T, s *server, method, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	s.guard(s.routes()).ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
}

func TestAuthRequiresKey(t *testing.T) {
	s := newTestServer(t)
	_, key := newKeyUser(t, s, "alice")

	if rec := do(t, s, "GET", "/v1/links", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key = %d, want 401", rec.Code)
	}
	if rec := do(t, s, "GET", "/v1/links", "wrong-key", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad key = %d, want 401", rec.Code)
	}
	if rec := do(t, s, "GET", "/v1/links", key, nil); rec.Code != http.StatusOK {
		t.Fatalf("valid key = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestCreateAndManageLink(t *testing.T) {
	s := newTestServer(t)
	_, key := newKeyUser(t, s, "alice")

	rec := do(t, s, "POST", "/v1/links", key, map[string]any{"destination": "https://example.com"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, body %s", rec.Code, rec.Body.String())
	}
	var l domain.Link
	decode(t, rec, &l)
	if l.Code == "" || l.Destination != "https://example.com" {
		t.Fatalf("created link = %+v", l)
	}

	// get, list, update, disable, enable
	if rec := do(t, s, "GET", "/v1/links/"+l.Code, key, nil); rec.Code != http.StatusOK {
		t.Fatalf("get = %d", rec.Code)
	}
	if rec := do(t, s, "GET", "/v1/links", key, nil); rec.Code != http.StatusOK {
		t.Fatalf("list = %d", rec.Code)
	}
	if rec := do(t, s, "PATCH", "/v1/links/"+l.Code, key, map[string]any{"destination": "https://new.example.com"}); rec.Code != http.StatusOK {
		t.Fatalf("update = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, "POST", "/v1/links/"+l.Code+"/disable", key, nil); rec.Code != http.StatusOK {
		t.Fatalf("disable = %d", rec.Code)
	}
	if rec := do(t, s, "POST", "/v1/links/"+l.Code+"/enable", key, nil); rec.Code != http.StatusOK {
		t.Fatalf("enable = %d", rec.Code)
	}

	// analytics + supporting reads
	if rec := do(t, s, "GET", "/v1/links/"+l.Code+"/analytics", key, nil); rec.Code != http.StatusOK {
		t.Fatalf("analytics = %d", rec.Code)
	}
	if rec := do(t, s, "GET", "/v1/base-urls", key, nil); rec.Code != http.StatusOK {
		t.Fatalf("base URLs = %d", rec.Code)
	}
	if rec := do(t, s, "GET", "/v1/teams", key, nil); rec.Code != http.StatusOK {
		t.Fatalf("teams = %d", rec.Code)
	}
}

func TestStatsOnAuthAPI(t *testing.T) {
	s := newTestServer(t)
	_, key := newKeyUser(t, s, "alice")

	rec := do(t, s, "GET", "/v1/stats", key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats = %d, body %s", rec.Code, rec.Body.String())
	}
	var st struct {
		TotalLinks    int64 `json:"total_links"`
		TotalVisits   int64 `json:"total_visits"`
		WindowVisits  int64 `json:"window_visits"`
		WindowUniques int64 `json:"window_uniques"`
		Timeseries    []any `json:"timeseries"`
	}
	decode(t, rec, &st)
	if st.TotalLinks != 0 || st.TotalVisits != 0 || st.WindowVisits != 0 || st.WindowUniques != 0 {
		t.Fatalf("stats for empty user = %+v", st)
	}
	if st.Timeseries == nil {
		t.Fatal("timeseries should be an empty array, not null")
	}

	// an unauthenticated stats call is rejected
	if rec := do(t, s, "GET", "/v1/stats", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("stats without key = %d, want 401", rec.Code)
	}
}

func TestNoDeleteOnAuthAPI(t *testing.T) {
	s := newTestServer(t)
	_, key := newKeyUser(t, s, "alice")

	rec := do(t, s, "POST", "/v1/links", key, map[string]any{"destination": "https://example.com"})
	var l domain.Link
	decode(t, rec, &l)

	// the delete route is not registered: 405, and the link survives
	if rec := do(t, s, "DELETE", "/v1/links/"+l.Code, key, nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("delete = %d, want 405", rec.Code)
	}
	if rec := do(t, s, "GET", "/v1/links/"+l.Code, key, nil); rec.Code != http.StatusOK {
		t.Fatalf("link should survive: get = %d", rec.Code)
	}
}

func TestTeamLinksOnAuthAPI(t *testing.T) {
	s := newTestServer(t)
	aliceID, aliceKey := newKeyUser(t, s, "alice")
	_, bobKey := newKeyUser(t, s, "bob")

	team := &domain.Team{Name: "growth", CreatedBy: aliceID}
	if err := s.teams.Create(context.Background(), team); err != nil {
		t.Fatal(err)
	}
	if err := s.teams.AddMember(context.Background(), team.ID, aliceID, domain.RoleOwner); err != nil {
		t.Fatal(err)
	}

	// alice (member) creates a team link
	rec := do(t, s, "POST", "/v1/teams/"+itoa(team.ID)+"/links", aliceKey, map[string]any{"destination": "https://example.com"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create team link = %d, body %s", rec.Code, rec.Body.String())
	}
	// bob (not a member) cannot
	if rec := do(t, s, "POST", "/v1/teams/"+itoa(team.ID)+"/links", bobKey, map[string]any{"destination": "https://x.example.com"}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-member create team link = %d, want 403", rec.Code)
	}
	// alice lists the team's links
	if rec := do(t, s, "GET", "/v1/teams/"+itoa(team.ID)+"/links", aliceKey, nil); rec.Code != http.StatusOK {
		t.Fatalf("list team links = %d", rec.Code)
	}
	// bob cannot even see the team
	if rec := do(t, s, "GET", "/v1/teams/"+itoa(team.ID), bobKey, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider get team = %d, want 404", rec.Code)
	}
}

func TestRateLimit(t *testing.T) {
	s := newTestServer(t)
	_, key := newKeyUser(t, s, "alice")
	s.cfg.keyWriteLimit = 1

	if rec := do(t, s, "POST", "/v1/links", key, map[string]any{"destination": "https://one.example.com"}); rec.Code != http.StatusCreated {
		t.Fatalf("first create = %d", rec.Code)
	}
	rec := do(t, s, "POST", "/v1/links", key, map[string]any{"destination": "https://two.example.com"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second create = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}
	// reads are a separate bucket: still allowed
	if rec := do(t, s, "GET", "/v1/links", key, nil); rec.Code != http.StatusOK {
		t.Fatalf("read after write-limit = %d, want 200", rec.Code)
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
