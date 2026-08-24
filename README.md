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

---

## Overview

**shrl.io** is a high-performance, self-hosted URL shortener and traffic analyzer designed for teams and individuals who want full control over their link data. Built with a microservices architecture, it offers fast redirects, real-time analytics, team collaboration, and enterprise-grade security.

### Why shrl.io?

- 🔒 **Privacy-First**: Your data stays on your infrastructure
- ⚡ **Blazing Fast**: Redis-backed redirects with sub-millisecond response times
- 📊 **Rich Analytics**: Track visits, visitors, devices, locations, and referrers
- 👥 **Team-Friendly**: Create teams, manage members, and collaborate on links
- 🛡️ **Secure**: Rate limiting, URL validation, and open redirect protection


## Features

### Core Features

| Feature | Description |
|---------|-------------|
| **URL Shortening** | Create short links with custom codes and multiple hostnames |
| **Link Management** | Edit, disable, or delete links; organize with names and tags |
| **Analytics Dashboard** | Visualize traffic with charts, maps, and detailed statistics |
| **Team Collaboration** | Create teams, invite members, and share link management |
| **Role-Based Access** | Admin and user roles with granular permissions |
| **QR Code Generation** | Auto-generate QR codes for every short link with download option |

### Analytics & Tracking

- 📈 **Visit Statistics**: Total visits, unique visitors, click-through rates
- 🗺️ **Geographic Data**: Country-level heat maps
- 📱 **Device Analytics**: Device type, OS, and browser breakdown
- 🔗 **Referrer Tracking**: See where your traffic comes from
- 🏷️ **UTM Support**: Track campaign parameters automatically

### Security Features

| Feature | Implementation |
|---------|----------------|
| **Rate Limiting** | Token bucket algorithm (10 req/s default, strict on auth) |
| **Open Redirect Protection** | Blocks localhost, private IPs, and non-HTTP schemes |
| **URL Validation** | Prevents malicious URLs (file://, javascript:, etc.) |
| **Session Management** | HTTP-only cookies with configurable expiration |
| **Password Security** | bcrypt hashing with salt |
| **Secrets Masking** | Sensitive env vars masked in logs |

### Technical Highlights

- 🔄 **Async Analytics**: Redis Streams for non-blocking visit tracking
- 💾 **Multi-Level Caching**: Redis cache with PostgreSQL persistence
- 🏎️ **Race-Condition Free**: Atomic increments for visit counts
- 📱 **Responsive UI**: Mobile-friendly SvelteKit frontend
- 🔌 **API-First**: RESTful API for all operations

### Data Flow

1. **Link Creation**: API writes to PostgreSQL → Cache in Redis
2. **Redirect**: Redirector reads from Redis → Logs to Redis Stream
3. **Analytics**: Worker consumes stream → Updates PostgreSQL atomically

## Development

Prerequisites: Go 1.25+, podman (with podman-compose).

### Run the full stack

    SHRL_ADMIN_KEY=dev-admin-key podman compose up --build

Services:

- Redirector: http://localhost:8080/{code}
- API: http://localhost:8081

### Create a link

    curl -X POST http://localhost:8081/links \
      -H "X-API-Key: dev-admin-key" \
      -H "Content-Type: application/json" \
      -d '{"hostname":"localhost","destination":"https://example.com"}'

Then visit `http://localhost:8080/{code}` — you get a 302 to the destination.

The API `hostname` defaults to `SHRL_DEFAULT_HOSTNAME` (`localhost`), or pass
`?hostname=` / a `hostname` field to target another.

### Analytics (worker aggregates visits from the Redis stream)

    curl -s "http://localhost:8081/links/{code}/analytics?hostname=localhost" \
      -H "X-API-Key: dev-admin-key"
    curl -s "http://localhost:8081/links/{code}/analytics/timeseries?hostname=localhost" \
      -H "X-API-Key: dev-admin-key"
    curl -s "http://localhost:8081/links/{code}/analytics/breakdowns?hostname=localhost&dimension=referrer" \
      -H "X-API-Key: dev-admin-key"

Dimensions: `referrer`, `device`, `os`, `browser`. Bots and link-preview
unfurlers are excluded. Rollups are pruned after `SHRL_RETENTION_DAYS`
(default 365); the lifetime visit total is never pruned.
