package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func seedLink(t *testing.T, rdb *redis.Client, code, destination string) {
	t.Helper()
	if err := cache.NewLinkCache(rdb).Put(context.Background(), &domain.Link{
		BaseURL:     "https://shrl.io",
		Code:        code,
		Destination: destination,
	}); err != nil {
		t.Fatalf("seed link: %v", err)
	}
}

func get(t *testing.T, h http.Handler, host, code, ip string) *httptest.ResponseRecorder {
	t.Helper()
	return getPath(t, h, host, "/"+code, ip)
}

func getPath(t *testing.T, h http.Handler, host, path, ip string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://"+host+path, nil)
	req.RemoteAddr = ip + ":12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestRedirectWithinLimits(t *testing.T) {
	h, rdb := testHandler(t, config{ipLimit: 0, linkLimit: 0})
	seedLink(t, rdb, "abc", "https://example.com/dest")

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
	seedLink(t, rdb, "abc", "https://example.com/a")
	seedLink(t, rdb, "def", "https://example.com/b")

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
	seedLink(t, rdb, "abc", "https://example.com/a")
	seedLink(t, rdb, "def", "https://example.com/b")

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
	seedLink(t, rdb, "abc", "https://example.com/a")

	if rr := get(t, h, "localhost", "nope", "192.0.2.1"); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown code: status = %d, want 404", rr.Code)
	}
}

func TestRedirectHasNoXRobotsTag(t *testing.T) {
	h, rdb := testHandler(t, config{ipLimit: 0, linkLimit: 0})
	seedLink(t, rdb, "abc", "https://example.com/dest")

	rr := get(t, h, "localhost", "abc", "192.0.2.1")
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	// The 302 itself keeps the short URL out of the index; the header would
	// be a placebo.
	if got := rr.Header().Get("X-Robots-Tag"); got != "" {
		t.Fatalf("X-Robots-Tag = %q, want empty", got)
	}
}

func TestNotFoundPage(t *testing.T) {
	h, _ := testHandler(t, config{ipLimit: 0, linkLimit: 0})

	rr := get(t, h, "localhost", "nope", "192.0.2.1")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
	if got := rr.Header().Get("X-Robots-Tag"); got != "noindex" {
		t.Fatalf("X-Robots-Tag = %q, want noindex", got)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"<title>Link not found — shrl.io</title>",
		"<meta name=\"description\"",
		"<meta name=\"robots\" content=\"noindex, nofollow\" />",
		"<link rel=\"icon\" type=\"image/svg+xml\" href=\"/favicon.svg\" />",
		"This link is not available",
		"Go to shrl.io",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("404 body missing %q", want)
		}
	}
	for _, absent := range []string{
		"http-equiv=\"refresh\"",
		"You will be redirected",
		"30 seconds",
	} {
		if strings.Contains(body, absent) {
			t.Errorf("404 body should not contain %q", absent)
		}
	}
}

func TestTrailingSlashRedirectsToCanonical(t *testing.T) {
	h, rdb := testHandler(t, config{ipLimit: 0, linkLimit: 0})
	seedLink(t, rdb, "abc", "https://example.com/dest")

	rr := getPath(t, h, "localhost", "/abc/?utm_source=news&utm_medium=email", "192.0.2.1")
	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/abc?utm_source=news&utm_medium=email" {
		t.Fatalf("Location = %q, want canonical path with query preserved", loc)
	}
	// the canonical path still serves the destination
	rr = get(t, h, "localhost", "abc", "192.0.2.1")
	if rr.Code != http.StatusFound {
		t.Fatalf("canonical status = %d, want 302", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "https://example.com/dest" {
		t.Fatalf("canonical Location = %q, want the destination", loc)
	}
}

func TestRootAndMultiSegmentGetBranded404(t *testing.T) {
	h, _ := testHandler(t, config{ipLimit: 0, linkLimit: 0})

	for _, path := range []string{"/", "/a/b", "/favicon.ico"} {
		rr := getPath(t, h, "localhost", path, "192.0.2.1")
		if rr.Code != http.StatusNotFound {
			t.Fatalf("path %q: status = %d, want 404", path, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "Go to shrl.io") {
			t.Errorf("path %q: 404 body missing the branded link", path)
		}
	}
}

func TestTrailingSlashSharesRateLimitBucket(t *testing.T) {
	h, rdb := testHandler(t, config{ipLimit: 0, linkLimit: 2})
	seedLink(t, rdb, "abc", "https://example.com/a")

	for i := 0; i < 2; i++ {
		if rr := get(t, h, "localhost", "abc", "192.0.2.1"); rr.Code != http.StatusFound {
			t.Fatalf("request %d: status = %d, want 302", i, rr.Code)
		}
	}
	// The link bucket is exhausted, but /abc/ still canonicalizes with a 301
	// before the gate: it never serves the destination, so there is no second
	// bucket to bypass the link limit through.
	rr := getPath(t, h, "localhost", "/abc/", "192.0.2.1")
	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("trailing-slash variant while limited: status = %d, want 301", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/abc" {
		t.Fatalf("Location = %q, want /abc", loc)
	}
	// the canonical path itself stays limited
	if rr := get(t, h, "localhost", "abc", "192.0.2.1"); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("canonical path after limit: status = %d, want 429", rr.Code)
	}
}

func TestFaviconSVG(t *testing.T) {
	h, _ := testHandler(t, config{ipLimit: 0, linkLimit: 0})

	rr := getPath(t, h, "localhost", "/favicon.svg", "192.0.2.1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Fatalf("Content-Type = %q, want image/svg+xml", ct)
	}
	if !strings.Contains(rr.Body.String(), "<svg") {
		t.Error("favicon body is not an SVG")
	}
}

func TestRateLimitPage(t *testing.T) {
	h, rdb := testHandler(t, config{ipLimit: 1, linkLimit: 0})
	seedLink(t, rdb, "abc", "https://example.com/a")

	if rr := get(t, h, "localhost", "abc", "192.0.2.1"); rr.Code != http.StatusFound {
		t.Fatalf("first request: status = %d, want 302", rr.Code)
	}
	rr := get(t, h, "localhost", "abc", "192.0.2.1")
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want 429", rr.Code)
	}
	if rr.Header().Get("X-Robots-Tag") != "noindex" {
		t.Errorf("429 X-Robots-Tag = %q, want noindex", rr.Header().Get("X-Robots-Tag"))
	}
	body := rr.Body.String()
	for _, want := range []string{
		"<title>Too many requests — shrl.io</title>",
		"Too many requests",
		"Go to shrl.io",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("429 body missing %q", want)
		}
	}
}

func TestInternalErrorPage(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	h := newHandler(rdb, config{ipLimit: 0, linkLimit: 0})
	mr.Close() // drop Redis so the cache read fails

	req := httptest.NewRequest(http.MethodGet, "http://localhost/nope", nil)
	req.RemoteAddr = "192.0.2.1:12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	if rr.Header().Get("X-Robots-Tag") != "noindex" {
		t.Errorf("500 X-Robots-Tag = %q, want noindex", rr.Header().Get("X-Robots-Tag"))
	}
	body := rr.Body.String()
	for _, want := range []string{
		"<title>Something went wrong — shrl.io</title>",
		"Something went wrong",
		"Go to shrl.io",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("500 body missing %q", want)
		}
	}
}
