# Security policy

shrl.io is pre-1.0 software. Treat deployments as beta: keep them updated,
pin versions instead of floating `:latest`, and expect the security posture
to keep hardening until 1.0.

## Supported versions

Only the latest release receives security fixes until 1.0. There are no
backports to older releases.

| Version | Supported |
| --- | --- |
| latest release | yes |
| anything older | no |

## Reporting a vulnerability

Do not open public issues for security problems.

Use GitHub's private vulnerability reporting: the **Report a vulnerability**
button under the **Security** tab of this repository.

Please include:

- the affected version (or commit) and how you run it (compose, binaries);
- the component: api, auth, redirector, worker, or frontend;
- a description and, if possible, a minimal reproduction.

You will receive an initial response within a few days. Fixes ship in the
next release; anything exploitable remotely without authentication gets an
out-of-band patch first.

## Hardening a deployment

- Serve the redirector and frontend behind an HTTPS reverse proxy and set
  `SHRL_COOKIE_SECURE=true`.
- Generate `SHRL_API_INTERNAL_SECRET` and `SHRL_SESSION_SECRET` with
  `openssl rand -hex 32`; never reuse values from another deployment.
- Keep PostgreSQL, Redis, and the Internal API off the public internet.
  The production compose file publishes only the redirector (:8080), the
  Auth API (:8083), and the frontend (:8082).
- Rotate API Keys when a machine or script might have leaked them; keys
  do not expire on their own (yet).
- The first-run admin password is generated and printed once to the api
  service logs unless `SHRL_ADMIN_PASSWORD` is set. Change it promptly.

See the [Security section of the README](README.md#security) for the
product's security model and the current pre-1.0 limitations.
