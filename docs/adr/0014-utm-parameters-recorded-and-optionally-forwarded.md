# UTM parameters: first-party dimensions plus optional per-link forwarding

UTM parameters (`utm_source`, `utm_medium`, `utm_campaign`, `utm_term`,
`utm_content`, `utm_id`) arrive on short URLs when a marketer tags a share;
today the redirector drops the short URL's query string entirely and analytics
knows only the referrer. We record all six parameters in shrl.io's own
analytics as Breakdown dimensions, so the dashboard shows which campaign drove
visits, and optionally append them to the Destination on redirect, so the
destination's own analytics credits the campaign too.

The redirector recognizes the six standard `utm_*` parameters on the short URL
on every redirect and includes them in the Visit event; the worker buckets them
into six new Breakdown dimensions (`utm_source`, `utm_medium`, `utm_campaign`,
`utm_term`, `utm_content`, `utm_id`). Values are truncated to 128 characters,
and absent or empty parameters bucket as "unknown" — the same convention as
every other dimension. This stays consistent with the aggregates-only decision
(ADR-0003) and the no-raw-IPs boundary (ADR-0004): UTM values are aggregate
dimension values, never raw events or identities.

Forwarding to the Destination is opt-in per Link: a `forward_utm` flag, default
off, set at creation and editable like Remark. When it is on, only the six
recognized `utm_*` parameters are appended to the Destination — an incoming
value overrides a same-named parameter already on the Destination, the
Destination's other query parameters are preserved, and empty values are
skipped. When it is off, redirect behavior is unchanged (the short URL's query
string is dropped), though UTM parameters are still recorded in first-party
analytics. The redirector needs the flag at serve time, so the Link cache value
carries it; the Redis-only read path (ADR-0001) is unchanged.

"Campaign" is the value of the `utm_campaign` parameter — a dimension value,
not a first-class entity. It does not group Links or own aggregates; a
campaign-level view spanning links is a separate future feature.

We rejected link-configured static campaigns (the Link owns a campaign that is
appended on every redirect regardless of the visitor's query string): that
makes Campaign a property of the Link rather than of the Visit, needs new
storage and UI on the Link, and is a different feature we can build on this
foundation later. We rejected forwarding arbitrary query parameters (an open,
unscoped pass-through that would leak unknown data to destinations) in favor of
the fixed six. We rejected dropping `utm_term`/`utm_content` to bound
cardinality: truncation is enough, and keeping all six preserves the promise of
full UTM tracking.

Consequences: the Link gains a `forward_utm` field (API create/PATCH and the
frontend form), the Link cache value grows to carry it, `validDimensions` and
the analytics UI dimension picker grow by six, and forwarding a visitor's
carried UTM values to the destination is an explicit per-Link choice — the
visitor navigates there regardless, so we expose nothing the destination would
not otherwise receive.
