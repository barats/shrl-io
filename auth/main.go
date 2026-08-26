// The auth service is the public API for programmatic access (ADR 0016):
// /v1 endpoints authenticated by an API Key on every request, rate-limited
// from Redis, validating keys from Postgres (ADR 0017). Link mutations go
// through the shared LinkService so the redirect cache is written exactly as
// the Internal API writes it.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/dbutil"
	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/env"
	"github.com/barats/shrl-io/internal/ratelimit"
	"github.com/barats/shrl-io/internal/redisutil"
	"github.com/barats/shrl-io/internal/service"
	"github.com/barats/shrl-io/internal/store"
)

type config struct {
	addr            string
	databaseURL     string
	redisAddr       string
	defaultHostname string
	retentionDays   int
	ipLimit         int
	keyReadLimit    int
	keyWriteLimit   int
	failLimit       int
	rateWindow      time.Duration
}

func loadConfig() config {
	return config{
		addr:            env.Or("SHRL_AUTH_ADDR", ":8080"),
		databaseURL:     env.Or("SHRL_DATABASE_URL", "postgres://shrl:shrl@localhost:5432/shrl"),
		redisAddr:       env.Or("SHRL_REDIS_ADDR", "localhost:6379"),
		defaultHostname: env.Or("SHRL_DEFAULT_HOSTNAME", "localhost"),
		retentionDays:   env.Int("SHRL_RETENTION_DAYS", 365),
		ipLimit:         env.Int("SHRL_AUTH_RATE_LIMIT_IP", 60),
		keyReadLimit:    env.Int("SHRL_AUTH_RATE_LIMIT_KEY_READ", 300),
		keyWriteLimit:   env.Int("SHRL_AUTH_RATE_LIMIT_KEY_WRITE", 30),
		failLimit:       env.Int("SHRL_AUTH_RATE_LIMIT_FAIL", 10),
		rateWindow:      time.Minute,
	}
}

type ctxKey int

const userKey ctxKey = 0

func currentUser(r *http.Request) *domain.User {
	u, _ := r.Context().Value(userKey).(*domain.User)
	return u
}

type server struct {
	users *store.UserStore
	teams *store.TeamStore
	svc   *service.LinkService
	rl    *ratelimit.Limiter
	cfg   config
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
	settings := store.NewSettingStore(db)
	// Migrations are idempotent; running them here means auth can start
	// alongside the api without depending on its start order. Admin, hostname,
	// and setting bootstrap stays the api's job.
	for _, migrate := range []func(context.Context) error{
		links.Migrate, analytics.Migrate, users.Migrate,
		hostnames.Migrate, teams.Migrate, settings.Migrate,
	} {
		if err := migrate(ctx); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	}

	rdb := redisutil.Connect(ctx, redisutil.ConfigFromEnv(cfg.redisAddr, 0, 2))
	linkCache := cache.NewLinkCache(rdb)
	svc := service.NewLinkService(links, analytics, hostnames, teams, settings, linkCache, cfg.defaultHostname, cfg.retentionDays)
	s := &server{users: users, teams: teams, svc: svc, rl: ratelimit.New(rdb), cfg: cfg}

	mux := s.routes()
	log.Printf("auth api listening on %s", cfg.addr)
	log.Fatal(http.ListenAndServe(cfg.addr, s.guard(mux)))
}

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/hostnames", s.listHostnames)
	mux.HandleFunc("GET /v1/teams", s.listTeams)
	mux.HandleFunc("GET /v1/teams/{id}", s.getTeam)
	mux.HandleFunc("GET /v1/teams/{id}/links", s.listTeamLinks)
	mux.HandleFunc("POST /v1/teams/{id}/links", s.createTeamLink)
	mux.HandleFunc("POST /v1/links", s.createLink)
	mux.HandleFunc("GET /v1/links", s.listLinks)
	mux.HandleFunc("GET /v1/links/{code}", s.getLink)
	mux.HandleFunc("PATCH /v1/links/{code}", s.updateLink)
	mux.HandleFunc("POST /v1/links/{code}/disable", s.disableLink)
	mux.HandleFunc("POST /v1/links/{code}/enable", s.enableLink)
	mux.HandleFunc("GET /v1/links/{code}/analytics", s.getAnalytics)
	mux.HandleFunc("GET /v1/links/{code}/analytics/timeseries", s.getAnalyticsTimeseries)
	mux.HandleFunc("GET /v1/links/{code}/analytics/breakdowns", s.getAnalyticsBreakdowns)
	return mux
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// clientIP resolves the caller's IP, honoring X-Forwarded-For when present so
// rate-limit buckets survive a reverse proxy.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if ip := net.ParseIP(strings.TrimSpace(xff)); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// authenticateKey resolves an API Key to its owner via the SHA-256 hash
// lookup; Postgres is the single source of truth (ADR 0017).
func (s *server) authenticateKey(ctx context.Context, key string) (*domain.User, error) {
	k, err := s.users.KeyByHash(ctx, domain.HashToken(key))
	if err != nil {
		return nil, err
	}
	return s.users.GetByID(ctx, k.UserID)
}

