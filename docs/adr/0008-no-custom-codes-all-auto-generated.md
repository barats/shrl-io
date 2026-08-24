# No custom Codes — every Code is auto-generated

Link creation used to accept an optional user-supplied Code (1–32 chars of
`[A-Za-z0-9_-]`) and the README advertised custom Codes. Nobody — including an
Admin — chooses a Code now: shrl.io generates every one. Codes are 6 characters,
lowercase, drawn from `[a-z0-9]` minus `l`, `o`, `0`, `1` (32 symbols,
unambiguous to hand-type). The create endpoint's `code` field and
`ValidateCustomCode` are removed. Existing Links with legacy custom Codes keep
serving; they are not migrated. Lookup remains exact-match: a Code resolves only
as stored.

We rejected keeping custom Codes (vanity slugs invite squatting and a second code
path) and case-sensitive base62 (Users must hand-type Codes, so case ambiguity is
a real cost now that we control generation).

Scope: backend only for now — the frontend's "Code (optional)" input is
intentionally left as-is until told otherwise.
