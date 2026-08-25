package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/barats/shrl-io/internal/analytics"
	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/env"
	"github.com/barats/shrl-io/internal/redisutil"
)

type config struct {
	addr      string
	redisAddr string
}

func loadConfig() config {
	return config{
		addr:      env.Or("SHRL_REDIRECTOR_ADDR", ":8080"),
		redisAddr: env.Or("SHRL_REDIS_ADDR", "localhost:6379"),
	}
}

func main() {
	cfg := loadConfig()
	ctx := context.Background()
	rdb := redisutil.Connect(ctx, redisutil.ConfigFromEnv(cfg.redisAddr, 50, 5))
	linkCache := cache.NewLinkCache(rdb)
	analyticsCache := cache.NewAnalyticsCache(rdb)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if code == "" {
			http.NotFound(w, r)
			return
		}
		host := hostOnly(r.Host)
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

	log.Printf("redirector listening on %s", cfg.addr)
	log.Fatal(http.ListenAndServe(cfg.addr, mux))
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
