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
An event recorded on every redirect of a `Link`. Collected from day one; its
aggregation into analytics is a later slice.
_Avoid_: click, hit
