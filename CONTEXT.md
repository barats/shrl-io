# shrl.io

Self-hosted URL shortener: turns a destination URL into a short, shareable URL
under your own hostname and redirects visitors there at sub-millisecond speed.

## Language

**Link**:
The core entity: a `Code` on a `Hostname` that redirects to a `Destination`.
_Avoid_: Short URL, shortlink, tiny URL

**Code**:
The identifier segment of a Link's short URL, e.g. `abc123` in `shrl.io/abc123`.
Together with its `Hostname`, uniquely identifies the `Link`. Case-sensitive;
unique per `Hostname`; never automatically reused after a `Link` is deleted.
_Avoid_: slug, key

**Hostname**:
The domain that serves a `Link`; one half of a `Link`'s identity.
_Avoid_: domain, brand

**Destination**:
The URL a `Link` redirects to.
_Avoid_: target, long URL, original URL

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
