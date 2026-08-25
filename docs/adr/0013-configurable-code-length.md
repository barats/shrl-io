# Configurable Code Length replaces the fixed six-character Code

ADR 0008 fixed every auto-generated Code at 6 characters. Operators now want
control over that knob, so the exact Code Length is a per-instance setting an
Admin changes in Settings, defaulting to 6 and validated to 4-12 characters.
It is the instance's first runtime-configurable setting: the value lives in a
settings store, seeded once at bootstrap from `SHRL_CODE_LENGTH` (or 6) and
authoritative in the database afterward, so a live admin edit persists across
restarts. Capacity per Hostname is 32^N (the unambiguous 32-symbol alphabet),
so 4 (~1M) suits a small team and 12 (~1e18) is far beyond any real need.
Existing Codes are unaffected: the length applies only to newly generated
Codes, and the redirector's exact-match lookup never cares about length.

We rejected variable-length generation (grow on collision, like legacy
shorteners — inconsistent lengths, more complexity, and the operator asked
for control, not auto-growth), a min+max range (flexibility with no real
demand), and env-only configuration (a Settings-page control that cannot
persist would be a lie; the DB row is the source of truth after first run).
