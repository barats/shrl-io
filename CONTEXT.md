# shrl.io

Self-hosted URL shortener: turns a destination URL into a short, shareable URL
under your own hostname and redirects visitors there at sub-millisecond speed.

## Language

**Link**:
The core entity: a `Code` on a `Hostname` that redirects to a `Destination`,
created by a `Creator`. A Link belongs to exactly one Team or is Personal (no
Team); its Team is fixed even if its Creator leaves or is removed from that
Team.
_Avoid_: Short URL, shortlink, tiny URL

**Code**:
The identifier segment of a Link's short URL, e.g. `abc123` in `shrl.io/abc123`.
Together with its `Hostname`, uniquely identifies the `Link`. Always generated
by shrl.io; a `User` never chooses a `Code`. Unique per `Hostname`; never
automatically reused after a `Link` is deleted.
_Avoid_: slug, key

**Hostname**:
A domain an `Admin` registers in the Hostname Registry, on which `Link`s are
served; one half of a `Link`'s identity. Every `User` may create `Link`s under
any registered `Hostname`.
_Avoid_: domain, brand

**Hostname Registry**:
The set of `Hostname`s an `Admin` has registered; the universe a `User` may
select from when creating a `Link`.
_Avoid_: hostname list, allowed domains

**Destination**:
The URL a `Link` redirects to.
_Avoid_: target, long URL, original URL

**Remark**:
The optional note a `Creator` writes on a `Link` to remember what it does.
Shown on the list and detail pages; editable after creation; never part of
the short URL or the redirect.
_Avoid_: note, description, memo, comment

**Redirect**:
The operation of sending a visitor from a `Link`'s short URL to its `Destination`.
_Avoid_: forward, bounce

**Disabled**:
A `Link` state in which the redirector returns 404 instead of redirecting.
Reversible; the `Link` and its data are preserved.
_Avoid_: inactive, off

**Delete**:
The permanent removal of a `Link`. Its `Code` is never automatically reused.
_Avoid_: remove, purge

**Visit**:
An event recorded on every redirect of a `Link`.
_Avoid_: click, hit

**Visitor**:
A unique `(Link, day, IP + user-agent)` bucket counted from `Visit` events.
Identity is stored as a hash, never as raw IPs.
_Avoid_: unique IP, device

**Bot**:
A `Visit` whose user-agent matches a known crawler or link-preview unfurler
(e.g. Googlebot, Slackbot, WhatsApp). Filtered out of analytics rollups at
aggregation time.
_Avoid_: crawler, spider, robot

**Location**:
The geographic attribution of a `Visitor` — `country`, `region`, and `city`,
derived from the visitor's IP at aggregation time. The IP itself is never
stored; only the `Location` and aggregate counts are persisted.
_Avoid_: geo, IP address

**User**:
A person with an account who signs in and manages Links. Every Link has a
`Creator`; a User manages their own Links and, as a Team Member, sees the
Links of any Team they belong to (read-only).
_Avoid_: account holder

**Admin**:
A `User` with the privilege to create accounts, register and remove
`Hostname`s, create Teams, and manage the instance. The first account
provisioned on first run is an `Admin`. An Admin may be a Team Member or
Team Owner of a Team, but holds no implicit role in a Team unless it was
granted.
_Avoid_: superuser, root

**Creator**:
The `User` who created a `Link`; recorded on the Link so dashboards and APIs
can scope to the current user.
_Avoid_: author

**Personal Link**:
A `Link` with no `Team`, visible and manageable only by its `Creator`. The
scope for Users who belong to no Team or choose not to assign a Link to a Team.
_Avoid_: private link, individual link

**Team**:
A group of `User`s created by an `Admin`. The `Team` is the visibility boundary
for Links assigned to it: every `Team Member` sees all of the `Team`'s Links
and their related data. Membership is many-to-many; a `User` may belong to
several Teams.
_Avoid_: group, workspace, organization

**Team Owner**:
A `Team Member` with the privilege to manage the `Team`'s membership: add and
remove members, and promote or demote other `Team Owner`s. Being a Team Owner
grants no authority over other Teams or over the instance; an `Admin` holds no
implicit Team Owner role.
_Avoid_: owner (alone), team admin, admin

**Team Member**:
A `User` who belongs to a `Team`. Sees the `Team`'s Links and their related
data (read-only) and manages only their own Links. Distinct from `Creator`,
which is a per-Link role.
_Avoid_: member (alone), contributor

**Password**:
The secret a `User` signs in with. Stored only as a bcrypt hash; the first-run
`Admin` password is generated randomly and shown once in the server logs
(override with `SHRL_ADMIN_PASSWORD`).
_Avoid_: passphrase, key

**Token**:
The bearer credential a `User` receives at Login and presents on every API
request (`Authorization: Bearer`). Stored as a SHA-256 hash and revocable at
Logout.
_Avoid_: API key, secret

**Session**:
An authenticated browser context for a `User` in the UI, established by a
Login and carried as an HttpOnly cookie; the UI's server holds the User's
`Token` and proxies API calls on their behalf. Distinct from the API's
per-request `Token` auth.
_Avoid_: login state