// guard rate-limits and authenticates every request by API key. GETs count
// against the read bucket, POST/PATCH against the write bucket; failed
// authentication counts against a per-IP fail bucket.
func (s *server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ok, retry := s.rl.Allow(r.Context(), "ip:"+ip, s.cfg.ipLimit, s.cfg.rateWindow); !ok {
			writeRateLimit(w, retry)
			return
		}
		key := bearerToken(r)
		if key == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		bucket := "key:w"
		limit := s.cfg.keyWriteLimit
		if r.Method == http.MethodGet {
			bucket = "key:r"
			limit = s.cfg.keyReadLimit
		}
		if ok, retry := s.rl.Allow(r.Context(), bucket+":"+domain.HashToken(key), limit, s.cfg.rateWindow); !ok {
			writeRateLimit(w, retry)
			return
		}
		u, err := s.authenticateKey(r.Context(), key)
		if err != nil {
			if ok, retry := s.rl.Allow(r.Context(), "fail:"+ip, s.cfg.failLimit, s.cfg.rateWindow); !ok {
				writeRateLimit(w, retry)
				return
			}
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

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
	l, err := s.svc.CreateLink(r.Context(), teamID, currentUser(r).ID, service.CreateLinkInput{
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

func (s *server) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.svc.ListLinks(r.Context(), currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list links")
		return
	}
	if links == nil {
		links = []domain.Link{}
	}
	writeJSON(w, http.StatusOK, links)
}

func (s *server) getLink(w http.ResponseWriter, r *http.Request) {
	l, err := s.svc.GetLink(r.Context(), currentUser(r), r.PathValue("code"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *server) updateLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Destination string `json:"destination"`
		Remark      string `json:"remark"`
		ForwardUTM  *bool  `json:"forward_utm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	l, err := s.svc.UpdateLink(r.Context(), currentUser(r), r.PathValue("code"), service.UpdateLinkInput{
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
	l, err := s.svc.SetDisabled(r.Context(), currentUser(r), r.PathValue("code"), disabled)
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

func (s *server) listTeams(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.svc.ListTeamSummaries(r.Context(), currentUser(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}
	items := make([]map[string]any, 0, len(summaries))
	for _, ts := range summaries {
		items = append(items, map[string]any{
			"id":         ts.Team.ID,
			"name":       ts.Team.Name,
			"created_by": ts.Team.CreatedBy,
			"created_at": ts.Team.CreatedAt,
			"role":       ts.Role,
		})
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *server) getTeam(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	t, err := s.svc.GetTeam(r.Context(), currentUser(r), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         t.ID,
		"name":       t.Name,
		"created_by": t.CreatedBy,
		"created_at": t.CreatedAt,
	})
}

func (s *server) listTeamLinks(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	if _, err := s.svc.GetTeam(r.Context(), currentUser(r), id); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load team")
		return
	}
	links, err := s.svc.ListTeamLinks(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list links")
		return
	}
	if links == nil {
		links = []domain.Link{}
	}
	writeJSON(w, http.StatusOK, links)
}

func (s *server) createTeamLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}
	if !s.svc.TeamMember(r.Context(), currentUser(r), id) {
		writeError(w, http.StatusForbidden, "not a team member")
		return
	}
	s.createLinkInScope(w, r, &id)
}

func (s *server) listHostnames(w http.ResponseWriter, r *http.Request) {
	names, err := s.svc.ListHostnames(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list hostnames")
		return
	}
	writeJSON(w, http.StatusOK, names)
}

func (s *server) getAnalytics(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	from, to := s.svc.AnalyticsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
	a, err := s.svc.GetAnalytics(r.Context(), currentUser(r), code, from, to)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":        a.Code,
		"window_days": a.RetentionDays,
		"lifetime":    map[string]int64{"visits": a.LifetimeVisits},
		"window":      map[string]int64{"visits": a.WindowVisits, "unique_visitors": a.WindowUniques},
	})
}

func (s *server) getAnalyticsTimeseries(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	from, to := s.svc.AnalyticsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())
	rows, err := s.svc.GetTimeseries(r.Context(), currentUser(r), code, from, to)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *server) getAnalyticsBreakdowns(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	dimension := r.URL.Query().Get("dimension")
	if dimension == "" {
		dimension = "referrer"
	}
	from, to := s.svc.AnalyticsWindow(r.URL.Query().Get("from"), r.URL.Query().Get("to"), time.Now().UTC())

	// limit defaults to 10; 0 returns every distinct value.
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 10000 {
			limit = n
		}
	}

	b, err := s.svc.GetBreakdowns(r.Context(), currentUser(r), code, dimension, from, to, limit)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	items := make([]map[string]int64, 0, len(b.Items))
	for _, item := range b.Items {
		items = append(items, map[string]int64{item.Value: item.Total})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"dimension": b.Dimension,
		"total":     b.Total,
		"items":     items,
		"other":     b.Other,
	})
}

func writeRateLimit(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
	}
	writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
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
