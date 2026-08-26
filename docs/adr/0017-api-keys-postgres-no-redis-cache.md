# API keys stay in Postgres; Redis is rate limiting only

Caching `API Key`s in Redis "forever" to speed validation on the public
`Auth API` (ADR 0016) was considered and rejected.

Key validation is a SHA-256 hash plus a unique-index lookup in Postgres —
already single-digit milliseconds — and only Keys whose hashes an attacker
already knows can be looked up (hashes of random secrets are unguessable), so
a public endpoint cannot hammer Postgres with guesses. A no-TTL credential
copy in Redis — the same Redis that holds the redirector's read path and the
TTL'd Link cache — would make revocation depend on explicit eviction on every
revoke path (per-Key revoke and the password-change bulk revoke), and a
missed eviction would let a revoked Key keep working indefinitely: a security
regression from Postgres-as-source-of-truth. A "forever" entry also has the
opposite lifecycle of the cache it would live beside, and a Redis flush or
LRU eviction would silently drop it, so "validate from Redis" could not be
relied on anyway.

Postgres therefore remains the single source of truth for Key validation;
Redis is used only for TTL'd rate-limit counters on the Auth API. Revocation
is effective immediately, with no cache to invalidate. If profiling ever
shows Key validation to be a real bottleneck, the escape hatch is a cache
with a short TTL plus eviction on every revoke path — never a no-TTL
"forever" entry.
