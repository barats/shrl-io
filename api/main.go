package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	adminKey        string
	defaultHostname string
	retentionDays   int
	warmInterval    time.Duration
}

func loadConfig() config {
	return config{
		addr:            envOr("SHRL_API_ADDR", ":8080"),
		databaseURL:     envOr("SHRL_DATABASE_URL", "postgres://shrl:shrl@localhost:5432/shrl"),
		redisAddr:       envOr("SHRL_REDIS_ADDR", "localhost:6379"),
		adminKey:        os.Getenv("SHRL_ADMIN_KEY"),
		defaultHostname: envOr("SHRL_DEFAULT_HOSTNAME", "localhost"),
		retentionDays:   envInt("SHRL_RETENTION_DAYS", 365),
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
	linkCache *cache.LinkCache
	cfg       config
}

func main() {
	cfg := loadConfig()
	if cfg.adminKey == "" {
		log.Fatal("SHRL_ADMIN_KEY is required")
	}
	ctx := context.Background()

	db := dbutil.Open(ctx, dbutil.ConfigFromEnv(cfg.databaseURL))
	links := store.NewLinkStore(db)
	analytics := store.NewAnalyticsStore(db)
	if err := links.Migrate(ctx); err != nil {
		log.Fatalf("migrate links: %v", err)
	}
	if err := analytics.Migrate(ctx); err != nil {
		log.Fatalf("migrate analytics: %v", err)
	}

	rdb := redisutil.Connect(ctx, redisutil.ConfigFromEnv(cfg.redisAddr, 0, 2))
	linkCache := cache.NewLinkCache(rdb)
	s := &server{links: links, analytics: analytics, linkCache: linkCache, cfg: cfg}

	go func() {
		warm(ctx, links, linkCache)
		t := time.NewTicker(cfg.warmInterval)
		defer t.Stop()
		for range t.C {
			warm(ctx, links, linkCache)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /links", s.createLink)
	mux.HandleFunc("GET /links", s.listLinks)
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

func warm(ctx context.Context, st *store.LinkStore, ca *cache.LinkCache) {
	n, err := ca.Warm(ctx, st)
	if err != nil {
		log.Printf("cache warm failed: %v", err)
		return
	}
	log.Printf("cache warmed with %d active links", n)
}

func (s *server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		if key == "" || key != s.cfg.adminKey {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) hostname(r *http.Request) string {
	if h := r.URL.Query().Get("hostname"); h != "" {
		return h
	}
	return s.cfg.defaultHostname
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

	if req.Code == "" {
		for attempt := 0; attempt < 8; attempt++ {
			code, err := domain.GenerateCode()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "code generation failed")
				return
			}
			l := &domain.Link{Hostname: hostname, Code: code, Destination: dest}
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
	l := &domain.Link{Hostname: hostname, Code: req.Code, Destination: dest}
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
	l, err := s.links.Get(r.Context(), s.hostname(r), r.PathValue("code"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *server) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.links.List(r.Context(), s.hostname(r))
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
	l, err := s.links.Get(r.Context(), s.hostname(r), r.PathValue("code"))
	if err != nil {
		writeStoreError(w, err)
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
	l, err := s.links.Get(r.Context(), s.hostname(r), r.PathValue("code"))
	if err != nil {
		writeStoreError(w, err)
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
	if _, err := s.links.Get(r.Context(), hostname, code); err != nil {
		writeStoreError(w, err)
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
