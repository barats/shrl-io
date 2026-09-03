```
   ███████╗██╗  ██╗██████╗ ██╗         ██╗ ██████╗
   ██╔════╝██║  ██║██╔══██╗██║         ██║██╔═══██╗
   ███████╗███████║██████╔╝██║         ██║██║   ██║
   ╚════██║██╔══██║██╔══██╗██║         ██║██║   ██║
   ███████║██║  ██║██║  ██║███████╗ ██ ██║╚██████╔╝
   ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝ ╚╝ ╚═╝  ╚═════╝
   https://shrl.io   
```

<div align="center">

[![CI](https://github.com/barats/shrl-io/actions/workflows/ci.yml/badge.svg)](https://github.com/barats/shrl-io/actions/workflows/ci.yml)
[![Release](https://github.com/barats/shrl-io/actions/workflows/release.yml/badge.svg)](https://github.com/barats/shrl-io/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![pre-1.0](https://img.shields.io/badge/status-pre--1.0-orange)](#security)

</div>

---

## Overview

**shrl.io** is a self-hosted URL shortener and traffic analyzer. It turns a
**Destination** URL into a short, shareable URL under your own **Base URL** and
redirects visitors there at sub-millisecond speed, while recording
privacy-first traffic analytics. It is built as a small set of Go
microservices — an internal API, a public auth service, a Redis-backed
redirector, and an analytics worker —
over PostgreSQL and Redis, with a SvelteKit admin UI, and is designed for
teams and individuals who want full control over their link data.

### Why shrl.io?

- 🔒 **Privacy-first**: your data stays on your infrastructure. Visitor IPs are
  never stored — only derived **Locations** and aggregate counts.
- ⚡ **Blazing fast**: Redis-backed redirects with sub-millisecond response
  times; the database never sits on the redirect hot path.
- 📊 **Rich analytics**: visits, unique **Visitors**, daily time series, and
  breakdowns by referrer, device, OS, browser, country, region, city, and the
  six UTM parameters — with optional per-link forwarding to the Destination.
- 🏠 **Simple to self-host**: prebuilt images on ghcr.io and one
  `podman compose up -d` bring up the whole stack; no external accounts
  required (GeoIP attribution is optional).
- 🖥️ **Built-in admin UI**: sign in with your account and manage Links from
  the browser, no curl required.
- 🛡️ **Secure by default**: URL validation and open-redirect protection, plus
  password accounts with bearer-token auth. The internal API is frontend-only
  (never published to the host); scripts and CI use the public Auth API with
  an API key, rate-limited per key and per IP, and the redirector rate-limits
  per IP and per Link.

## Screenshots

<p align="center">
  <a href="https://github.com/barats/shrl-io/raw/main/docs/screenshots/dashboard.png">
    <img src="docs/screenshots/dashboard.png" alt="Dashboard with a 14-day traffic chart, top links, sources, environment, location, and a world map of visitor countries" width="28%">
  </a>
  <a href="https://github.com/barats/shrl-io/raw/main/docs/screenshots/link-detail.png">
    <img src="docs/screenshots/link-detail.png" alt="Link detail with lifetime analytics, per-day chart, breakdowns, campaigns, and QR code" width="28%">
  </a>
</p>

<p align="center"><sub>Click a screenshot to open the full-size PNG.</sub></p>

## Features

### Link management

- **Auto codes**: shrl.io generates every **Code**: lowercase, from an
  unambiguous alphabet (no `0`/`O`/`1`/`l`). Users never choose a Code. A Code
  is globally unique across every Base URL. The exact **Code Length** defaults
  to 6; an Admin can set it per instance (4–12) in Settings.
- **Admin-managed base URLs**: an **Admin** registers **Base URLs**; Users
  select from the registry when creating a Link. A Base URL labels where a Link
  lives and is shown alongside the Code (`base_url/code`) when displaying
  Links, but it is not part of a Link's identity — a Code alone identifies it.
- **Remark**: an optional note on a Link so you can remember what it does;
  editable after creation.
- **QR codes**: every Link's detail page shows a QR code for its short URL,
  generated entirely in the browser and downloadable as a 1024px PNG — no
  server round-trip.
- **Full lifecycle**: create, list, get, update the Destination, disable,
  enable, or delete a Link.
- **Disabled** Links return 404 from the redirector (reversible — the Link and
  its data are preserved). A **Delete**d Link's Code is never automatically
  reused.

### Analytics & tracking

- **Async tracking**: each **Redirect** pushes a **Visit** onto a Redis stream;
  the worker aggregates in batches, keeping the redirect path non-blocking.
- **Read API**: lifetime and window totals, daily time series, and top-N
  breakdowns per dimension.
- **Dimensions**: `referrer`, `device`, `os`, `browser`, `country`, `region`,
  `city`, and the six `utm_*` parameters.
- **Geographic map**: a country map view of visits on the dashboard, team
  dashboard, and each Link's detail page, with a top-countries list (data
  requires GeoIP attribution).
- **Bot filtering**: known crawlers and link-preview unfurlers are excluded at
  aggregation time.
- **Retention pruning**: daily rollups are pruned after `SHRL_RETENTION_DAYS`
  (default 365); the lifetime visit total is never pruned.
- **Optional GeoIP**: set `SHRL_GEOLITE_LICENSE` (a free MaxMind account) to
  attribute country/region/city; without it, locations report as `unknown`.

### Admin UI & users

- **Users**: password-based accounts with bcrypt hashes; the first user is
  an **Admin** provisioned on first run with a random password shown once in
  the api logs (`SHRL_ADMIN_PASSWORD` sets a known one). Admins create
  accounts; there is no self-registration.
- **Login**: sign in with username + password; the UI issues an HttpOnly
  session cookie and proxies API calls server-side with the user's token, so
  the password and token never reach the browser.
- **Change password**: a User replaces their own password on the Profile
  page; doing so revokes every other sign-in and every API key, keeping only
  the current session.
- **API keys**: long-lived, named bearer credentials for the public **Auth
  API** — scripts and CI authenticate with them instead of a session. Created
  and revoked per User on the Profile page, shown once at creation, and
  revoked on a password change.
- **Admin password reset**: an Admin resets a forgotten password to a
  generated temporary one (shown once); the User must change it on their next
  sign-in before using the instance. There is no SMTP-based reset.
- **Link management**: each User sees and manages their own Links, across every
  Base URL: create, edit the Destination, disable/enable, and delete.
- **Analytics view**: lifetime and window totals, a daily visits chart, and
  top-N breakdowns with a dimension picker.

### Teams

- **Shared ownership**: an **Admin** creates Teams and becomes their first
  **Team Owner**. A Link belongs to exactly one Team or is **Personal**; a
  Team's Links are visible to every **Team Member** (read-only) and managed by
  their **Creator** or a **Team Owner**.
- **Invite-code membership**: Team Owners generate single-use, revocable
  **Invite Codes** and share them out of band; a User joins a Team by entering
  the code. Only an Admin adds members directly (by username).
- **Team dashboard**: each Team page lists members, invite codes (for owners),
  and the Team's Links (labeled `base_url/code`); Team Links have their own detail
  page with analytics.

### Settings (admin)

- **Base URLs**: register and remove Base URLs in the Registry.
- **Users**: create and delete accounts; deleting a User removes their
  Personal Links and memberships — Team Links they created stay with the Team.
- **Teams**: create and delete Teams; deleting a Team reverts its Links to
  Personal.

### Security model

- **Open-redirect protection**: only `http`/`https` Destinations are accepted;
  loopback, private, and link-local addresses are rejected at create/update
  time.
- **Account auth**: the **Internal API** is reachable only by the frontend,
  which presents a shared secret header and the user's bearer token (ADR
  0015). The **Auth API** is public and validates an **API Key** on every
  request, rate-limited per key and per IP; keys are checked against
  Postgres, the single source of truth (ADR 0017).
- **Redirect rate limiting**: the redirector caps Redirects per IP and per
  Link on a 1-minute window (`SHRL_REDIRECTOR_RATE_LIMIT_IP`,
  `SHRL_REDIRECTOR_RATE_LIMIT_LINK`; `0` disables a bucket). Excess requests
  get `429` with `Retry-After`, are not redirected, and are not recorded as
  Visits. The counters are transient TTL'd Redis keys, so visitor IPs are
  still never stored (ADR 0004).
- **No raw IPs persisted**: Visitor identity is stored as a hash of
  `(Link, day, IP + user-agent)`; the IP itself is never written.

## Architecture

```
                          ┌───────────────────┐
   user (browser) ───────►│ frontend (Svelte) │   login + session cookie
                          └─────────┬─────────┘
                                    │ proxies with the user's token
                                    ▼
                              ┌──────────────┐
                              │     api      │   Internal API, frontend-only
                              └──────┬───────┘
                                     │ write-through (Link cache)
   script / CI ── API key ──► ┌────────────┐
                              │    auth    │   Auth API /v1, rate-limited
                              └──────┬─────┘
                                     │ write-through (Link cache)
                          ┌──────────┴──────────┐
                          ▼                     ▼
                   ┌──────────────┐     ┌──────────────┐
                   │  PostgreSQL  │     │    Redis     │
                   │ source of    │     │   Link cache │
                   │ truth        │     │ Visit stream │
                   └──────────────┘     └──────┬───────┘
                                              │ cache read
                                              ▼
              visitor ─── GET /{code} ───► ┌──────────────┐
                                           │  redirector  │ ── 302 ──► Destination
                                           └──────┬───────┘
                                                  │ Visit → stream
                                                  ▼
                                           ┌──────────────┐  batch   ┌──────────────┐
                                           │    Redis     │ ────────► │    worker    │
                                           │ Visit stream │          └──────┬───────┘
                                           └──────────────┘                 │ upsert rollups
                                                                            ▼
                                                                     ┌──────────────┐
                                                                     │  PostgreSQL  │
                                                                     └──────────────┘
```

### Data flow

1. **Create**: the api or auth service writes a Link to PostgreSQL, then caches
   it in Redis (write-through). The api also re-warms the cache every 5
   minutes.
2. **Redirect**: the redirector rate-limits per IP and per Link, reads the
   Link from Redis — never PostgreSQL — and returns a 302. It pushes a Visit
   event onto the Redis stream. Requests over a limit get `429` with
   `Retry-After` and are neither redirected nor recorded as Visits.
3. **Aggregate**: the worker consumes the stream in batches and upserts daily,
   lifetime, and breakdown rollups into PostgreSQL in a single transaction;
   stale rollups are pruned after the retention window.

## Production

> [!WARNING]
> **Pre-1.0 software.** Treat deployments as beta: watch releases, pin a
> version instead of `:latest` when surprises are expensive, and read the
> release notes before upgrading. Security fixes land only in the latest
> release — see [Security](#security).

shrl.io publishes one image per service to ghcr.io, built for linux/amd64
and linux/arm64:

| Image                               | Runs                            | Ports |
|-------------------------------------|---------------------------------|-------|
| `ghcr.io/barats/shrl-io-api`        | Internal API (UI-only)          | none, compose-network only |
| `ghcr.io/barats/shrl-io-auth`       | Auth API for API Keys           | 8083  |
| `ghcr.io/barats/shrl-io-redirect`   | Redirector                      | 8080  |
| `ghcr.io/barats/shrl-io-worker`     | Analytics worker                | none  |
| `ghcr.io/barats/shrl-io-frontend`   | UI server                       | 8082  |

The `compose.yaml` in the repo root assembles the full stack: those five
images plus PostgreSQL, Redis, and the GeoIP data volume. Copy it to your
server, export the two required secrets, and start:

    curl -O https://raw.githubusercontent.com/barats/shrl-io/main/compose.yaml
    export SHRL_API_INTERNAL_SECRET="$(openssl rand -hex 32)"
    export SHRL_SESSION_SECRET="$(openssl rand -hex 32)"
    podman compose up -d

Then sign in at http://localhost:8082 with the first-run **admin** account:
`SHRL_ADMIN_PASSWORD` if you exported it, otherwise the generated value
printed once to the api service logs.

The compose file tracks the **latest** images, so
`podman compose pull && podman compose up -d` upgrades to the newest
release. To control when you upgrade, pin a release tag instead (e.g.
`:0.1.0`); every release also tags its minor version (`:0.1`) and attaches
linux/amd64 and arm64 archives of the Go services to the GitHub release.

Two first-run notes: the ghcr packages are created **private** — flip each
to public in its package settings so anonymous `pull` works — and behind an
HTTPS reverse proxy, set `SHRL_COOKIE_SECURE=true` and point
`SHRL_DEFAULT_BASE_URL` at the redirector's public URL.

## Security

**shrl.io is pre-1.0.** The design is privacy-first and defensive, but the
software is young: watch releases, keep deployments updated, and assume the
security posture will keep hardening until 1.0.

### Deploy with care

- Serve the redirector and frontend behind an HTTPS reverse proxy and set
  `SHRL_COOKIE_SECURE=true`.
- Generate `SHRL_API_INTERNAL_SECRET` and `SHRL_SESSION_SECRET` with
  `openssl rand -hex 32`; never reuse values from another deployment.
- Keep PostgreSQL, Redis, and the Internal API off the public internet —
  the production compose file publishes only the redirector (:8080), the
  Auth API (:8083), and the frontend (:8082).
- Rotate API Keys when a machine or script might have leaked them: keys
  do not expire on their own (yet).
- The first-run admin password is generated and printed once to the api
  logs unless `SHRL_ADMIN_PASSWORD` is set; change it promptly.

### Known limitations (pre-1.0)

- API Keys never expire — rotate them manually when in doubt.
- No two-factor authentication and no audit log yet.
- Admins can read every Link on the instance, including Team Links.

### Reporting a vulnerability

Please do not open public issues. Use GitHub's private vulnerability
reporting (the **Report a vulnerability** button under the **Security**
tab) — see [`SECURITY.md`](SECURITY.md) for what to include and how fixes
ship. The product's security design is described under
[Security model](#security-model).

## Development

Prerequisites: Go 1.25+, podman (with podman-compose).

### Run the full stack

`dev.compose.yaml` is the local development stack (it builds images from
source). For deployment from prebuilt images, see **Production** above.

    podman compose -f dev.compose.yaml up --build

On first run the api provisions an **admin** account: the password is either
`SHRL_ADMIN_PASSWORD` (if set) or a random value printed once to the api
service logs (`podman logs shrl-io_api_1`).

Services:

- Redirector: http://localhost:8080/{code}
- Auth API: http://localhost:8083 (public, `/v1`, API-key auth)
- Frontend: http://localhost:8082 (sign in with the admin account)

The Internal API is not published to the host (frontend-only, ADR 0015).

### Create a Link

Scripts and CI use the public **Auth API** with an **API key**. Create a key
on the Profile page of the UI (it is shown once), then:

    curl -X POST http://localhost:8083/v1/links \
      -H "Authorization: Bearer <your-api-key>" \
      -H "Content-Type: application/json" \
      -d '{"base_url":"http://localhost:8080","destination":"https://example.com"}'

Then visit `http://localhost:8080/{code}` — you get a 302 to the destination.
The redirector rate-limits per IP (default 600 req/min) and per Link (default
3000 req/min); excess requests get `429` with a `Retry-After` header.

The Auth API is rate-limited per IP (default 60 req/min) and per key (300
req/min reads, 30 req/min writes); excess requests get `429` with a
`Retry-After` header. Link `base_url` must be a registered Base URL
(admin-managed); it defaults to `SHRL_DEFAULT_BASE_URL` (`http://localhost:8080`,
auto-registered on first run), or pass a `base_url` field to target another.

### Analytics (the worker aggregates visits from the Redis stream)

    curl -s "http://localhost:8083/v1/links/{code}/analytics" \
      -H "Authorization: Bearer <your-api-key>"
    curl -s "http://localhost:8083/v1/links/{code}/analytics/timeseries" \
      -H "Authorization: Bearer <your-api-key>"
    curl -s "http://localhost:8083/v1/links/{code}/analytics/breakdowns?dimension=referrer" \
      -H "Authorization: Bearer <your-api-key>"

Dimensions: `referrer`, `device`, `os`, `browser`, `country`, `region`,
`city`, and the six `utm_*` parameters (`utm_source`, `utm_medium`,
`utm_campaign`, `utm_term`, `utm_content`, `utm_id`). Bots and link-preview
unfurlers are excluded. Rollups are pruned after
`SHRL_RETENTION_DAYS` (default 365); the lifetime visit total is never pruned.
Country/region/city attribution is optional — set `SHRL_GEOLITE_LICENSE` (a
free MaxMind account) to enable it; without it, locations report as `unknown`.
Visitor IPs are never stored, only the derived location.

### Run the tests

    go test ./...

## Configuration

All services are configured via environment variables. Each service reads its
own set; variables shared across services (Postgres, Redis, retention) are
listed per service so every section is self-contained.

To run a service without compose (or to override compose defaults), start from
the repo-root `.env.example`: copy it to `.env` and adjust. Compose reads the
root `.env` automatically; standalone Go runs need it sourced first
(`set -a; source .env; set +a`). The file mirrors the tables below, one
section per service.

### Redirector

The Redis-only public server that 302s visitors to their Destination and
records each Visit onto the Redis stream (ADR 0001, ADR 0018). Reads Links from
Redis only, never from Postgres.

| Variable                          | Default          | Purpose                                    |
|-----------------------------------|------------------|--------------------------------------------|
| `SHRL_REDIRECTOR_ADDR`            | `:8080`          | Redirector listen address                  |
| `SHRL_REDIRECTOR_RATE_LIMIT_IP`   | `600`            | Per-IP redirects per minute; `0` disables  |
| `SHRL_REDIRECTOR_RATE_LIMIT_LINK` | `3000`           | Per-Link redirects per minute; `0` disables |
| `SHRL_REDIS_ADDR`                 | `localhost:6379` | Redis address                              |
| `SHRL_REDIS_POOL_SIZE`            | `50`             | Redis connection pool size                 |
| `SHRL_REDIS_MIN_IDLE_CONNS`       | `5`              | Minimum idle Redis connections             |

### Worker

The analytics aggregator: consumes the Redis visit stream in batches and
upserts daily, lifetime, and breakdown rollups into Postgres in a single
transaction (ADR 0003).

| Variable                  | Default                                              | Purpose                                       |
|---------------------------|------------------------------------------------------|-----------------------------------------------|
| `SHRL_DATABASE_URL`       | `postgres://shrl:shrl@localhost:5432/shrl`           | PostgreSQL connection string                  |
| `SHRL_DB_MAX_OPEN_CONNS`  | `20`                                                 | Max open Postgres connections                 |
| `SHRL_DB_MAX_IDLE_CONNS`  | `5`                                                  | Max idle Postgres connections                 |
| `SHRL_DB_CONN_MAX_LIFETIME` | `30m`                                              | Max lifetime of a Postgres connection         |
| `SHRL_DB_CONN_MAX_IDLE_TIME` | `5m`                                               | Max idle time of a Postgres connection        |
| `SHRL_REDIS_ADDR`         | `localhost:6379`                                     | Redis address                                 |
| `SHRL_REDIS_POOL_SIZE`    | `0` (auto, 10×CPU)                                   | Redis connection pool size                    |
| `SHRL_REDIS_MIN_IDLE_CONNS` | `2`                                                | Minimum idle Redis connections                |
| `SHRL_RETENTION_DAYS`     | `365`                                                | Analytics retention window (daily rollups)    |
| `SHRL_GEOLITE_LICENSE`    | *(unset)*                                            | MaxMind license key; enables GeoIP attribution |
| `SHRL_GEOLITE_DB_PATH`    | `/data/GeoLite2-City.mmdb`                           | Path to the GeoLite2 City database            |

### Internal API

The API that serves the UI — reachable only by the frontend, which proxies
every request on the signed-in user's behalf and presents the session token
(ADR 0015).

| Variable                  | Default                                              | Purpose                                       |
|---------------------------|------------------------------------------------------|-----------------------------------------------|
| `SHRL_API_ADDR`           | `:8080`                                              | Internal API listen address                   |
| `SHRL_API_INTERNAL_SECRET` | `dev-internal-secret`                               | Shared secret the Internal API demands on every request (set to the same value on the frontend) |
| `SHRL_ADMIN_USERNAME`     | `admin`                                              | Username of the first-run Admin account       |
| `SHRL_ADMIN_PASSWORD`     | *(random, shown once)*                               | First-run Admin password (bcrypt-hashed)      |
| `SHRL_TOKEN_TTL`          | `86400`                                              | Bearer token lifetime in seconds              |
| `SHRL_CODE_LENGTH`        | `6`                                                  | Seed for the per-instance Code Length setting (4–12) |
| `SHRL_DEFAULT_BASE_URL`   | `http://localhost:8080`                            | Base URL auto-registered on first run and pre-selected when creating a Link |
| `SHRL_DATABASE_URL`       | `postgres://shrl:shrl@localhost:5432/shrl`           | PostgreSQL connection string                  |
| `SHRL_DB_MAX_OPEN_CONNS`  | `20`                                                 | Max open Postgres connections                 |
| `SHRL_DB_MAX_IDLE_CONNS`  | `5`                                                  | Max idle Postgres connections                 |
| `SHRL_DB_CONN_MAX_LIFETIME` | `30m`                                              | Max lifetime of a Postgres connection         |
| `SHRL_DB_CONN_MAX_IDLE_TIME` | `5m`                                               | Max idle time of a Postgres connection        |
| `SHRL_REDIS_ADDR`         | `localhost:6379`                                     | Redis address                                 |
| `SHRL_REDIS_POOL_SIZE`    | `0` (auto, 10×CPU)                                   | Redis connection pool size                    |
| `SHRL_REDIS_MIN_IDLE_CONNS` | `2`                                                | Minimum idle Redis connections                |
| `SHRL_RETENTION_DAYS`     | `365`                                                | Analytics retention window (daily rollups)    |

### Auth API

The public `/v1` API for scripts and CI, authenticated by an API key on every
request and rate-limited per IP and per key (ADR 0016, ADR 0017).

| Variable                        | Default                                              | Purpose                                      |
|---------------------------------|------------------------------------------------------|----------------------------------------------|
| `SHRL_AUTH_ADDR`                | `:8080`                                              | Auth API listen address                      |
| `SHRL_AUTH_RATE_LIMIT_IP`       | `60`                                                 | Per-IP requests per minute                   |
| `SHRL_AUTH_RATE_LIMIT_KEY_READ` | `300`                                                | Per-key reads per minute                     |
| `SHRL_AUTH_RATE_LIMIT_KEY_WRITE`| `30`                                                 | Per-key writes per minute                    |
| `SHRL_AUTH_RATE_LIMIT_FAIL`     | `10`                                                 | Failed key validations per minute per IP     |
| `SHRL_DEFAULT_BASE_URL`         | `http://localhost:8080`                              | Base URL pre-selected when creating a Link   |
| `SHRL_DATABASE_URL`             | `postgres://shrl:shrl@localhost:5432/shrl`           | PostgreSQL connection string                 |
| `SHRL_DB_MAX_OPEN_CONNS`        | `20`                                                 | Max open Postgres connections                |
| `SHRL_DB_MAX_IDLE_CONNS`        | `5`                                                  | Max idle Postgres connections                |
| `SHRL_DB_CONN_MAX_LIFETIME`     | `30m`                                                | Max lifetime of a Postgres connection        |
| `SHRL_DB_CONN_MAX_IDLE_TIME`    | `5m`                                                 | Max idle time of a Postgres connection       |
| `SHRL_REDIS_ADDR`               | `localhost:6379`                                     | Redis address                                |
| `SHRL_REDIS_POOL_SIZE`          | `0` (auto, 10×CPU)                                   | Redis connection pool size                   |
| `SHRL_REDIS_MIN_IDLE_CONNS`     | `2`                                                  | Minimum idle Redis connections               |
| `SHRL_RETENTION_DAYS`           | `365`                                                | Analytics retention window (daily rollups)   |

### Frontend

The SvelteKit admin UI: signs users in with an HttpOnly session cookie and
proxies every API call to the Internal API (ADR 0005).

| Variable                  | Default                  | Purpose                                                    |
|---------------------------|--------------------------|------------------------------------------------------------|
| `SHRL_API_URL`            | `http://localhost:8080`  | Internal API address the UI proxies to                     |
| `SHRL_API_INTERNAL_SECRET` | `dev-internal-secret`   | Shared secret the Internal API demands on every request (set to the same value on the api) |
| `SHRL_DEFAULT_BASE_URL`   | `http://localhost:8080`              | Base URL pre-selected when creating a Link                 |
| `SHRL_SESSION_SECRET`     | *(random per boot)*      | HMAC secret for signing UI session cookies                 |
| `SHRL_SESSION_TTL`        | `86400`                  | UI session cookie lifetime in seconds                      |
| `SHRL_COOKIE_SECURE`      | `false`                  | Set `true` to send the session cookie over TLS only        |

## API reference

Programmatic access goes through the **Auth API** below: the public `/v1`
surface authenticated with an API key. The **Internal API** that serves the
UI is frontend-only (ADR 0015) and is not documented here.

### API model

A Link is a JSON object: `base_url`, `code`, `destination`, `remark`,
`forward_utm`, `disabled`, `created_by`, `team_id`, `created_at`, `updated_at`.
`team_id` is `null` for Personal Links. `forward_utm` (default `false`) appends
the six recognized `utm_*` parameters from a Visitor's short URL to the
Destination on Redirect; a same-named parameter on the Destination is
overridden, other Destination query parameters are preserved, and empty values
are skipped. `forward_utm` may be omitted from a PATCH to keep its current
value.

Teams are the ownership boundary for Links: a Link belongs to exactly one Team
or is Personal (no Team), and a Link's Team is fixed — it never moves. Team
Members see all of the Team's Links and their analytics read-only; a Link is
managed by its Creator (while a member of the Team) or by a Team Owner.
Membership runs on Invite Codes: a Team Owner generates single-use codes and a
User joins by entering one; only an Admin adds members directly by username.
Joining or leaving a Team never moves existing Personal Links.

Query parameters:

- `from` / `to` (analytics reads) — `YYYY-MM-DD` bounds; default to the
  retention window.
- `dimension` (breakdowns) — one of the analytics dimensions, default
  `referrer`.
- `limit` (breakdowns) — top-N, default `10`; `0` returns all values.

The `hostname` query parameter was removed from every read/manage endpoint:
a Code is globally unique, so Links are identified by Code alone. `base_url`
is still required in the create request body (the target domain).

### Auth API (public, API keys)

The **Auth API** is the public `/v1` surface for scripts and CI. Every
request must present a valid **API key** as `Authorization: Bearer <key>`
(missing or invalid keys get `401`). It serves everything except deletion for
both Personal and Team Links; admin, key-management, and login endpoints are
not exposed. Requests are rate-limited per IP and per key (see the Auth API
[configuration](#auth-api) below); excess requests get `429` with a
`Retry-After` header.

| Method | Path                                   | Purpose                                             |
|--------|----------------------------------------|-----------------------------------------------------|
| POST   | `/v1/links`                            | Create a Personal Link                              |
| GET    | `/v1/links`                            | List the caller's Personal Links (across every Base URL) |
| GET    | `/v1/base-urls`                       | List registered Base URLs                            |
| GET    | `/v1/links/{code}`                     | Get a Link (Personal or Team)                       |
| PATCH  | `/v1/links/{code}`                     | Update a Link's Destination, Remark, Forward UTM   |
| POST   | `/v1/links/{code}/disable`             | Disable a Link                                      |
| POST   | `/v1/links/{code}/enable`              | Enable a Link                                       |
| GET    | `/v1/links/{code}/analytics`           | Lifetime and window visit totals                    |
| GET    | `/v1/links/{code}/analytics/timeseries` | Daily visit buckets in the window, ascending      |
| GET    | `/v1/links/{code}/analytics/breakdowns` | Top-N dimension values in the window              |
| GET    | `/v1/stats`                            | Dashboard totals and daily timeseries for the caller's Personal Links |
| GET    | `/v1/teams`                            | List the caller's Teams (with their role)          |
| GET    | `/v1/teams/{id}`                       | Team details (members and admins)                   |
| GET    | `/v1/teams/{id}/links`                 | The Team's Links, read-only for members             |
| POST   | `/v1/teams/{id}/links`                 | Create a Link in the Team (members)                 |
| GET    | `/v1/teams/{id}/stats`                 | Dashboard totals and daily timeseries for the Team's Links (read-only for members) |

There is no delete endpoint on the Auth API (ADR 0016). Links are managed
with the same permissions as in the UI: a Team Member reads Team Links; the
Creator (while a member) or a Team Owner manages them. Keys are created and
revoked on the Profile page of the UI.

## Terminology

This project uses a precise domain vocabulary (Link, Code, Base URL,
Destination, Remark, Visit, Visitor, Bot, Location, UTM Parameter, Forward UTM,
Campaign, Redirect, Disabled, Delete,
User, Admin, Creator, Personal Link, Team, Team Link, Team Owner, Team Member,
Invite Code, Token, Password). See [`CONTEXT.md`](CONTEXT.md) for definitions
and the words to avoid.

## Documentation

Architecture decision records (ADRs) live in [`docs/adr/`](docs/adr/).

## License

MIT — see [`LICENSE`](LICENSE).
