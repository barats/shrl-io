```
   ███████╗██╗  ██╗██████╗ ██╗         ██╗ ██████╗
   ██╔════╝██║  ██║██╔══██╗██║         ██║██╔═══██╗
   ███████╗███████║██████╔╝██║         ██║██║   ██║
   ╚════██║██╔══██║██╔══██╗██║         ██║██║   ██║
   ███████║██║  ██║██║  ██║███████╗ ██ ██║╚██████╔╝
   ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝ ╚╝ ╚═╝  ╚═════╝
   shrl.io
   SELF-HOSTED URL SHORTENER & TRAFFIC ANALYZER
```

> **Status: MVP** — self-hosted redirects, privacy-first analytics, a multi-user admin UI, and Teams today. QR codes, rate limiting, and UTM are planned (see [Roadmap](#roadmap)).

[![CI](https://github.com/barats/shrl-io/actions/workflows/ci.yml/badge.svg)](https://github.com/barats/shrl-io/actions/workflows/ci.yml)

---

## Overview

**shrl.io** is a self-hosted URL shortener and traffic analyzer. It turns a
**Destination** URL into a short, shareable URL under your own **Hostname** and
redirects visitors there at sub-millisecond speed, while recording
privacy-first traffic analytics. It is built as a small set of Go
microservices — an API, a Redis-backed redirector, and an analytics worker —
over PostgreSQL and Redis, with a SvelteKit admin UI, and is designed for
teams and individuals who want full control over their link data.

### Why shrl.io?

- 🔒 **Privacy-first**: your data stays on your infrastructure. Visitor IPs are
  never stored — only derived **Locations** and aggregate counts.
- ⚡ **Blazing fast**: Redis-backed redirects with sub-millisecond response
  times; the database never sits on the redirect hot path.
- 📊 **Rich analytics**: visits, unique **Visitors**, daily time series, and
  breakdowns by referrer, device, OS, browser, country, region, and city.
- 🏠 **Simple to self-host**: one `podman compose up` brings up the whole
  stack; no external accounts required (GeoIP attribution is optional).
- 🖥️ **Built-in admin UI**: sign in with your account and manage Links from
  the browser, no curl required.
- 🛡️ **Secure by default**: URL validation and open-redirect protection, plus
  password accounts with bearer-token auth on every API endpoint.

## Features

### Link management

- **Auto codes**: shrl.io generates every **Code**: 6 characters, lowercase,
  from an unambiguous alphabet (no `0`/`O`/`1`/`l`). Users never choose a Code.
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
- **API keys**: long-lived, named bearer credentials for scripts and CI,
  created and revoked per User on the Account page, shown once at creation,
  and revoked on a password change.
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
- **Account auth**: every API endpoint requires a bearer token from a password
  login; tokens are stored hashed, revocable on logout.
- **No raw IPs persisted**: Visitor identity is stored as a hash of
  `(Link, day, IP + user-agent)`; the IP itself is never written.

## Architecture

```
                          ┌───────────────────┐
   user (browser) ───────►│ frontend (Svelte) │   login + session cookie
                          └─────────┬─────────┘
                                    │ proxies with the user's token
                                    ▼
   create / manage Link ──► ┌──────────────┐
                            │     api      │   REST API, bearer-token auth
                            └──────┬───────┘
                                 │ write-through (Link cache)
                     ┌───────────┴───────────┐
                     ▼                       ▼
              ┌──────────────┐        ┌──────────────┐
              │  PostgreSQL  │        │    Redis     │
              │ source of    │        │   Link cache │
              │ truth        │        │ Visit stream │
              └──────────────┘        └──────┬───────┘
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

1. **Create**: the api writes a Link to PostgreSQL, then caches it in Redis
   (write-through). The api also re-warms the cache every 5 minutes.
2. **Redirect**: the redirector reads the Link from Redis — never PostgreSQL —
   and returns a 302. It pushes a Visit event onto the Redis stream.
3. **Aggregate**: the worker consumes the stream in batches and upserts daily,
   lifetime, and breakdown rollups into PostgreSQL in a single transaction;
   stale rollups are pruned after the retention window.

## Roadmap

Planned, not yet built:

- **Geographic maps**: country/region map views in the admin analytics screen
  (the base dashboard UI now ships with charts)
- **Rate limiting** on the API and redirector
- **UTM campaign tracking**

## Development

Prerequisites: Go 1.25+, podman (with podman-compose).

### Run the full stack

    podman compose up --build

On first run the api provisions an **admin** account: the password is either
`SHRL_ADMIN_PASSWORD` (if set) or a random value printed once to the api
service logs (`podman logs shrl-io_api_1`).

Services:

- Redirector: http://localhost:8080/{code}
- API: http://localhost:8081
- Frontend: http://localhost:8082 (sign in with the admin account)

### Create a Link

    # get a bearer token
    TOKEN=$(curl -s http://localhost:8081/login \
      -H "Content-Type: application/json" \
      -d '{"username":"admin","password":"<the-admin-password>"}' \
      | jq -r .token)

    curl -X POST http://localhost:8081/links \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"hostname":"localhost","destination":"https://example.com"}'

Then visit `http://localhost:8080/{code}` — you get a 302 to the destination.

The API `hostname` must be a registered Hostname (admin-managed); it defaults to
`SHRL_DEFAULT_HOSTNAME` (`localhost`, auto-registered on first run), or pass
`?hostname=` / a `hostname` field to target another.

### Analytics (the worker aggregates visits from the Redis stream)

    curl -s "http://localhost:8081/links/{code}/analytics?hostname=localhost" \
      -H "Authorization: Bearer $TOKEN"
    curl -s "http://localhost:8081/links/{code}/analytics/timeseries?hostname=localhost" \
      -H "Authorization: Bearer $TOKEN"
    curl -s "http://localhost:8081/links/{code}/analytics/breakdowns?hostname=localhost&dimension=referrer" \
      -H "Authorization: Bearer $TOKEN"

Dimensions: `referrer`, `device`, `os`, `browser`, `country`, `region`,
`city`. Bots and link-preview unfurlers are excluded. Rollups are pruned after
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
| `SHRL_DATABASE_URL`     | `postgres://shrl:shrl@localhost:5432/shrl`           | api, worker   | PostgreSQL connection string                     |
| `SHRL_REDIS_ADDR`       | `localhost:6379`                                     | all           | Redis address                                    |
| `SHRL_API_ADDR`         | `:8080`                                              | api           | API listen address                               |
| `SHRL_REDIRECTOR_ADDR`  | `:8080`                                              | redirector    | Redirector listen address                        |
| `SHRL_DEFAULT_HOSTNAME` | `localhost`                                          | api, frontend| Hostname auto-registered on first run; used when a request specifies none |
| `SHRL_RETENTION_DAYS`   | `365`                                                | api, worker   | Analytics retention window (daily rollups)       |
| `SHRL_GEOLITE_LICENSE`  | *(unset)*                                            | worker        | MaxMind license key; enables GeoIP attribution   |
| `SHRL_GEOLITE_DB_PATH`  | `/data/GeoLite2-City.mmdb`                           | worker        | Path to the GeoLite2 City database               |
| `SHRL_API_URL`          | `http://localhost:8081`                              | frontend      | Backend API address the UI proxies to           |
| `SHRL_SESSION_SECRET`   | *(random per boot)*                                  | frontend      | HMAC secret for signing UI session cookies       |
| `SHRL_COOKIE_SECURE`    | `false`                                              | frontend      | Set `true` to send the session cookie over TLS only |

## API reference

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
| POST   | `/links`                               | Create a Link (Code auto-generated; Remark optional) |
| GET    | `/links`                               | List the current user's Links for a Hostname        |
| GET    | `/hostnames`                           | List registered Hostnames (the registry)            |
| POST   | `/hostnames`                           | Register a Hostname (admin only)                    |
| DELETE | `/hostnames/{hostname}`                | Remove a Hostname from the registry (admin only)    |
| GET    | `/links/{code}`                        | Get a Link                                          |
| PATCH  | `/links/{code}`                        | Update a Link's Destination and Remark              |
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
`disabled`, `created_by`, `team_id`, `created_at`, `updated_at`. `team_id` is
`null` for Personal Links.

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

## Terminology

This project uses a precise domain vocabulary (Link, Code, Hostname,
Destination, Remark, Visit, Visitor, Bot, Location, Redirect, Disabled, Delete,
User, Admin, Creator, Personal Link, Team, Team Link, Team Owner, Team Member,
Invite Code, Token, Password). See [`CONTEXT.md`](CONTEXT.md) for definitions
and the words to avoid.

## Documentation

Architecture decision records (ADRs) live in [`docs/adr/`](docs/adr/).
