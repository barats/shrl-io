# Auth API: public /v1, API-key-authenticated link management

Programmatic access used to mean pointing an `API Key` at the same api the UI
uses (ADR 0011). With the api now frontend-only (ADR 0015), scripts and CI
need a public surface of their own, authenticated by the caller's `API Key`.

A new `auth` binary — same Go module, sharing `internal/` packages with the
existing services — is deployed as its own compose service with its own
published port and hostname (e.g. `api.shrl.io`). It exposes `/v1` endpoints
authenticated by an `API Key` on **every** request
(`Authorization: Bearer`), validated from Postgres. The surface is Link
management minus deletion, plus analytics, for both `Personal Link`s and
`Team Link`s: team create/list under `/teams/{id}/links`, single-Link
get/update/disable/enable and analytics on the flat `/links/{code}` routes,
with supporting reads `GET /hostnames`, `GET /teams`, `GET /teams/{id}`.
Permissions mirror the UI exactly — a Team Member reads Team Links, the
`Creator` (while a member) or a `Team Owner` manages them. Delete, admin
endpoints, `API Key` management, and login/logout are not exposed. Every
request is rate-limited from Redis (sliding window, env-tunable): per-IP
60/min, per-key reads 300/min, per-key writes 30/min, failed-key 10/min per
IP; excess returns `429` with `Retry-After`. Link create/update/disable logic
is shared with the Internal API so both write the Redis link cache
identically — the redirector reads Redis only.

This supersedes ADR 0011's "unscoped" stance. A Key is no longer
"the same powers as its owner's Login": its scope is now everything-except-
deletion on the `Auth API` — `CONTEXT.md`: API Key.

We rejected a token exchange (turn the Key into a short-lived token once,
then send that) in favor of Key-per-request — no session state on the public
side, and a leaked Key is revocable anyway; a flat "all my links" list instead
of mirroring internal addressing (the internal routes are the model scripts
already know, and Team scoping stays explicit); exposing admin or `API Key`
management publicly; and a second mux inside the api binary (the repo is
one-binary-per-concern, and a port split inside one process makes the
internal-only boundary from ADR 0015 harder to hold).
