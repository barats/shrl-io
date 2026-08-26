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
	defaultHostname string
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
		defaultHostname: env.Or("SHRL_DEFAULT_HOSTNAME", "localhost"),
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
	hostnames *store.HostnameStore
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
	hostnames := store.NewHostnameStore(db)
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
	if err := hostnames.Migrate(ctx); err != nil {
		log.Fatalf("migrate hostnames: %v", err)
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
	bootstrapHostnames(ctx, hostnames, cfg)
	bootstrapSettings(ctx, settings, cfg)

	rdb := redisutil.Connect(ctx, redisutil.ConfigFromEnv(cfg.redisAddr, 0, 2))
	linkCache := cache.NewLinkCache(rdb)
	linkSvc := service.NewLinkService(links, analytics, hostnames, teams, settings, linkCache, cfg.defaultHostname, cfg.retentionDays)
	s := &server{links: links, analytics: analytics, users: users, hostnames: hostnames, teams: teams, invites: invites, settings: settings, linkCache: linkCache, linkSvc: linkSvc, cfg: cfg}

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
	mux.HandleFunc("GET /hostnames", s.listHostnames)
	mux.HandleFunc("POST /hostnames", s.createHostname)
	mux.HandleFunc("DELETE /hostnames/{hostname}", s.deleteHostname)
	mux.HandleFunc("GET /links/{code}", s.getLink)
	mux.HandleFunc("PATCH /links/{code}", s.updateLink)
	mux.HandleFunc("POST /links/{code}/disable", s.disableLink)
	mux.HandleFunc("POST /links/{code}/enable", s.enableLink)
	mux.HandleFunc("DELETE /links/{code}", s.deleteLink)
	mux.HandleFunc("GET /links/{code}/analytics", s.getAnalytics)
	mux.HandleFunc("GET /links/{code}/analytics/timeseries", s.getAnalyticsTimeseries)
	mux.HandleFunc("GET /links/{code}/analytics/breakdowns", s.getAnalyticsBreakdowns)
	mux.HandleFunc("POST /teams", s.createTeam)
	mux.HandleFunc("GET /teams", s.listTeams)
	mux.HandleFunc("GET /teams/{id}", s.getTeam)
	mux.HandleFunc("GET /teams/{id}/links", s.listTeamLinks)
	mux.HandleFunc("POST /teams/{id}/links", s.createTeamLink)
	mux.HandleFunc("POST /teams/{id}/members", s.addTeamMember)
	mux.HandleFunc("PATCH /teams/{id}/members/{userID}", s.setTeamMemberRole)
	mux.HandleFunc("DELETE /teams/{id}/members/{userID}", s.removeTeamMember)
	mux.HandleFunc("POST /teams/{id}/invites", s.createInvite)
	mux.HandleFunc("GET /teams/{id}/invites", s.listInvites)
	mux.HandleFunc("DELETE /teams/{id}/invites/{code}", s.revokeInvite)
	mux.HandleFunc("POST /teams/join", s.joinTeam)
	mux.HandleFunc("DELETE /teams/{id}", s.deleteTeam)
	mux.HandleFunc("DELETE /users/{id}", s.deleteUser)
	mux.HandleFunc("POST /users/{id}/reset", s.resetUserPassword)
	mux.HandleFunc("POST /account/password", s.changePassword)
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

// bootstrapHostnames registers the default hostname in the registry so a fresh
// instance always has a selectable hostname.
func bootstrapHostnames(ctx context.Context, st *store.HostnameStore, cfg config) {
	name, err := domain.NormalizeAndValidateHostname(cfg.defaultHostname)
	if err != nil {
		log.Printf("skip default hostname %q: %v", cfg.defaultHostname, err)
		return
	}
	if _, err := st.Get(ctx, name); err == nil {
		return
	}
	if err := st.Create(ctx, &domain.Hostname{Name: name}); err != nil && !errors.Is(err, store.ErrDuplicatedKey) {
		log.Printf("register default hostname: %v", err)
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

// createLinkInScope validates the create-link request and persists the link,
// scoped to a Team (teamID non-nil) or Personal (teamID nil).
func (s *server) createLinkInScope(w http.ResponseWriter, r *http.Request, teamID *int64) {
	var req struct {
		Hostname    string `json:"hostname"`
		Destination string `json:"destination"`
		Remark      string `json:"remark"`
		ForwardUTM  bool   `json:"forward_utm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	l, err := s.linkSvc.CreateLink(r.Context(), teamID, currentUser(r).ID, service.CreateLinkInput{
		Hostname:    req.Hostname,
		Destination: req.Destination,
		Remark:      req.Remark,
		ForwardUTM:  req.ForwardUTM,
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
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
	writeJSON(w, http.StatusOK, l)
}

func (s *server) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.linkSvc.ListLinks(r.Context(), currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list links")
		return
	}
	if links == nil {
		links = []domain.Link{}
	}
	writeJSON(w, http.StatusOK, links)
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
	writeJSON(w, http.StatusOK, l)
}

func (s *server) setDisabled(w http.ResponseWriter, r *http.Request, disabled bool) {
	l, err := s.linkSvc.SetDisabled(r.Context(), currentUser(r), r.PathValue("code"), disabled)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
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
