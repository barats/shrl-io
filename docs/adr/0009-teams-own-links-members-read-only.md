# Teams own Links; members read them, creators and team owners manage them

> **Status: partially superseded by ADR 0010** — the Team-ownership and
> read-only-member model stands; the membership mechanism changes: Team Owners
> now generate single-use Invite Codes instead of adding members by username,
> and direct add is admin-only.

Shrl.io gains a Team: a group of Users created by an Admin, and the ownership
boundary for Links. A Link belongs to exactly one Team or is Personal (no
Team); it is created in that context and can never be transferred — when a
member leaves or is removed, their Links stay with the Team. Users may belong
to many Teams (many-to-many). Every Team Member sees all of the Team's Links
and their analytics read-only; only the Link's Creator or a Team Owner may
edit, disable, or delete it, so a Team Owner can steward Links whose creator
has left.

Admins create Teams and become their first Team Owner; they hold no implicit
owner role in any other Team, but may delete any Team as instance housekeeping,
at which point the Team's Links revert to Personal. Team Owners manage
membership — adding existing Users directly (account creation stays
admin-only), removing members, and promoting or demoting owners — subject to a
last-owner rule: a Team always keeps at least one Owner. The API addresses
Teams by numeric id with nested resources (`POST /teams/{id}/links`,
`GET /teams/{id}/links`, membership under `POST/PATCH/DELETE
/teams/{id}/members/...`). The public redirector is unchanged: short URLs
still redirect for anyone; the Team only gates the management API, not the
redirect.

We rejected link transfer between Teams (the space a Link is born in is its
only home — avoids reassignment bugs and surprise data movement), read-write
access for plain members (read-only keeps accidental damage out of shared
Links; stewards are explicit), and an invite-and-accept membership flow
(superseded by ADR 0010: membership now runs on single-use Invite Codes).
Joining a Team never moves existing Personal Links.
