# Per-user API Keys: long-lived, named, revocable credentials

Programmatic access to the API previously meant a login `Token`, which expires
with the session TTL and is revoked at Logout — the wrong shape for a script or
CI job that runs for months. A User can now create `API Key`s: long-lived,
named bearer credentials (e.g. "ci", "backup script") presented as
`Authorization: Bearer` exactly like a Token, stored as SHA-256 hashes, shown
in full only at creation, and revocable individually. Keys never expire;
revoking is explicit. Keys are unscoped for now — a Key grants the same powers
as its owner's `Login` — with scoping deferred until a concrete need appears.
A password change is treated as a security event: it revokes all other login
`Token`s and all of the User's `API Key`s, forcing a fresh login and
re-creation.

We rejected making login Tokens simply long-lived (a session credential that
survives Logout is not a session credential; revocation and hygiene become
ambiguous), expiring Keys (the point of a Key is to outlive sessions; an expiry
would push scripts back to periodic re-auth), and scoped Keys now (read-only or
per-hostname grants are a real feature, but gold-plating until someone asks).
Recording Keys as a distinct entity — `CONTEXT.md`: API Key — rather than a
Token variant keeps the Login lifecycle honest.
