# UI authenticates via an Operator Session cookie and proxies the API server-side

> **Status: partially superseded by ADR 0006** — the session-cookie + proxy
> decision stands; the "no accounts, single Operator holds the admin key" part
> is replaced by password accounts and per-user bearer tokens.

The SvelteKit admin UI authenticates the Operator with a login screen: the UI
server validates the submitted admin key against its env copy, issues an
HttpOnly session cookie, and proxies every API call to the api service with the
key from env. There are no accounts or roles yet — the single Operator is the
holder of `SHRL_ADMIN_KEY`, and "User" is deliberately not part of the model.
We rejected holding the admin key in the browser (it would sit in page JS and
browser storage — the weakest option for a tool strangers deploy) and a
wide-open proxy with no UI auth boundary (anyone who could reach the UI could
manage Links).
