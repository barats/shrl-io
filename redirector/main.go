package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/barats/shrl-io/internal/analytics"
	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/env"
	"github.com/barats/shrl-io/internal/ratelimit"
	"github.com/barats/shrl-io/internal/redisutil"
)

type config struct {
	addr      string
	redisAddr string
	ipLimit   int
	linkLimit int
}

func loadConfig() config {
	return config{
		addr:      env.Or("SHRL_REDIRECTOR_ADDR", ":8080"),
		redisAddr: env.Or("SHRL_REDIS_ADDR", "localhost:6379"),
		ipLimit:   env.Int("SHRL_REDIRECTOR_RATE_LIMIT_IP", 600),
		linkLimit: env.Int("SHRL_REDIRECTOR_RATE_LIMIT_LINK", 3000),
	}
}

func main() {
	cfg := loadConfig()
	ctx := context.Background()
	rdb := redisutil.Connect(ctx, redisutil.ConfigFromEnv(cfg.redisAddr, 50, 5))
	log.Printf("redirector listening on %s", cfg.addr)
	log.Fatal(http.ListenAndServe(cfg.addr, newHandler(rdb, cfg)))
}

func newHandler(rdb *redis.Client, cfg config) http.Handler {
	linkCache := cache.NewLinkCache(rdb)
	analyticsCache := cache.NewAnalyticsCache(rdb)
	rl := ratelimit.New(rdb)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if code == "" {
			http.NotFound(w, r)
			return
		}
		// Rate-limit gates run before any cache read: per-IP protects the
		// redirector, per-Link protects the Destination and the Visit stream.
		// A rejected request is neither redirected nor recorded as a Visit.
		if ok, retry := rl.Allow(r.Context(), "ip:"+clientIP(r), cfg.ipLimit, time.Minute); !ok {
			writeRateLimit(w, retry)
			return
		}
		host := hostOnly(r.Host)
		if ok, retry := rl.Allow(r.Context(), "link:"+host+":"+code, cfg.linkLimit, time.Minute); !ok {
			writeRateLimit(w, retry)
			return
		}
		cl, ok, err := linkCache.Get(r.Context(), host, code)
		if err != nil {
			log.Printf("cache lookup error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			// Redis miss means unknown or disabled: the cache warmer plus
			// write-through keep real links present.
			http.NotFound(w, r)
			return
		}
		// Detached context: r.Context() is cancelled when this handler returns.
		utm := analytics.UTMValues(r.URL.Query())
		go analyticsCache.RecordVisit(context.Background(), host, code, clientIP(r), r.UserAgent(), r.Referer(), utm)
		dest := cl.Destination
		if cl.ForwardUTM {
			if merged, err := analytics.MergeUTMIntoDestination(dest, utm); err == nil {
				dest = merged
			}
		}
		http.Redirect(w, r, dest, http.StatusFound)
	})
	return mux
}

func writeRateLimit(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
	}
	http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
}

func hostOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
