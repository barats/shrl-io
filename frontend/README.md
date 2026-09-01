# shrl.io frontend

The SvelteKit admin UI for [shrl.io](https://shrl.io): signs a `User` in,
lets them manage their `Link`s, and renders analytics. It is one service in a
Go + Svelte stack — see the [root README](../README.md) for the full
architecture, and [`CONTEXT.md`](../CONTEXT.md) for the domain vocabulary
(`Link`, `Code`, `Destination`, `Hostname`, `Team`, `Visit`, …) used here.

This is a **private admin surface**: every route except `/login` and the
static assets requires a session (see `src/hooks.server.ts`) and `robots.txt`
denies all bots. There is no public marketing page.

## Stack

- SvelteKit with Svelte 5 runes (`adapter-node`)
- Tailwind CSS 4 + [shadcn-svelte](https://www.shadcn-svelte.com/) (bits-ui)
- Client data access via `src/lib/api.ts`, proxied server-side to the
  Internal API with the session `Token` (ADR 0005)

## Local development

```sh
npm install
npm run dev
```

The dev server proxies API calls to the Internal API. Point it at a running
stack (e.g. `podman compose -f ../dev.compose.yaml up --build`) with:

```sh
SHRL_API_URL=http://localhost:8080 \
SHRL_API_INTERNAL_SECRET=dev-internal-secret \
npm run dev
```

## Configuration

Environment variables (see the *Frontend* section of the root
[README](../README.md) for the full table and defaults):

| Variable                  | Purpose                                        |
|---------------------------|------------------------------------------------|
| `SHRL_API_URL`            | Internal API address the UI proxies to         |
| `SHRL_API_INTERNAL_SECRET`| Shared secret the Internal API demands         |
| `SHRL_DEFAULT_HOSTNAME`   | Hostname pre-selected when creating a Link     |
| `SHRL_SESSION_SECRET`     | HMAC secret for signing UI session cookies     |
| `SHRL_SESSION_TTL`        | UI session cookie lifetime in seconds          |
| `SHRL_COOKIE_SECURE`      | Send the session cookie over TLS only          |

## Building and deploying

```sh
npm run build
npm run preview
```

The production build is a Node server via `adapter-node`; the included
`Dockerfile` runs it on port 3000.

## Project structure

- `src/routes/` — pages and server routes: `/` (Links), `links/[code]`
  (Link detail), `login`, `profile`, `settings`, `teams` and `teams/[id]`
- `src/lib/api.ts` — the typed client for the proxied Internal API
- `src/lib/server/` — session cookie handling and server-side API proxy
- `src/lib/components/` — shared components (charts, QR code, UI kit)
- `src/static/robots.txt` — denies all bots (private instance)

## Checks

```sh
npm run check   # svelte-check
```
