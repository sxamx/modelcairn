# Administrative session contract v1

[Español](sesiones-admin-v1.es.md)

## Initialization and storage

- Initial user created only during local bootstrap.
- Minimum password length is 12 characters, with no artificial maximum below 1024
  bytes; processed as UTF-8 and stored in PHC format with Argon2id.
- Uniform login response for a nonexistent user and an incorrect password.
- Per trusted client IP and global rate limits with increasing cooldown (not the
  browser Origin header); never a permanent lock
  that enables trivial denial of service.
- The session ID contains 32 random bytes; the database stores only its hash.
- Login returns a `SessionContext` with administrator, expiration, and CSRF token;
  `/session/me` obtains a new CSRF token after reloading the SPA.

## Cookie and CSRF

The [administrative runtime specification](admin-runtime-v1.md) defines proxy
trust, Origin checks, bounded authentication and the session recovery exception.

- Cookie `mc_session`: `HttpOnly`, `SameSite=Strict`, `Path=/`, no `Domain`.
- `Secure` is mandatory when access uses HTTPS. Outside localhost, onboarding does
  not enable insecure HTTP administration.
- After login or `/session/me`, every authenticated administrative request sends
  the session-bound CSRF token as `X-CSRF-Token`; mutations also strictly verify
  `Origin`. The SPA keeps the token in memory. It rotates on login, on
  `/session/me`, and on privilege changes. To avoid breaking another tab during a
  reload, the previous hash is accepted for at most 60 seconds. Business-resource
  mutations through GET are not accepted. `/session/me` explicitly requires only
  the session cookie plus the runtime's same-origin checks so it can recover CSRF.
- Administrative CORS is disabled by default. A future exception requires an
  exact allowlist, never `*` with credentials.

## Lifetime and revocation

- Inactivity: 30 minutes by default, configurable from 5 minutes to 24 hours.
- Absolute lifetime: 12 hours by default, maximum 7 days.
- ID rotation on login and after any privilege change.
- Logout revokes server-side state and clears the cookie.
- Password reset or change increments `auth_version` and immediately invalidates all sessions.
- Restarting the service does not revive expired or revoked sessions.

## Identity separation

The administrative cookie does not authenticate `/v1/*`. Agent tokens do not
authenticate `/api/v1/admin/*`. Health endpoints do not reveal configuration,
provider names, or internal errors.

## Recovery

`modelcairn admin reset-password` requires local execution with service permissions,
prompts for the new password through TTY/secure stdin, invalidates sessions, and
generates an audit record. It does not print hashes or secrets.
