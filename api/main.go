package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/dbutil"
	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/env"
	"github.com/barats/shrl-io/internal/redisutil"
	"github.com/barats/shrl-io/internal/service"
	"github.com/barats/shrl-io/internal/store"
)

type config struct {
	addr            string
	databaseURL     string
	redisAddr       string
	internalSecret  string
	adminUsername   string
	adminPassword   string
	defaultBaseURL  string
	codeLength      int
	retentionDays   int
	tokenTTL        time.Duration
	warmInterval    time.Duration
}

func loadConfig() config {
	return config{
		addr:            env.Or("SHRL_API_ADDR", ":8080"),
		databaseURL:     env.Or("SHRL_DATABASE_URL", "postgres://shrl:shrl@localhost:5432/shrl"),
		redisAddr:       env.Or("SHRL_REDIS_ADDR", "localhost:6379"),
		internalSecret:  env.Or("SHRL_API_INTERNAL_SECRET", "dev-internal-secret"),
		adminUsername:   env.Or("SHRL_ADMIN_USERNAME", "admin"),
		adminPassword:   os.Getenv("SHRL_ADMIN_PASSWORD"),
		defaultBaseURL:  env.Or("SHRL_DEFAULT_BASE_URL", "http://localhost:8080"),
		codeLength:      env.Int("SHRL_CODE_LENGTH", domain.DefaultCodeLength),
		retentionDays:   env.Int("SHRL_RETENTION_DAYS", 365),
		tokenTTL:        time.Duration(env.Int("SHRL_TOKEN_TTL", 86400)) * time.Second,
		warmInterval:    5 * time.Minute,
	}
}

type server struct {
	links     *store.LinkStore
	analytics *store.AnalyticsStore
	users     *store.UserStore
	baseURLs  *store.BaseURLStore
	teams     *store.TeamStore
	invites   *store.InviteStore
	settings  *store.SettingStore
	linkCache *cache.LinkCache
	linkSvc   *service.LinkService
	cfg       config
}

func main() {
	cfg := loadConfig()
	ctx := context.Background()

	db := dbutil.Open(ctx, dbutil.ConfigFromEnv(cfg.databaseURL))
	links := store.NewLinkStore(db)
	analytics := store.NewAnalyticsStore(db)
	users := store.NewUserStore(db)
	baseURLs := store.NewBaseURLStore(db)
	teams := store.NewTeamStore(db)
	invites := store.NewInviteStore(db)
	settings := store.NewSettingStore(db)
	if err := links.Migrate(ctx); err != nil {
		log.Fatalf("migrate links: %v", err)
	}
	if err := analytics.Migrate(ctx); err != nil {
		log.Fatalf("migrate analytics: %v", err)
	}
	if err := users.Migrate(ctx); err != nil {
		log.Fatalf("migrate users: %v", err)
	}
	if err := baseURLs.Migrate(ctx); err != nil {
		log.Fatalf("migrate base urls: %v", err)
	}
	if err := teams.Migrate(ctx); err != nil {
		log.Fatalf("migrate teams: %v", err)
	}
	if err := invites.Migrate(ctx); err != nil {
		log.Fatalf("migrate invites: %v", err)
	}
	if err := settings.Migrate(ctx); err != nil {
		log.Fatalf("migrate settings: %v", err)
	}
	bootstrapAdmin(ctx, users, cfg)
	bootstrapBaseURLs(ctx, baseURLs, cfg)
	bootstrapSettings(ctx, settings, cfg)

	rdb := redisutil.Connect(ctx, redisutil.ConfigFromEnv(cfg.redisAddr, 0, 2))
	linkCache := cache.NewLinkCache(rdb)
	linkSvc := service.NewLinkService(links, analytics, baseURLs, teams, settings, linkCache, cfg.defaultBaseURL, cfg.retentionDays)
	s := &server{links: links, analytics: analytics, users: users, baseURLs: baseURLs, teams: teams, invites: invites, settings: settings, linkCache: linkCache, linkSvc: linkSvc, cfg: cfg}

	go func() {
		warm(ctx, links, linkCache)
		t := time.NewTicker(cfg.warmInterval)
		defer t.Stop()
		for range t.C {
			warm(ctx, links, linkCache)
		}
	}()

	mux := s.routes()
	log.Printf("api listening on %s", cfg.addr)
	log.Fatal(http.ListenAndServe(cfg.addr, s.internalHeader(s.auth(mux))))
}

