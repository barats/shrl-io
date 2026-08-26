package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/barats/shrl-io/internal/cache"
	"github.com/barats/shrl-io/internal/domain"
)

func testHandler(t *testing.T, cfg config) (http.Handler, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return newHandler(rdb, cfg), rdb
}

func seedLink(t *testing.T, rdb *redis.Client, hostname, code, destination string) {
	t.Helper()
	if err := cache.NewLinkCache(rdb).Put(context.Background(), &domain.Link{
		Hostname:    hostname,
		Code:        code,
		Destination: destination,
	}); err != nil {
		t.Fatalf("seed link: %v", err)
	}
}

func get(t *testing.T, h http.Handler, host, code, ip string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://"+host+"/"+code, nil)
	req.RemoteAddr = ip + ":12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestRedirectWithinLimits(t *testing.T) {
	h, rdb := testHandler(t, config{ipLimit: 0, linkLimit: 0})
	seedLink(t, rdb, "localhost", "abc", "https://example.com/dest")

	rr := get(t, h, "localhost", "abc", "192.0.2.1")
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	if loc := rr.Header().Get("Location"); loc != "https://example.com/dest" {
		t.Fatalf("Location = %q, want the destination", loc)
	}
}

func TestRateLimitByIP(t *testing.T) {
	h, rdb := testHandler(t, config{ipLimit: 3, linkLimit: 0})
	seedLink(t, rdb, "localhost", "abc", "https://example.com/a")
	seedLink(t, rdb, "localhost", "def", "https://example.com/b")

	for i := 0; i < 3; i++ {
		if rr := get(t, h, "localhost", "abc", "192.0.2.1"); rr.Code != http.StatusFound {
			t.Fatalf("request %d: status = %d, want 302", i, rr.Code)
		}
	}
	rr := get(t, h, "localhost", "def", "192.0.2.1")
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request from same IP: status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Error("429 response should include Retry-After")
	}
	// a different IP is unaffected
	if rr := get(t, h, "localhost", "abc", "198.51.100.7"); rr.Code != http.StatusFound {
		t.Fatalf("different IP: status = %d, want 302", rr.Code)
	}
}

func TestRateLimitByLink(t *testing.T) {
	h, rdb := testHandler(t, config{ipLimit: 0, linkLimit: 3})
	seedLink(t, rdb, "localhost", "abc", "https://example.com/a")
	seedLink(t, rdb, "localhost", "def", "https://example.com/b")

	for i := 0; i < 3; i++ {
		if rr := get(t, h, "localhost", "abc", "192.0.2.1"); rr.Code != http.StatusFound {
			t.Fatalf("request %d: status = %d, want 302", i, rr.Code)
		}
	}
	rr := get(t, h, "localhost", "abc", "192.0.2.1")
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request to same link: status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}
	// a different link is unaffected
	if rr := get(t, h, "localhost", "def", "192.0.2.1"); rr.Code != http.StatusFound {
		t.Fatalf("different link: status = %d, want 302", rr.Code)
	}
}

func TestUnknownCodeNotFound(t *testing.T) {
	h, rdb := testHandler(t, config{ipLimit: 0, linkLimit: 0})
	seedLink(t, rdb, "localhost", "abc", "https://example.com/a")

	if rr := get(t, h, "localhost", "nope", "192.0.2.1"); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown code: status = %d, want 404", rr.Code)
	}
}
