```
   ███████╗██╗  ██╗██████╗ ██╗         ██╗ ██████╗
   ██╔════╝██║  ██║██╔══██╗██║         ██║██╔═══██╗
   ███████╗███████║██████╔╝██║         ██║██║   ██║
   ╚════██║██╔══██║██╔══██╗██║         ██║██║   ██║
   ███████║██║  ██║██║  ██║███████╗ ██ ██║╚██████╔╝
   ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝ ╚╝ ╚═╝  ╚═════╝
   https://www.shrl.io   
```

<div align="center">

[![CI](https://github.com/barats/shrl-io/actions/workflows/ci.yml/badge.svg)](https://github.com/barats/shrl-io/actions/workflows/ci.yml)

</div>

---

## Overview

**shrl.io** is a self-hosted URL shortener and traffic analyzer. It turns a
**Destination** URL into a short, shareable URL under your own **Hostname** and
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
- 🏠 **Simple to self-host**: one `podman compose up` brings up the whole
  stack; no external accounts required (GeoIP attribution is optional).
- 🖥️ **Built-in admin UI**: sign in with your account and manage Links from
  the browser, no curl required.
- 🛡️ **Secure by default**: URL validation and open-redirect protection, plus
  password accounts with bearer-token auth. The internal API is frontend-only
  (never published to the host); scripts and CI use the public Auth API with
  an API key, rate-limited per key and per IP.

## Features

### Link management

- **Auto codes**: shrl.io generates every **Code**: lowercase, from an
  unambiguous alphabet (no `0`/`O`/`1`/`l`). Users never choose a Code. The
  exact **Code Length** defaults to 6; an Admin can set it per instance
  (4–12) in Settings.
- **Admin-managed hostnames**: an **Admin** registers **Hostnames**; Users
  select from the registry when creating a Link. A Code is unique per Hostname.
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
  `city`.
- **Bot filtering**: known crawlers and link-preview unfurlers are excluded at
  aggregation time.
- **Retention pruning**: daily rollups are pruned after `SHRL_RETENTION_DAYS`
  (default 365); the lifetime visit total is never pruned.
- **Optional GeoIP**: set `SHRL_GEOLITE_LICENSE` (a free MaxMind account) to
  attribute country/region/city; without it, locations report as `unknown`.

### Admin UI & accounts

- **Accounts**: password-based Users with bcrypt hashes; the first account is
  an **Admin** provisioned on first run with a random password shown once in
  the api logs (`SHRL_ADMIN_PASSWORD` sets a known one). Admins create
  accounts; there is no self-registration.
- **Login**: sign in with username + password; the UI issues an HttpOnly
  session cookie and proxies API calls server-side with the user's token, so
  the password and token never reach the browser.
- **Change password**: a User replaces their own password on the Account
  page; doing so revokes every other sign-in and every API key, keeping only
  the current session.
- **API keys**: long-lived, named bearer credentials for the public **Auth
  API** — scripts and CI authenticate with them instead of a session. Created
  and revoked per User on the Account page, shown once at creation, and
  revoked on a password change.
- **Admin password reset**: an Admin resets a forgotten password to a
  generated temporary one (shown once); the User must change it on their next
  sign-in before using the instance. There is no SMTP-based reset.
- **Link management**: each User sees and manages only their own Links, per
  Hostname: create, edit the Destination, disable/enable, and delete.
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
  and the Team's Links with a hostname filter; Team Links have their own detail
  page with analytics.

### Settings (admin)

- **Hostnames**: register and remove Hostnames in the Registry.
- **Accounts**: create and delete Users; deleting a User removes their
  Personal Links and memberships — Team Links they created stay with the Team.
- **Teams**: create and delete Teams; deleting a Team reverts its Links to
  Personal.

### Security

- **Open-redirect protection**: only `http`/`https` Destinations are accepted;
  loopback, private, and link-local addresses are rejected at create/update
  time.
- **Account auth**: the **Internal API** is reachable only by the frontend,
  which presents a shared secret header and the user's bearer token (ADR
  0015). The **Auth API** is public and validates an **API Key** on every
  request, rate-limited per key and per IP; keys are checked against
  Postgres, the single source of truth (ADR 0017).
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
2. **Redirect**: the redirector reads the Link from Redis — never PostgreSQL —
   and returns a 302. It pushes a Visit event onto the Redis stream.
3. **Aggregate**: the worker consumes the stream in batches and upserts daily,
   lifetime, and breakdown rollups into PostgreSQL in a single transaction;
   stale rollups are pruned after the retention window.

## Roadmap

Planned, not yet built:

- **Geographic maps**: country/region map views in the admin analytics screen
  (the base dashboard UI now ships with charts)
- **Rate limiting** on the redirector (the Auth API is already rate-limited
  per key and per IP)

## Development

Prerequisites: Go 1.25+, podman (with podman-compose).

### Run the full stack

    podman compose up --build

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
on the Account page of the UI (it is shown once), then:

    curl -X POST http://localhost:8083/v1/links \
      -H "Authorization: Bearer <your-api-key>" \
      -H "Content-Type: application/json" \
      -d '{"hostname":"localhost","destination":"https://example.com"}'

Then visit `http://localhost:8080/{code}` — you get a 302 to the destination.

The Auth API is rate-limited per IP (default 60 req/min) and per key (300
req/min reads, 30 req/min writes); excess requests get `429` with a
`Retry-After` header. Link `hostname` must be a registered Hostname
(admin-managed); it defaults to `SHRL_DEFAULT_HOSTNAME` (`localhost`,
auto-registered on first run), or pass a `hostname` field to target another.

### Analytics (the worker aggregates visits from the Redis stream)

    curl -s "http://localhost:8083/v1/links/{code}/analytics?hostname=localhost" \
      -H "Authorization: Bearer <your-api-key>"
    curl -s "http://localhost:8083/v1/links/{code}/analytics/timeseries?hostname=localhost" \
      -H "Authorization: Bearer <your-api-key>"
    curl -s "http://localhost:8083/v1/links/{code}/analytics/breakdowns?hostname=localhost&dimension=referrer" \
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

All services are configured via environment variables.

| Variable                | Default                                              | Services      | Purpose                                          |
|-------------------------|------------------------------------------------------|---------------|--------------------------------------------------|
| `SHRL_ADMIN_USERNAME`   | `admin`                                              | api           | Username of the first-run Admin account          |
| `SHRL_ADMIN_PASSWORD`   | *(random, shown once)*                               | api           | First-run Admin password (bcrypt-hashed)         |
| `SHRL_TOKEN_TTL`        | `86400`                                              | api           | Bearer token lifetime in seconds                 |
| `SHRL_DATABASE_URL`     | `postgres://shrl:shrl@localhost:5432/shrl`           | api, auth, worker | PostgreSQL connection string                 |
| `SHRL_REDIS_ADDR`       | `localhost:6379`                                     | all           | Redis address                                    |
| `SHRL_API_ADDR`         | `:8080`                                              | api           | Internal API listen address                      |
| `SHRL_API_INTERNAL_SECRET` | `dev-internal-secret`                             | api, frontend | Shared secret the Internal API demands on every request (set to the same value on both) |
| `SHRL_AUTH_ADDR`        | `:8080`                                              | auth          | Auth API listen address                          |
| `SHRL_AUTH_RATE_LIMIT_IP` | `60`                                              | auth          | Per-IP requests per minute on the Auth API       |
| `SHRL_AUTH_RATE_LIMIT_KEY_READ` | `300`                                        | auth          | Per-key reads per minute on the Auth API         |
| `SHRL_AUTH_RATE_LIMIT_KEY_WRITE` | `30`                                        | auth          | Per-key writes per minute on the Auth API        |
| `SHRL_AUTH_RATE_LIMIT_FAIL` | `10`                                              | auth          | Failed key validations per minute per IP         |
| `SHRL_REDIRECTOR_ADDR`  | `:8080`                                              | redirector    | Redirector listen address                        |
| `SHRL_DEFAULT_HOSTNAME` | `localhost`                                          | api, auth, frontend | Hostname auto-registered on first run; used when a request specifies none |
| `SHRL_CODE_LENGTH`      | `6`                                                  | api           | Seed for the per-instance Code Length setting (4–12) |
| `SHRL_RETENTION_DAYS`   | `365`                                                | api, auth, worker | Analytics retention window (daily rollups)    |
| `SHRL_GEOLITE_LICENSE`  | *(unset)*                                            | worker        | MaxMind license key; enables GeoIP attribution   |
| `SHRL_GEOLITE_DB_PATH`  | `/data/GeoLite2-City.mmdb`                           | worker        | Path to the GeoLite2 City database               |
| `SHRL_API_URL`          | `http://localhost:8080`                              | frontend      | Internal API address the UI proxies to           |
| `SHRL_SESSION_SECRET`   | *(random per boot)*                                  | frontend      | HMAC secret for signing UI session cookies       |
| `SHRL_COOKIE_SECURE`    | `false`                                              | frontend      | Set `true` to send the session cookie over TLS only |

## API reference

Two API surfaces exist. The **Internal API** is frontend-only: it demands the
shared `X-Shrl-Internal-Secret` header on every request (ADR 0015) and every
endpoint except `POST /login` and `POST /logout` additionally requires
`Authorization: Bearer <token>`. The **Auth API** (below) is public and
authenticates with an API key.

### Internal API (frontend only)

Every endpoint except `POST /login` and `POST /logout` requires
`Authorization: Bearer <token>`; requests without a valid token get `401`.
Get a token from `POST /login`. Links are scoped to the authenticated User: a
User sees the Personal Links they created and, read-only, the Links of any
Team they belong to.

| Method | Path                                   | Purpose                                             |
|--------|----------------------------------------|-----------------------------------------------------|
| POST   | `/login`                               | Sign in; returns a bearer `token` and the `user`    |
| POST   | `/logout`                              | Revoke the presented token                          |
| GET    | `/me`                                  | The authenticated user                              |
| GET    | `/users`                               | List users (admin only)                             |
| POST   | `/users`                               | Create a user (admin only); password returned once  |
| DELETE | `/users/{id}`                          | Delete a user (admin only): removes Personal Links and memberships; Team Links stay with the Team |
| POST   | `/users/{id}/reset`                   | Reset a user's password (admin only); temp password shown once, forced change on next sign-in |
| POST   | `/account/password`                   | Change your own password; revokes other tokens and all API keys |
| POST   | `/keys`                               | Create an API key; the secret is shown once          |
| GET    | `/keys`                               | List your API keys                                   |
| DELETE | `/keys/{id}`                          | Revoke an API key                                    |
| GET    | `/settings`                           | Instance settings (admin only)                       |
| PATCH  | `/settings/code-length`               | Set the per-instance Code Length, 4-12 (admin only)  |
| POST   | `/links`                               | Create a Link (Code auto-generated; Remark and Forward UTM optional) |
| GET    | `/links`                               | List the current user's Links for a Hostname        |
| GET    | `/hostnames`                           | List registered Hostnames (the registry)            |
| POST   | `/hostnames`                           | Register a Hostname (admin only)                    |
| DELETE | `/hostnames/{hostname}`                | Remove a Hostname from the registry (admin only)    |
| GET    | `/links/{code}`                        | Get a Link                                          |
| PATCH  | `/links/{code}`                        | Update a Link's Destination, Remark, and Forward UTM |
| POST   | `/links/{code}/disable`                | Disable a Link (redirector returns 404)             |
| POST   | `/links/{code}/enable`                 | Enable a Link                                       |
| DELETE | `/links/{code}`                        | Delete a Link                                       |
| GET    | `/links/{code}/analytics`              | Lifetime and window visit totals                    |
| GET    | `/links/{code}/analytics/timeseries`   | Daily visit buckets in the window, ascending        |
| GET    | `/links/{code}/analytics/breakdowns`   | Top-N dimension values in the window                |
| POST   | `/teams`                               | Create a Team (admin only); the admin becomes first Team Owner |
| GET    | `/teams`                               | List the caller's Teams (with their role); admins see all |
| GET    | `/teams/{id}`                          | Team details and members (members and admins)       |
| GET    | `/teams/{id}/links`                    | The Team's Links, read-only for members             |
| POST   | `/teams/{id}/links`                    | Create a Link in the Team (members)                 |
| POST   | `/teams/{id}/members`                  | Add an existing user as a member (admin only)       |
| PATCH  | `/teams/{id}/members/{userID}`         | Promote or demote a member (Team Owner)             |
| DELETE | `/teams/{id}/members/{userID}`         | Remove a member (Team Owner); a member may remove themself |
| POST   | `/teams/{id}/invites`                  | Generate a single-use Invite Code (Team Owner)      |
| GET    | `/teams/{id}/invites`                  | List outstanding Invite Codes (Team Owner)          |
| DELETE | `/teams/{id}/invites/{code}`           | Revoke an outstanding Invite Code (Team Owner)      |
| POST   | `/teams/join`                          | Join a Team by entering an Invite Code              |
| DELETE | `/teams/{id}`                          | Delete a Team (admin only); its Links revert to Personal |

A Link is a JSON object: `hostname`, `code`, `destination`, `remark`,
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

- `hostname` (all endpoints) — defaults to `SHRL_DEFAULT_HOSTNAME`; must be a
  registered Hostname.
- `from` / `to` (analytics reads) — `YYYY-MM-DD` bounds; default to the
  retention window.
- `dimension` (breakdowns) — one of the analytics dimensions, default
  `referrer`.
- `limit` (breakdowns) — top-N, default `10`; `0` returns all values.

### Auth API (public, API keys)

The **Auth API** is the public `/v1` surface for scripts and CI. Every
request must present a valid **API key** as `Authorization: Bearer <key>`
(missing or invalid keys get `401`). It serves everything except deletion for
both Personal and Team Links; admin, key-management, and login endpoints are
not exposed. Requests are rate-limited per IP and per key (see the env table);
excess requests get `429` with a `Retry-After` header.

| Method | Path                                   | Purpose                                             |
|--------|----------------------------------------|-----------------------------------------------------|
| POST   | `/v1/links`                            | Create a Personal Link                              |
| GET    | `/v1/links`                            | List the caller's Personal Links for a Hostname    |
| GET    | `/v1/hostnames`                        | List registered Hostnames                           |
| GET    | `/v1/links/{code}`                     | Get a Link (Personal or Team)                       |
| PATCH  | `/v1/links/{code}`                     | Update a Link's Destination, Remark, Forward UTM   |
| POST   | `/v1/links/{code}/disable`             | Disable a Link                                      |
| POST   | `/v1/links/{code}/enable`              | Enable a Link                                       |
| GET    | `/v1/links/{code}/analytics`           | Lifetime and window visit totals                    |
| GET    | `/v1/links/{code}/analytics/timeseries` | Daily visit buckets in the window, ascending      |
| GET    | `/v1/links/{code}/analytics/breakdowns` | Top-N dimension values in the window              |
| GET    | `/v1/teams`                            | List the caller's Teams (with their role)          |
| GET    | `/v1/teams/{id}`                       | Team details (members and admins)                   |
| GET    | `/v1/teams/{id}/links`                 | The Team's Links, read-only for members             |
| POST   | `/v1/teams/{id}/links`                 | Create a Link in the Team (members)                 |

There is no delete endpoint on the Auth API (ADR 0016). Links are managed
with the same permissions as in the UI: a Team Member reads Team Links; the
Creator (while a member) or a Team Owner manages them. Keys are created and
revoked on the Account page of the UI.

## Terminology

This project uses a precise domain vocabulary (Link, Code, Hostname,
Destination, Remark, Visit, Visitor, Bot, Location, UTM Parameter, Forward UTM,
Campaign, Redirect, Disabled, Delete,
User, Admin, Creator, Personal Link, Team, Team Link, Team Owner, Team Member,
Invite Code, Token, Password). See [`CONTEXT.md`](CONTEXT.md) for definitions
and the words to avoid.

## Documentation

Architecture decision records (ADRs) live in [`docs/adr/`](docs/adr/).