// routes registers every API route on a fresh mux.
func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.logout)
	mux.HandleFunc("GET /me", s.me)
	mux.HandleFunc("GET /users", s.listUsers)
	mux.HandleFunc("POST /users", s.createUser)
	mux.HandleFunc("POST /links", s.createLink)
	mux.HandleFunc("GET /links", s.listLinks)
	mux.HandleFunc("GET /base-urls", s.listBaseURLs)
	mux.HandleFunc("POST /base-urls", s.createBaseURL)
	mux.HandleFunc("DELETE /base-urls", s.deleteBaseURL)
	mux.HandleFunc("GET /links/{code}", s.getLink)
	mux.HandleFunc("PATCH /links/{code}", s.updateLink)
	mux.HandleFunc("POST /links/{code}/disable", s.disableLink)
	mux.HandleFunc("POST /links/{code}/enable", s.enableLink)
	mux.HandleFunc("DELETE /links/{code}", s.deleteLink)
	mux.HandleFunc("GET /links/{code}/analytics", s.getAnalytics)
	mux.HandleFunc("GET /links/{code}/analytics/timeseries", s.getAnalyticsTimeseries)
	mux.HandleFunc("GET /links/{code}/analytics/breakdowns", s.getAnalyticsBreakdowns)
	mux.HandleFunc("GET /stats", s.getStats)
	mux.HandleFunc("GET /stats/breakdowns", s.getStatsBreakdowns)
	mux.HandleFunc("GET /stats/top-links", s.getTopLinks)
	mux.HandleFunc("GET /dashboard", s.getDashboard)
	mux.HandleFunc("GET /teams/{id}/stats", s.getTeamStats)
	mux.HandleFunc("GET /teams/{id}/dashboard", s.getTeamDashboard)
	mux.HandleFunc("GET /teams/{id}/stats/breakdowns", s.getTeamStatsBreakdowns)
	mux.HandleFunc("GET /teams/{id}/stats/top-links", s.getTeamTopLinks)
	mux.HandleFunc("POST /teams", s.createTeam)
	mux.HandleFunc("GET /teams", s.listTeams)
	mux.HandleFunc("GET /teams/{id}", s.getTeam)
	mux.HandleFunc("PATCH /teams/{id}", s.updateTeam)
	mux.HandleFunc("GET /teams/{id}/links", s.listTeamLinks)
	mux.HandleFunc("POST /teams/{id}/links", s.createTeamLink)
	mux.HandleFunc("POST /teams/{id}/members", s.addTeamMember)
	mux.HandleFunc("PATCH /teams/{id}/members/{username}", s.setTeamMemberRole)
	mux.HandleFunc("DELETE /teams/{id}/members/{username}", s.removeTeamMember)
	mux.HandleFunc("POST /teams/{id}/invites", s.createInvite)
	mux.HandleFunc("GET /teams/{id}/invites", s.listInvites)
	mux.HandleFunc("DELETE /teams/{id}/invites/{code}", s.revokeInvite)
	mux.HandleFunc("POST /teams/join", s.joinTeam)
	mux.HandleFunc("DELETE /teams/{id}", s.deleteTeam)
	mux.HandleFunc("DELETE /users/{id}", s.deleteUser)
	mux.HandleFunc("POST /users/{id}/reset", s.resetUserPassword)
	mux.HandleFunc("POST /profile/password", s.changePassword)
	mux.HandleFunc("POST /keys", s.createKey)
	mux.HandleFunc("GET /keys", s.listKeys)
	mux.HandleFunc("DELETE /keys/{id}", s.deleteKey)
	mux.HandleFunc("GET /settings", s.getSettings)
	mux.HandleFunc("PATCH /settings/code-length", s.updateCodeLength)
	return mux
}

// bootstrapAdmin provisions the first Admin when the users table is empty and
// backfills any orphaned links to it. The plaintext password is logged once.
func bootstrapAdmin(ctx context.Context, users *store.UserStore, cfg config) {
	n, err := users.Count(ctx)
	if err != nil {
		log.Fatalf("count users: %v", err)
	}
	if n > 0 {
		first, err := users.List(ctx)
		if err == nil && len(first) > 0 {
			if err := users.AssignLinksToCreator(ctx, first[0].ID); err != nil {
				log.Printf("backfill links: %v", err)
			}
		}
		return
	}

	password := cfg.adminPassword
	if password == "" {
		p, err := domain.GeneratePassword(20)
		if err != nil {
			log.Fatalf("generate admin password: %v", err)
		}
		password = p
	}
	hash, err := domain.HashPassword(password)
	if err != nil {
		log.Fatalf("hash admin password: %v", err)
	}
	u := &domain.User{Username: cfg.adminUsername, PasswordHash: hash, IsAdmin: true}
	if err := users.Create(ctx, u); err != nil {
		log.Fatalf("create admin: %v", err)
	}
	if err := users.AssignLinksToCreator(ctx, u.ID); err != nil {
		log.Printf("backfill links: %v", err)
	}
	log.Printf("provisioned admin user %q (password shown only once): %s", u.Username, password)
}

// bootstrapBaseURLs registers the default base URL in the registry so a fresh
// instance always has a selectable base URL.
func bootstrapBaseURLs(ctx context.Context, st *store.BaseURLStore, cfg config) {
	baseURL, err := domain.NormalizeAndValidateBaseURL(cfg.defaultBaseURL)
	if err != nil {
		log.Printf("skip default base URL %q: %v", cfg.defaultBaseURL, err)
		return
	}
	if _, err := st.Get(ctx, baseURL); err == nil {
		return
	}
	if err := st.Create(ctx, &domain.BaseURL{BaseURL: baseURL}); err != nil && !errors.Is(err, store.ErrDuplicatedKey) {
		log.Printf("register default base URL: %v", err)
	}
}

