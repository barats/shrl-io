# Redirector read path is Redis-only, with write-through and a cache warmer

The redirector (the horizontally-scaling hot path) never touches Postgres. All
link lookups hit Redis; the API writes through to Redis on every link
create/update/disable/delete, and a cache warmer loads the full code→URL
mapping into Redis at boot and periodically. A Redis miss therefore means an
unknown or disabled link and returns 404.

We rejected Redis-only with no warmer, which false-404s valid links after
eviction or a cold cache, and falling back to Postgres on a miss, which
re-couples the redirect path to the database and defeats the performance
isolation that motivated separate services in the first place.
