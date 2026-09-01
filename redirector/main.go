package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
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

// homeURL is the target of the "Go to shrl.io →" link on every error page.
// The redirector has no landing page of its own, so dead links resolve to the
// project home; that funnel is deliberate (see docs/adr/0020), not a stray
// vendor URL to be stripped.
const homeURL = "https://shrl.io"

const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1285.2 1285.2"><g transform="translate(39.46,175.61) translate(0,933) scale(0.1,-0.1)"><path fill="#01aaa7" d="M3075 9093 c-183 -11 -408 -95 -589 -221 -132 -92 -1600 -1548 -1747 -1734 -189 -238 -327 -516 -424 -853 -31 -108 -34 -119 -44 -175 -6 -30 -16 -84 -23 -120 -18 -89 -18 -568 0 -670 42 -244 81 -388 149 -560 36 -92 111 -246 150 -308 18 -29 33 -55 33 -59 0 -17 173 -249 248 -333 47 -52 210 -223 362 -380 153 -157 324 -334 381 -395 249 -263 539 -551 604 -599 59 -44 171 -106 250 -138 45 -18 146 -39 237 -50 98 -12 263 5 336 34 69 27 155 72 189 99 66 53 1123 1111 1123 1125 0 33 -36 35 -638 33 -683 -2 -681 -2 -815 68 -15 8 -38 20 -51 27 -40 20 -244 223 -600 596 -71 74 -216 223 -321 330 -192 195 -253 271 -298 371 -78 170 -100 277 -101 469 0 199 21 299 100 465 74 156 90 174 772 854 361 359 673 664 694 677 51 32 131 51 252 59 l99 7 233 227 c128 124 437 430 686 679 427 427 452 454 434 467 -17 13 -133 15 -825 13 -443 -1 -828 -3 -856 -5z"/><path fill="#029ace" d="M5765 8290 c-111 -7 -151 -12 -305 -41 -92 -17 -266 -69 -302 -89 -10 -6 -25 -10 -33 -10 -8 0 -23 -4 -33 -10 -9 -5 -57 -27 -107 -47 -474 -200 -942 -628 -1164 -1066 -61 -120 -92 -191 -110 -247 -6 -19 -21 -64 -33 -100 -20 -62 -28 -95 -48 -205 -82 -442 1 -926 215 -1261 19 -30 35 -56 35 -59 0 -7 72 -107 115 -160 103 -126 224 -238 368 -339 86 -61 253 -152 332 -181 28 -10 55 -21 60 -25 6 -3 28 -12 50 -18 22 -7 69 -21 105 -32 169 -52 181 -53 1040 -59 l805 -7 84 -28 c223 -74 362 -218 420 -434 15 -54 16 -254 2 -281 -6 -11 -11 -28 -11 -37 0 -10 -20 -59 -44 -109 -84 -171 -225 -280 -436 -337 -72 -20 -110 -20 -1015 -24 -620 -3 -960 -8 -1000 -15 -124 -23 -222 -65 -325 -142 -75 -55 -1260 -1247 -1260 -1267 0 -41 -45 -40 1752 -40 1924 0 1777 -5 2098 70 354 82 818 381 1079 695 173 209 266 363 358 595 67 167 94 267 134 485 18 96 18 494 0 605 -16 103 -74 334 -102 410 -44 119 -110 254 -166 343 -24 37 -43 70 -43 73 0 9 -144 184 -192 234 -173 179 -369 320 -558 399 -30 13 -68 29 -85 37 -87 38 -223 76 -355 98 -192 31 -331 36 -1042 36 l-716 0 -69 23 c-213 73 -341 246 -330 451 5 98 14 133 57 217 52 102 189 244 300 309 60 35 156 80 172 80 9 0 19 4 22 9 3 5 18 11 33 14 16 3 60 13 98 23 65 17 149 19 1125 24 1020 5 1056 6 1094 25 69 33 148 104 483 431 179 176 334 326 343 334 10 9 156 150 324 315 229 224 306 305 304 320 l-3 20 -1700 1 c-935 0 -1754 -2 -1820 -6z"/><path fill="#0289e8" d="M9648 7696 c-26 -7 -72 -29 -103 -50 -86 -57 -907 -878 -899 -899 5 -15 24 -17 138 -17 251 0 381 -39 522 -156 89 -74 363 -306 469 -398 44 -38 112 -96 150 -129 39 -33 84 -71 100 -86 17 -14 57 -49 90 -76 302 -252 394 -386 440 -640 16 -85 20 -310 7 -319 -5 -3 -395 -6 -868 -7 -472 -1 -865 -4 -871 -6 -15 -6 -17 -26 -4 -51 5 -9 17 -35 26 -57 9 -22 21 -50 26 -62 47 -115 81 -213 110 -318 6 -22 17 -60 24 -85 8 -25 19 -76 25 -115 7 -38 16 -90 20 -115 23 -122 33 -381 21 -532 -14 -180 -45 -390 -71 -490 -6 -23 -16 -61 -22 -83 -21 -77 -81 -260 -90 -270 -4 -5 -8 -15 -8 -22 0 -19 -85 -196 -152 -318 -277 -499 -731 -917 -1218 -1123 -150 -64 -174 -84 -146 -127 8 -13 89 -88 178 -166 90 -79 181 -160 203 -180 22 -20 94 -85 161 -145 487 -439 466 -425 613 -432 118 -6 183 21 321 133 58 47 143 116 190 152 508 399 1170 979 1514 1327 325 329 520 579 712 911 98 169 225 432 254 525 5 14 33 95 63 180 51 147 68 207 117 414 33 137 37 161 75 401 25 157 34 252 50 510 31 505 13 761 -74 1090 -14 51 -60 163 -105 255 -98 199 -245 385 -436 550 -41 36 -77 67 -80 70 -3 3 -32 28 -65 55 -53 44 -178 150 -345 294 -30 26 -104 87 -165 136 -60 49 -158 130 -216 180 -275 233 -302 252 -399 284 -73 24 -206 27 -282 7z m420 -3582 c111 -37 221 -172 239 -294 9 -57 -4 -163 -24 -210 -18 -41 -87 -123 -132 -155 l-42 -30 6 -80 c3 -44 10 -105 15 -135 12 -72 24 -150 55 -350 14 -91 30 -191 35 -224 10 -64 4 -85 -32 -110 -19 -13 -62 -16 -266 -16 l-243 0 -24 26 c-30 32 -30 39 -5 179 11 61 25 142 31 180 26 168 30 193 38 245 5 30 15 91 21 135 6 44 13 92 16 107 4 21 -1 32 -28 55 -75 64 -112 109 -140 171 -28 61 -30 73 -26 161 4 112 23 161 95 241 105 117 257 156 411 104z"/></g></svg>`

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
	mux.HandleFunc("GET /favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		io.WriteString(w, faviconSVG)
	})
	// GET /{code} is the happy path: exactly one path segment with no
	// trailing slash (a trailing slash routes to the catch-all below).
	mux.HandleFunc("GET /{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
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
		cl, ok, err := linkCache.Get(r.Context(), code)
		if err != nil {
			log.Printf("cache lookup error: %v", err)
			writeInternalError(w)
			return
		}
		if !ok {
			// Redis miss means unknown or disabled: the cache warmer plus
			// write-through keep real links present.
			writeNotFound(w)
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
		// The 302 itself keeps the short URL out of the index: crawlers see a
		// redirect and follow it, so no explicit robots header is needed here.
		http.Redirect(w, r, dest, http.StatusFound)
	})
	// GET /{path...} is the catch-all. For a single-segment path that ends in
	// a slash it canonicalizes to /code with a 301 before any rate limiting,
	// so /code and /code/ share one host:code bucket. Everything else — /,
	// multi-segment paths, and every other unmatched path — gets the branded
	// 404 page.
	mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		if strings.HasSuffix(path, "/") {
			if trimmed := strings.TrimSuffix(path, "/"); trimmed != "" && !strings.Contains(trimmed, "/") {
				target := "/" + trimmed
				if r.URL.RawQuery != "" {
					target += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, target, http.StatusMovedPermanently)
				return
			}
		}
		writeNotFound(w)
	})
	return mux
}

