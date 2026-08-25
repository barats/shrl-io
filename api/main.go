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
	"github.com/barats/shrl-io/internal/store"
)

type config struct {
	addr            string
	databaseURL     string
	redisAddr       string
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
	s := &server{links: links, analytics: analytics, users: users, hostnames: hostnames, teams: teams, invites: invites, settings: settings, linkCache: linkCache, cfg: cfg}

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
	log.Fatal(http.ListenAndServe(cfg.addr, s.auth(mux)))
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

func (s *server) hostname(r *http.Request) string {
	if h := r.URL.Query().Get("hostname"); h != "" {
		return h
	}
	return s.cfg.defaultHostname
}

// canReadLink reports whether the current user may see a link. Personal links
// are visible only to their creator. Team links are visible to any member of
// the team and to admins (as instance oversight); a creator who left the team
// is an outsider and loses access.
func (s *server) canReadLink(r *http.Request, l *domain.Link) bool {
	u := currentUser(r)
	if u == nil {
		return false
	}
	if l.TeamID == nil {
		return l.CreatedBy == u.ID
	}
	if u.IsAdmin {
		return true
	}
	_, err := s.teams.MemberRole(r.Context(), *l.TeamID, u.ID)
	return err == nil
}

// canManageLink reports whether the current user may edit, disable, or delete
// a link: its creator (while a member of its team), or a Team Owner of its
// team. Admins manage team links only through an actual Team Owner role.
func (s *server) canManageLink(r *http.Request, l *domain.Link) bool {
	u := currentUser(r)
	if u == nil {
		return false
	}
	if l.TeamID == nil {
		return l.CreatedBy == u.ID
	}
	role, err := s.teams.MemberRole(r.Context(), *l.TeamID, u.ID)
	if err != nil {
		return false
	}
	return role == domain.RoleOwner || l.CreatedBy == u.ID
}

// accessibleLink loads a link and enforces read access for the current user.
func (s *server) accessibleLink(w http.ResponseWriter, r *http.Request, code string) (*domain.Link, bool) {
	l, err := s.links.Get(r.Context(), s.hostname(r), code)
	if err != nil {
		writeStoreError(w, err)
		return nil, false
	}
	if !s.canReadLink(r, l) {
		writeError(w, http.StatusNotFound, "link not found")
		return nil, false
	}
	return l, true
}

// manageableLink loads a link and enforces manage access for the current
// user. A user who can read but not manage gets 403; a user with no access
// gets 404 so link existence is not leaked.
func (s *server) manageableLink(w http.ResponseWriter, r *http.Request, code string) (*domain.Link, bool) {
	l, err := s.links.Get(r.Context(), s.hostname(r), code)
	if err != nil {
		writeStoreError(w, err)
		return nil, false
	}
	if s.canManageLink(r, l) {
		return l, true
	}
	if s.canReadLink(r, l) {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return nil, false
	}
	writeError(w, http.StatusNotFound, "link not found")
	return nil, false
}

// createLinkInScope validates the create-link request and persists the link,
// scoped to a Team (teamID non-nil) or Personal (teamID nil).
func (s *server) createLinkInScope(w http.ResponseWriter, r *http.Request, teamID *int64, creatorID int64) {
	var req struct {
		Hostname    string `json:"hostname"`
		Destination string `json:"destination"`
		Remark      string `json:"remark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	hostname := req.Hostname
	if hostname == "" {
		hostname = s.cfg.defaultHostname
	}
	hostname, err := domain.NormalizeAndValidateHostname(hostname)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.hostnames.Get(r.Context(), hostname); err != nil {
		writeError(w, http.StatusBadRequest, "hostname is not registered")
		return
	}
	dest, err := domain.NormalizeAndValidateDestination(req.Destination)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	remark, err := domain.NormalizeRemark(req.Remark)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	codeLength, err := s.settings.CodeLength(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read settings")
		return
	}

	for attempt := 0; attempt < 8; attempt++ {
		code, err := domain.GenerateCode(codeLength)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "code generation failed")
			return
		}
		l := &domain.Link{Hostname: hostname, Code: code, Destination: dest, Remark: remark, CreatedBy: creatorID, TeamID: teamID}
		if err := s.links.Create(r.Context(), l); err == nil {
			s.linkCache.Put(r.Context(), l)
			writeJSON(w, http.StatusCreated, l)
			return
		} else if errors.Is(err, store.ErrDuplicatedKey) {
			continue // auto codes never reuse an existing code
		} else {
			writeError(w, http.StatusInternalServerError, "failed to create link")
			return
		}
	}
	writeError(w, http.StatusInternalServerError, "could not allocate a unique code")
}

func (s *server) createLink(w http.ResponseWriter, r *http.Request) {
	s.createLinkInScope(w, r, nil, currentUser(r).ID)
}

func (s *server) getLink(w http.ResponseWriter, r *http.Request) {
	l, ok := s.accessibleLink(w, r, r.PathValue("code"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *server) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.links.List(r.Context(), s.hostname(r), currentUser(r).ID)
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
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	dest, err := domain.NormalizeAndValidateDestination(req.Destination)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	remark, err := domain.NormalizeRemark(req.Remark)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	l, ok := s.manageableLink(w, r, r.PathValue("code"))
	if !ok {
		return
	}
	l.Destination = dest
	l.Remark = remark
	if err := s.links.Save(r.Context(), l); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update link")
		return
	}
	s.linkCache.Put(r.Context(), l)
	writeJSON(w, http.StatusOK, l)
}

func (s *server) setDisabled(w http.ResponseWriter, r *http.Request, disabled bool) {
	l, ok := s.manageableLink(w, r, r.PathValue("code"))
	if !ok {
		return
	}
	l.Disabled = disabled
	if err := s.links.Save(r.Context(), l); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update link")
		return
	}
	// Put handles both directions: active links are cached, disabled ones are
	// evicted so the redirector 404s them.
	s.linkCache.Put(r.Context(), l)
	writeJSON(w, http.StatusOK, l)
}

func (s *server) disableLink(w http.ResponseWriter, r *http.Request) {
	s.setDisabled(w, r, true)
}

func (s *server) enableLink(w http.ResponseWriter, r *http.Request) {
	s.setDisabled(w, r, false)
}

func (s *server) deleteLink(w http.ResponseWriter, r *http.Request) {
	hostname := s.hostname(r)
	code := r.PathValue("code")
	if _, ok := s.manageableLink(w, r, code); !ok {
		return
	}
	if err := s.links.Delete(r.Context(), hostname, code); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete link")
		return
	}
	s.linkCache.Delete(r.Context(), hostname, code)
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
