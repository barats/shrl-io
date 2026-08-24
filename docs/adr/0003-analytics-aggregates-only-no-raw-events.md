# Analytics stores only pre-aggregated daily rollups, never raw events

The analytics slice keeps no raw visit events in Postgres. The worker consumes
the `visits` Redis stream and upserts per-link daily rollups (`visits`,
`unique_visitors`), dimension breakdowns (referrer, device, os, browser), and a
per-link lifetime counter. Unique visitors are deduplicated in Redis via a
hashed IP+user-agent set; the visits counter is at-least-once (a crash between
apply and ack can double-count one batch). Raw events older than the capped
1M-event stream window are gone.

Retention: rollups and breakdowns older than `SHRL_RETENTION_DAYS` (default
365) are pruned nightly; the lifetime counter is never pruned, so the "total
visits" headline stays accurate for the life of a link.

We rejected storing raw events and aggregating on query (slow at scale, and
defeats the worker's purpose), exactly-once watermark machinery (unwarranted
at self-hosted scale), and keeping all history forever (the operator chose a
bounded footprint). The consequence is that granular history is permanently
lost after aggregation and pruning — which is why the lifetime counter exists.
