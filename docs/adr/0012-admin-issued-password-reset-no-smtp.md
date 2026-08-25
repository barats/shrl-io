# Admin-issued password reset with forced change; no SMTP

The stack deliberately has no email infrastructure, so a forgotten password
cannot be recovered by an emailed link. Reset is admin-mediated: an Admin
resets a User's password to a generated value shown once (mirroring account
creation), and the User is forced to change it on their next login before using
the instance — the Admin cannot silently keep knowing the password. The forced
change is a per-User flag; a login with a temp password lands on a change
screen and nothing else is usable until the password is replaced.

Self-service moved to a per-user Account page: a User changes their own
password and manages their own `API Key`s there, while the Settings page
remains admin-only instance administration (Hostnames, Accounts, Teams).
Separating the two keeps "Settings = the instance" and "Account = me" distinct
in both the UI and the domain model.

We rejected email-based reset links (adds SMTP to a stack that deliberately
has none, and self-hosters would rather not run mail), reset without forced
change (the Admin keeps a known password indefinitely, and there is no way to
tell a fresh sign-in from a compromised one), and folding per-user credential
management into the admin Settings page (conflates two subjects and would
expose instance administration to every User).
