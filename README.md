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

> **Status: MVP** — self-hosted redirects and privacy-first analytics today. Teams & roles, a dashboard UI, QR codes, rate limiting, and accounts are planned (see [Roadmap](#roadmap)).

---

## Overview

**shrl.io** is a self-hosted URL shortener and traffic analyzer. It turns a
**Destination** URL into a short, shareable URL under your own **Hostname** and
redirects visitors there at sub-millisecond speed, while recording
privacy-first traffic analytics. It is built as a small set of Go
microservices — an API, a Redis-backed redirector, and an analytics worker —
over PostgreSQL and Redis, and is designed for teams and individuals who want
full control over their link data.

### Why shrl.io?

- 🔒 **Privacy-first**: your data stays on your infrastructure. Visitor IPs are
  never stored — only derived **Locations** and aggregate counts.
- ⚡ **Blazing fast**: Redis-backed redirects with sub-millisecond response
  times; the database never sits on the redirect hot path.
- 📊 **Rich analytics**: visits, unique **Visitors**, daily time series, and
  breakdowns by referrer, device, OS, browser, country, region, and city.
- 🏠 **Simple to self-host**: one `podman compose up` brings up the whole
  stack; no external accounts required (GeoIP attribution is optional).
- 🛡️ **Secure by default**: URL validation and open-redirect protection, plus
  admin-key auth on every API endpoint.

## Features

### Link management

- **Auto codes**: 6-character, case-sensitive base62 **Codes** are generated
  for you; custom Codes of 1–32 characters (`[A-Za-z0-9_-]`) are also
  supported.
- **Multiple hostnames**: one instance can serve Links on several Hostnames;
  a Code is unique per Hostname and case-sensitive.
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

### Security

- **Open-redirect protection**: only `http`/`https` Destinations are accepted;
  loopback, private, and link-local addresses are rejected at create/update
  time.
- **Admin-key auth**: every API endpoint requires `SHRL_ADMIN_KEY`, sent as
  `X-API-Key` or `Authorization: Bearer`.
- **No raw IPs persisted**: Visitor identity is stored as a hash of
  `(Link, day, IP + user-agent)`; the IP itself is never written.

## Architecture

```
                          ┌──────────────┐
   create / manage Link ─►│     api      │   REST API, admin-key auth
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

- **Teams & roles**: multi-user collaboration with granular permissions
- **Web dashboard**: a UI with charts and maps (SvelteKit frontend)
- **QR code generation** for every Link, with download
- **Rate limiting** on the API and redirector
- **Sessions & accounts**: login, per-user API keys, bcrypt password hashing
- **UTM campaign tracking**

## Development

Prerequisites: Go 1.25+, podman (with podman-compose).

### Run the full stack

    SHRL_ADMIN_KEY=dev-admin-key podman compose up --build

Services:

- Redirector: http://localhost:8080/{code}
- API: http://localhost:8081

### Create a Link

    curl -X POST http://localhost:8081/links \
      -H "X-API-Key: dev-admin-key" \
      -H "Content-Type: application/json" \
      -d '{"hostname":"localhost","destination":"https://example.com"}'

Then visit `http://localhost:8080/{code}` — you get a 302 to the destination.

The API `hostname` defaults to `SHRL_DEFAULT_HOSTNAME` (`localhost`), or pass
`?hostname=` / a `hostname` field to target another.

### Analytics (the worker aggregates visits from the Redis stream)

    curl -s "http://localhost:8081/links/{code}/analytics?hostname=localhost" \
      -H "X-API-Key: dev-admin-key"
    curl -s "http://localhost:8081/links/{code}/analytics/timeseries?hostname=localhost" \
      -H "X-API-Key: dev-admin-key"
    curl -s "http://localhost:8081/links/{code}/analytics/breakdowns?hostname=localhost&dimension=referrer" \
      -H "X-API-Key: dev-admin-key"

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
| `SHRL_ADMIN_KEY`        | *(required)*                                         | api           | Admin key required by every API endpoint         |
| `SHRL_DATABASE_URL`     | `postgres://shrl:shrl@localhost:5432/shrl`           | api, worker   | PostgreSQL connection string                     |
| `SHRL_REDIS_ADDR`       | `localhost:6379`                                     | all           | Redis address                                    |
| `SHRL_API_ADDR`         | `:8080`                                              | api           | API listen address                               |
| `SHRL_REDIRECTOR_ADDR`  | `:8080`                                              | redirector    | Redirector listen address                        |
| `SHRL_DEFAULT_HOSTNAME` | `localhost`                                          | api           | Hostname used when a request specifies none      |
| `SHRL_RETENTION_DAYS`   | `365`                                                | api, worker   | Analytics retention window (daily rollups)       |
| `SHRL_GEOLITE_LICENSE`  | *(unset)*                                            | worker        | MaxMind license key; enables GeoIP attribution   |
| `SHRL_GEOLITE_DB_PATH`  | `/data/GeoLite2-City.mmdb`                           | worker        | Path to the GeoLite2 City database               |

## API reference

All endpoints require the admin key (`X-API-Key: <key>` or
`Authorization: Bearer <key>`); requests without it get `401`.

| Method | Path                                   | Purpose                                             |
|--------|----------------------------------------|-----------------------------------------------------|
| POST   | `/links`                               | Create a Link (auto or custom Code)                 |
| GET    | `/links`                               | List Links for a Hostname                           |
| GET    | `/links/{code}`                        | Get a Link                                          |
| PATCH  | `/links/{code}`                        | Update a Link's Destination                         |
| POST   | `/links/{code}/disable`                | Disable a Link (redirector returns 404)             |
| POST   | `/links/{code}/enable`                 | Enable a Link                                       |
| DELETE | `/links/{code}`                        | Delete a Link                                       |
| GET    | `/links/{code}/analytics`              | Lifetime and window visit totals                    |
| GET    | `/links/{code}/analytics/timeseries`   | Daily visit buckets in the window, ascending        |
| GET    | `/links/{code}/analytics/breakdowns`   | Top-N dimension values in the window                |

A Link is a JSON object: `hostname`, `code`, `destination`, `disabled`,
`created_at`, `updated_at`.

Query parameters:

- `hostname` (all endpoints) — defaults to `SHRL_DEFAULT_HOSTNAME`.
- `from` / `to` (analytics reads) — `YYYY-MM-DD` bounds; default to the
  retention window.
- `dimension` (breakdowns) — one of the analytics dimensions, default
  `referrer`.
- `limit` (breakdowns) — top-N, default `20`, max `100`.

## Terminology

This project uses a precise domain vocabulary (Link, Code, Hostname,
Destination, Visit, Visitor, Bot, Location, Redirect, Disabled, Delete).
See [`CONTEXT.md`](CONTEXT.md) for definitions and the words to avoid.

## Documentation

Architecture decision records (ADRs) live in [`docs/adr/`](docs/adr/).
