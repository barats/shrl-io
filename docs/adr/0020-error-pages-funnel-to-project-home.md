# Error pages funnel to the hardcoded project home

The redirector has no landing page: short links are its only public surface,
so every error state (404, 429, 500) renders a branded page whose only
outbound link is `Go to shrl.io →` pointing at the hardcoded
`homeURL = "https://shrl.io"`. A dead link — and the bare domain `/` itself —
is treated as acquisition, not as a dead end. The catch-all route serves the
same branded 404 for `/`, `/a/b`, and every other unmatched path, so the
single most-typed URL on the domain funnels into the project instead of
returning raw text.

**Consequences:**

- `homeURL` is a deliberate constant in the redirector, documented as a
  funnel; it is not a stray vendor URL to be stripped in review.
- Error pages never auto-forward (no refresh meta, no countdown copy): the
  funnel is a deliberate click, and dead-link traffic keeps its referer.
- `/code/` is canonicalized to `/code` with a 301 (preserving the query
  string) before the rate-limit gate, so both variants share one
  `host:code` bucket and the redirect never double-serves.

**Rejected alternatives:**

- Leaving the redirector's dead ends as raw `http.NotFound` text — leaks
  acquisition traffic at the front door.
- Auto-forwarding to the home after a countdown — muddies attribution and
  reads as a redirect trap rather than a page.