func writeRateLimit(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds()) + 1
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	details := "Please slow down and try again shortly."
	if retryAfter > 0 {
		details = fmt.Sprintf("Please wait about %d seconds and try again.", seconds)
	}
	writeErrorPage(w, http.StatusTooManyRequests, errPage{
		Title:       "Too many requests — shrl.io",
		Description: "shrl.io rate limited this request.",
		Status:      "429 Too Many Requests",
		Heading:     "Too many requests",
		Message:     "Too many requests — please try again shortly.",
		Details:     details,
		HomeURL:     homeURL,
	})
}

func writeNotFound(w http.ResponseWriter) {
	writeErrorPage(w, http.StatusNotFound, errPage{
		Title:       "Link not found — shrl.io",
		Description: "This short link does not exist, has been disabled, or has expired.",
		Status:      "404 Not Found",
		Heading:     "This link is not available",
		Message:     "The link you followed does not exist, has been disabled, or has expired.",
		HomeURL:     homeURL,
	})
}

func writeInternalError(w http.ResponseWriter) {
	writeErrorPage(w, http.StatusInternalServerError, errPage{
		Title:       "Something went wrong — shrl.io",
		Description: "shrl.io hit an internal error while resolving this link.",
		Status:      "500 Internal Error",
		Heading:     "Something went wrong",
		Message:     "shrl.io could not resolve this link right now.",
		Details:     "Please try again in a moment.",
		HomeURL:     homeURL,
	})
}

