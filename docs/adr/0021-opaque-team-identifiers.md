# Opaque team identifiers — teams carry a random Ref, not their row id

> **Status: accepted** — amends ADR 0009, whose API addressed Teams by
> numeric id.

Team URLs and JSON exposed the sequential `bigint` primary key
(`/teams/{id}`), which enumerates: a guessable id invites probing, and
numeric ids leak instance scale. Every team endpoint already returned
indistinguishable 404s to outsiders, so isolation never depended on the id's
unguessability — the change is defense in depth plus consistency with the
random Link `Code` and Invite `Code` (ADR 0008, ADR 0010). Every Team now
carries a 10-character random `Ref` from the Code alphabet; it is the only
team identifier in URLs and JSON (marshaled as `id`), while the bigint
primary key keeps keying rows and foreign keys. Sequential ids are gone from
team payloads entirely: members are addressed by `username` in membership
endpoints, and `created_by` fields render usernames.

**Consequences:** breaking change to the Auth API's team paths and JSON id
types, accepted pre-release per ADR 0019's precedent (no deployed consumers
worth preserving); existing rows are backfilled with generated Refs on
migrate. Links carry `team_id` as the Team's Ref (null for Personal) and
their `created_by` is a username. Admin user-management endpoints
(`/users/{id}`) keep numeric ids: they are instance-admin surfaces, not Team
scope.

**Rejected alternatives:**

- UUID v4 — standard and collision-free, but 36-character URLs foreign to
  this codebase's short-code style.
- UUID v7 — index-friendly, but encodes the creation timestamp, partially
  defeating the enumeration-privacy goal.
- Replacing the bigint primary key with the Ref — rewrites three tables'
  keys and foreign keys for no external benefit; the database is not the
  attack surface.
