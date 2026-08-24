package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/dbutil"
	"github.com/barats/shrl-io/internal/domain"
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
	retentionDays   int
	tokenTTL        time.Duration
	warmInterval    time.Duration
}

func loadConfig() config {
	return config{
		addr:            envOr("SHRL_API_ADDR", ":8080"),
		databaseURL:     envOr("SHRL_DATABASE_URL", "postgres://shrl:shrl@localhost:5432/shrl"),
		redisAddr:       envOr("SHRL_REDIS_ADDR", "localhost:6379"),
		adminUsername:   envOr("SHRL_ADMIN_USERNAME", "admin"),
		adminPassword:   os.Getenv("SHRL_ADMIN_PASSWORD"),
		defaultHostname: envOr("SHRL_DEFAULT_HOSTNAME", "localhost"),
		retentionDays:   envInt("SHRL_RETENTION_DAYS", 365),
		tokenTTL:        time.Duration(envInt("SHRL_TOKEN_TTL", 86400)) * time.Second,
		warmInterval:    5 * time.Minute,
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

type server struct {
	links     *store.LinkStore
	analytics *store.AnalyticsStore
	users     *store.UserStore
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
	if err := links.Migrate(ctx); err != nil {
		log.Fatalf("migrate links: %v", err)
	}
	if err := analytics.Migrate(ctx); err != nil {
		log.Fatalf("migrate analytics: %v", err)
	}
	if err := users.Migrate(ctx); err != nil {
		log.Fatalf("migrate users: %v", err)
	}
	bootstrapAdmin(ctx, users, cfg)

	rdb := redisutil.Connect(ctx, redisutil.ConfigFromEnv(cfg.redisAddr, 0, 2))
	linkCache := cache.NewLinkCache(rdb)
	s := &server{links: links, analytics: analytics, users: users, linkCache: linkCache, cfg: cfg}

	go func() {
		warm(ctx, links, linkCache)
		t := time.NewTicker(cfg.warmInterval)
		defer t.Stop()
		for range t.C {
			warm(ctx, links, linkCache)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.logout)
	mux.HandleFunc("GET /me", s.me)
	mux.HandleFunc("GET /users", s.listUsers)
	mux.HandleFunc("POST /users", s.createUser)
	mux.HandleFunc("POST /links", s.createLink)
	mux.HandleFunc("GET /links", s.listLinks)
	mux.HandleFunc("GET /hostnames", s.listHostnames)
	mux.HandleFunc("GET /links/{code}", s.getLink)
	mux.HandleFunc("PATCH /links/{code}", s.updateLink)
	mux.HandleFunc("POST /links/{code}/disable", s.disableLink)
	mux.HandleFunc("POST /links/{code}/enable", s.enableLink)
	mux.HandleFunc("DELETE /links/{code}", s.deleteLink)
	mux.HandleFunc("GET /links/{code}/analytics", s.getAnalytics)
	mux.HandleFunc("GET /links/{code}/analytics/timeseries", s.getAnalyticsTimeseries)
	mux.HandleFunc("GET /links/{code}/analytics/breakdowns", s.getAnalyticsBreakdowns)

	log.Printf("api listening on %s", cfg.addr)
	log.Fatal(http.ListenAndServe(cfg.addr, s.auth(mux)))
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

// ownedLink loads a link and enforces that it belongs to the current user.
func (s *server) ownedLink(w http.ResponseWriter, r *http.Request, code string) (*domain.Link, bool) {
	l, err := s.links.Get(r.Context(), s.hostname(r), code)
	if err != nil {
		writeStoreError(w, err)
		return nil, false
	}
	if l.CreatedBy != currentUser(r).ID {
		writeError(w, http.StatusNotFound, "link not found")
		return nil, false
	}
	return l, true
}

func (s *server) createLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname    string `json:"hostname"`
		Code        string `json:"code"`
		Destination string `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	hostname := req.Hostname
	if hostname == "" {
		hostname = s.cfg.defaultHostname
	}
	dest, err := domain.NormalizeAndValidateDestination(req.Destination)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	creator := currentUser(r).ID

	if req.Code == "" {
		for attempt := 0; attempt < 8; attempt++ {
			code, err := domain.GenerateCode()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "code generation failed")
				return
			}
			l := &domain.Link{Hostname: hostname, Code: code, Destination: dest, CreatedBy: creator}
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
		return
	}

	if err := domain.ValidateCustomCode(req.Code); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	l := &domain.Link{Hostname: hostname, Code: req.Code, Destination: dest, CreatedBy: creator}
	if err := s.links.Create(r.Context(), l); err != nil {
		if errors.Is(err, store.ErrDuplicatedKey) {
			writeError(w, http.StatusConflict, "code already exists on this hostname")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to create link")
		}
		return
	}
	s.linkCache.Put(r.Context(), l)
	writeJSON(w, http.StatusCreated, l)
}

func (s *server) getLink(w http.ResponseWriter, r *http.Request) {
	l, ok := s.ownedLink(w, r, r.PathValue("code"))
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

func (s *server) listHostnames(w http.ResponseWriter, r *http.Request) {
	hostnames, err := s.links.ListHostnames(r.Context(), currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list hostnames")
		return
	}
	if hostnames == nil {
		hostnames = []string{}
	}
	writeJSON(w, http.StatusOK, hostnames)
}

func (s *server) updateLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Destination string `json:"destination"`
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
	l, ok := s.ownedLink(w, r, r.PathValue("code"))
	if !ok {
		return
	}
	l.Destination = dest
	if err := s.links.Save(r.Context(), l); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update link")
		return
	}
	s.linkCache.Put(r.Context(), l)
	writeJSON(w, http.StatusOK, l)
}

func (s *server) setDisabled(w http.ResponseWriter, r *http.Request, disabled bool) {
	l, ok := s.ownedLink(w, r, r.PathValue("code"))
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
	if _, ok := s.ownedLink(w, r, code); !ok {
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
