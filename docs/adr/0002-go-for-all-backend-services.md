# Go for all backend services, SvelteKit for the UI

The three backend services (redirector, API, worker) are all written in Go;
the admin UI is SvelteKit. Go gives the horizontally-scaling redirector a
single static binary and sub-millisecond redirects, and one language across
all backend services keeps the Link model, code validation, and destination
validation in a single shared package.

We rejected an all-TypeScript stack (SvelteKit API + Node redirector + Node
worker), which would share one language across the whole stack including the
UI, but would lean on Node for the hot-path redirector that motivated
separate services in the first place.
