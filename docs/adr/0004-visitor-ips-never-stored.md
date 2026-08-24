# Visitor IPs are never stored; analytics keeps only derived location

Analytics never persists a visitor's IP address. The worker attributes each
visit to a Location (country, region, city) via the offline MaxMind GeoLite2
database at aggregation time, then stores only the location strings and the
aggregate counts. The raw IP exists transiently in the capped Redis visits
stream and as a one-way ip+user-agent hash in the 48-hour dedup set; nothing
IP-derived reaches Postgres.

This is a deliberate privacy boundary, not an accident of the current schema:
future code must not add IP persistence. We rejected resolving location via an
external HTTP API (that would send every visitor IP to a third party and
contradict the self-hosted, privacy-first positioning) in favor of an offline
.mmdb lookup in the worker. The worker auto-downloads and refreshes the
database when a license key is configured; without one, geo attribution is
disabled and locations are "unknown". Location granularity is best-effort —
GeoLite2's free region/city coverage varies by country — and unresolved IPs
are bucketed as "unknown".