// bootstrapSettings seeds the runtime-configurable settings from env on first
// run; afterwards the database row is authoritative (ADR 0013).
func bootstrapSettings(ctx context.Context, st *store.SettingStore, cfg config) {
	ok, err := st.Has(ctx, domain.CodeLengthSetting)
	if err != nil {
		log.Fatalf("check settings: %v", err)
	}
	if ok {
		return
	}
	if err := st.SetCodeLength(ctx, cfg.codeLength); err != nil {
		log.Fatalf("seed settings: %v", err)
	}
	log.Printf("seeded code length setting: %d", cfg.codeLength)
}

func warm(ctx context.Context, st *store.LinkStore, ca *cache.LinkCache) {
	n, err := ca.Warm(ctx, st)
	if err != nil {
		log.Printf("cache warm failed: %v", err)
		return
	}
	log.Printf("cache warmed with %d active links", n)
}

// writeServiceError maps a LinkService error to an HTTP response.
func (s *server) writeServiceError(w http.ResponseWriter, err error) {
	var ve *service.ValidationError
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "link not found")
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, "insufficient permissions")
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, ve.Msg)
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

// linkJSON renders a Link with only opaque external identifiers (ADR 0021):
// team_id is the team's Ref (null for Personal links) and created_by is the
// creator's username.
func (s *server) linkJSON(r *http.Request, l *domain.Link) map[string]any {
	out := map[string]any{
		"base_url":    l.BaseURL,
		"code":        l.Code,
		"destination": l.Destination,
		"remark":      l.Remark,
		"disabled":    l.Disabled,
		"forward_utm": l.ForwardUTM,
		"created_by":  s.usernameByID(r, l.CreatedBy),
		"team_id":     nil,
		"created_at":  l.CreatedAt,
		"updated_at":  l.UpdatedAt,
	}
	if l.TeamID != nil {
		if t, err := s.teams.Get(r.Context(), *l.TeamID); err == nil {
			out["team_id"] = t.Ref
		}
	}
	return out
}

// writeLinks renders a Link list with ids mapped per linkJSON.
func (s *server) writeLinks(w http.ResponseWriter, r *http.Request, links []domain.Link) {
	items := make([]map[string]any, 0, len(links))
	for i := range links {
		items = append(items, s.linkJSON(r, &links[i]))
	}
	writeJSON(w, http.StatusOK, items)
}

// createLinkInScope validates the create-link request and persists the link,
// scoped to a Team (teamID non-nil) or Personal (teamID nil).
func (s *server) createLinkInScope(w http.ResponseWriter, r *http.Request, teamID *int64) {
	var req struct {
		BaseURL     string `json:"base_url"`
		Destination string `json:"destination"`
		Remark      string `json:"remark"`
		ForwardUTM  bool   `json:"forward_utm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	l, err := s.linkSvc.CreateLink(r.Context(), teamID, currentUser(r).ID, service.CreateLinkInput{
		BaseURL:     req.BaseURL,
		Destination: req.Destination,
		Remark:      req.Remark,
		ForwardUTM:  req.ForwardUTM,
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.linkJSON(r, l))
}

func (s *server) createLink(w http.ResponseWriter, r *http.Request) {
	s.createLinkInScope(w, r, nil)
}

func (s *server) getLink(w http.ResponseWriter, r *http.Request) {
	l, err := s.linkSvc.GetLink(r.Context(), currentUser(r), r.PathValue("code"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.linkJSON(r, l))
}

func (s *server) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.linkSvc.ListLinks(r.Context(), currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list links")
		return
	}
	s.writeLinks(w, r, links)
}

func (s *server) updateLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Destination string `json:"destination"`
		Remark      string `json:"remark"`
		// Pointer so a client that omits forward_utm keeps the current value.
		ForwardUTM *bool `json:"forward_utm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	l, err := s.linkSvc.UpdateLink(r.Context(), currentUser(r), r.PathValue("code"), service.UpdateLinkInput{
		Destination: req.Destination,
		Remark:      req.Remark,
		ForwardUTM:  req.ForwardUTM,
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.linkJSON(r, l))
}

func (s *server) setDisabled(w http.ResponseWriter, r *http.Request, disabled bool) {
	l, err := s.linkSvc.SetDisabled(r.Context(), currentUser(r), r.PathValue("code"), disabled)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.linkJSON(r, l))
}

func (s *server) disableLink(w http.ResponseWriter, r *http.Request) {
	s.setDisabled(w, r, true)
}

func (s *server) enableLink(w http.ResponseWriter, r *http.Request) {
	s.setDisabled(w, r, false)
}

func (s *server) deleteLink(w http.ResponseWriter, r *http.Request) {
	if err := s.linkSvc.DeleteLink(r.Context(), currentUser(r), r.PathValue("code")); err != nil {
		s.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