type errPage struct {
	Title       string
	Description string
	Status      string
	Heading     string
	Message     string
	Details     string
	HomeURL     string
}

const errPageTemplate = `<!doctype html>
<html lang="en">
	<head>
		<meta charset="utf-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1" />
		<title>{{.Title}}</title>
		<meta name="description" content="{{.Description}}" />
		<meta name="robots" content="noindex, nofollow" />
		<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
		<style>
			:root { color-scheme: light dark; }
			* { box-sizing: border-box; }
			html, body { height: 100%; }
			body {
				margin: 0;
				display: flex;
				align-items: center;
				justify-content: center;
				padding: 1.25rem;
				font-family: system-ui, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
				background: #fafafa;
				color: #1c1c1e;
			}
			main {
				width: 100%;
				max-width: 30rem;
				background: #ffffff;
				border: 1px solid #e4e4e7;
				border-radius: 0.75rem;
				padding: 2rem 1.5rem;
				text-align: center;
			}
			p { margin: 0; }
			.status {
				font-size: 0.75rem;
				font-weight: 700;
				letter-spacing: 0.08em;
				text-transform: uppercase;
				color: #71717a;
			}
			h1 {
				margin: 0.5rem 0 0.75rem;
				font-size: 1.375rem;
				line-height: 1.3;
			}
			.message { color: #52525b; line-height: 1.6; }
			.details { margin-top: 0.75rem; color: #71717a; line-height: 1.6; }
			.home { margin-top: 1.5rem; }
			a {
				color: #2563eb;
				font-weight: 600;
				text-decoration: none;
			}
			a:hover, a:focus { text-decoration: underline; }
			@media (prefers-color-scheme: dark) {
				body { background: #18181b; color: #fafafa; }
				main { background: #27272a; border-color: #3f3f46; }
				.status { color: #a1a1aa; }
				.message { color: #d4d4d8; }
				.details { color: #a1a1aa; }
			}
		</style>
	</head>
	<body>
		<main>
			<p class="status">{{.Status}}</p>
			<h1>{{.Heading}}</h1>
			<p class="message">{{.Message}}</p>
			{{if .Details}}<p class="details">{{.Details}}</p>{{end}}
			<p class="home"><a href="{{.HomeURL}}">Go to shrl.io →</a></p>
		</main>
	</body>
</html>`

var errorPage = template.Must(template.New("error").Parse(errPageTemplate))

func writeErrorPage(w http.ResponseWriter, status int, p errPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(status)
	if err := errorPage.Execute(w, p); err != nil {
		log.Printf("render error page: %v", err)
	}
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
