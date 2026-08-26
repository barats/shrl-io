# Redirector rate limiting: per-IP and per-Link Redis sliding-window counters

The redirector is the one public surface with no protection: it is
Redis-only on the hot path (ADR 0001), unauthenticated, and anyone can hit
`GET /{code}`. A runaway client could hammer a single Destination or flood
the Visit stream behind it. It now reuses `internal/ratelimit` — the same
Redis sliding-window limiter the Auth API uses (ADR 0016) — to cap Redirects
per IP and per Link on a 1-minute window: `SHRL_REDIRECTOR_RATE_LIMIT_IP`
(default 600/min) and `SHRL_REDIRECTOR_RATE_LIMIT_LINK` (default 3000/min);
`0` disables a bucket. Both checks run before the cache read, IP first, then
Link. Over-limit requests get `429` with `Retry-After`, are not redirected,
and are not recorded as Visits — a request that never redirected is not a
Visit. The limiter fails open on Redis errors, so rate limiting can never
take the redirector down. Counters live in shared Redis, so limits are
enforced across replicas rather than per process.

Two choices look like bugs to a future reader and are deliberate. The per-IP
counter briefly writes the visitor's IP as a TTL'd Redis key; that is not
"stored" in the ADR 0004 sense — the key expires with the window, is never
written to Postgres, and the persisted analytics identity stays the hash of
`(Link, day, IP + user-agent)`. And there is no user-agent exemption:
link-preview unfurlers (Slack, WhatsApp, iMessage) share a few data-center
IPs, but the generous per-IP default keeps previews working; we special-case
bots only if previews actually break.
