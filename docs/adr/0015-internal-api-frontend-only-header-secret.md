# Internal API is frontend-only, gated by a shared header secret

The UI has always proxied the API server-side (ADR 0005, 0006): the frontend
holds the `Session` cookie, presents the User's `Token` to the api, and the
browser never talks to the api directly. But the api was still published to
the host (compose `8081:8080`), so on a deployed instance it was reachable
from outside, protected only by bearer credentials.

The api now rejects every request that lacks the shared internal header
(`X-Shrl-Internal-Secret`, env `SHRL_API_INTERNAL_SECRET`, held by the api and
mirrored by the frontend). The check applies to **all** routes, including
`/login` and `/logout`: those only ever arrive proxied by the frontend
server, so there is no route an external caller may reach. compose stops
publishing the api port; the service binds the compose-internal network only.
The header secret is defense-in-depth for the network boundary — its job is
"only the frontend can talk to the api," not user auth — while per-User
identity still comes from bearer `Token`s presented by the UI's server. The
secret never reaches the browser.

We rejected mTLS and per-deployment network ACLs (heavier than a self-hosted
single-node compose deployment needs), and keeping the boundary as the port
publish alone (a defense-in-depth secret costs one middleware and survives
misconfiguration such as a port accidentally republished). This completes the
split begun by ADR 0005/0006: the api is now the `Internal API` — `CONTEXT.md`
— and programmatic access moves to the `Auth API` (ADR 0016).
