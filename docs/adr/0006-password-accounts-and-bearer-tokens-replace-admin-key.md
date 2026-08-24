# Password accounts with per-user bearer tokens replace the shared admin key

Users now sign in with a username and password; passwords are stored only as
bcrypt hashes. The first account is an `Admin` provisioned on first run when
the users table is empty: username `admin` (`SHRL_ADMIN_USERNAME` override),
password from `SHRL_ADMIN_PASSWORD` or a crypto-random value printed once to
the server logs. Admins create accounts; there is no self-registration. Login
issues an opaque bearer `Token` stored as a SHA-256 hash in Postgres and
revocable at logout. Every API endpoint requires the token and scopes to the
authenticated `User`; each `Link` records its `Creator`, and a User sees only
their own Links. `SHRL_ADMIN_KEY` is removed.

This supersedes ADR 0005's "no accounts, single Operator" stance — the UI
session-cookie + server-side proxy decision stands. We rejected keeping the
shared admin key (it cannot distinguish Users or support per-creator scoping)
and JWTs (statelessness buys nothing here, and revocation is harder).
