# Globally unique Codes — Hostname is a display attribute

A Link used to be identified by the pair `(Hostname, Code)`: Codes were unique
per Hostname, and every management query — list, get, update, delete, disable,
analytics — required a `hostname` query parameter to scope it. That made
Hostname a query dimension: the UI needed a per-hostname dropdown filter, and
link tables showed only the Code. A Code is now unique across every Hostname on
the instance: it alone identifies a Link, so no management query needs a
Hostname. Hostname remains a required attribute the `User` selects at creation
(picked from the admin-managed Registry, pre-selected to
`SHRL_DEFAULT_HOSTNAME`) and is carried on the Link for display and to build the
short URL; lists show the concatenated `hostname/code`.

**Consequences:**

- The Link primary key becomes `code` (unique instance-wide); the analytics
  rollup tables and the Redis cache drop Hostname from identity. The redirector
  still resolves `(host, code)`: a Code serves only on the Hostname it was
  created under.
- The UI's Links and Team pages show one table per scope labeled with
  `hostname/code` and no hostname filter; detail pages fetch by Code alone and
  read Hostname from the Link object.
- The `hostname` query parameter is removed from every Auth API read/manage
  endpoint (a breaking change for scripts); `POST` still takes `hostname` in
  the body as the creation target.
- Existing data is cleared rather than migrated: per-Hostname uniqueness can
  produce Code collisions across Hostnames, and there is no deployed data worth
  preserving, so the migration resets the tables instead of resolving
  collisions.

**Rejected alternatives:**

- Keeping per-Hostname Codes — Hostname stays a query dimension and the UI
  keeps its filter; exactly the shape this supersedes.
- Serving any Code on any Hostname at redirect time — would let a foreign
  domain pointed at the instance serve another user's Codes.
