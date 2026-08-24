# Admin-managed Hostname Registry

A Hostname used to be a free-text string any User typed when creating a Link,
with no registry and no admin control. A Hostname is now a first-class resource
that an `Admin` registers in the instance's Hostname Registry; every
authenticated User may create Links under any registered Hostname (a global
allowlist, not per-user grants — grants are deferred to the Teams roadmap).
All Users, including Admins, select a Hostname from the Registry when creating
a Link and never type one. `SHRL_DEFAULT_HOSTNAME` is auto-registered on first
run; registered Hostnames are normalized to lowercase and validated as bare
hosts (no scheme, port, or path). Removing a Hostname only takes it out of the
Registry — existing Links on it keep serving.

We rejected a config/env-seeded Hostname list (no operator path to add one,
every change is a redeploy) and per-user Hostname grants (that is the Teams
feature). The API's `/hostnames` endpoint now returns the full Registry to any
authenticated User; Hostname management (add/remove) is admin-only.

Scope: backend only for now — the frontend is intentionally left as-is until
told otherwise.
