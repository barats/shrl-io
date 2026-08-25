# Membership via single-use Invite Codes; only Admins add members directly

ADR 0009 shipped Teams with Team Owners adding members directly by username and
explicitly rejected an invite-and-accept flow. That rejection is now reversed:
a Team Owner's only membership action is generating a single-use Invite Code
for the Team; a User joins by entering an Invite Code, and only an Admin adds
members directly.

Membership has two paths. An Admin adds an existing User by username (the
existing `POST /teams/{id}/members` endpoint, now admin-only — the Admin is
the only role with a view of every account). A Team Owner generates a single-use,
non-expiring Invite Code for their Team (`POST /teams/{id}/invites`), shares it
out of band, and may revoke outstanding codes; a User joins by presenting a
code (`POST /teams/join`), which is consumed on first use and never reusable.
Deleting a User removes their memberships and their Personal Links; Team Links
they created stay with the Team (the fixed-team rule), leaving `created_by` as
a dangling id.

We rejected multi-use codes (a leaked code admits outsiders until noticed),
time-limited codes (clock + pruning machinery for marginal gain, and slow
joiners would need re-issued codes), and letting Team Owners keep direct
username-add (owners cannot enumerate accounts — `GET /users` is admin-only —
so a username picker was unusable for them anyway). We also rejected keeping
the ADR 0009 rejection: an owner-generated-code flow is precisely the
self-hosted shape the operator wants, and single-use codes sidestep the
acceptance-state complexity that motivated the original rejection.
