# AgentToken lifecycle v1

[Español](agent-token-lifecycle-v1.es.md)

Status: core and administrative interface implemented in Milestone 3; data API
integration remains part of the vertical router.

## Purpose and separation

An `AgentToken` authenticates data API clients and limits the routes they may use.
It cannot authenticate the administrative API, and an administrative cookie cannot
replace it. Declarative configuration owns route permissions, expiry, and enabled
state; the bearer value is never part of configuration, exports, or backups.

In v1, `enabled: false` irreversibly revokes that resource identity (`uid`); it is
not a pause. Deleting and recreating the same name creates a new identity and never
revives the old token.

## Format and persistence

Issuance generates 32 CSPRNG bytes and returns
`mc_at_v1_<unpadded-base64url>`. SQLite stores SHA-256 of the complete bearer, a
non-sensitive display prefix, `issued_at`, `expires_at`, and `revoked_at`, never the
bearer. The prefix contains the format tag plus eight random characters.

The fast hash is appropriate because the bearer has 256 random bits. Verifier
comparison is constant-time. Bearers, hashes, Authorization headers, and request
bodies never enter logs, audit, metrics, or errors.

## Administrative issuance and revocation

`POST /api/v1/admin/agent-tokens/{name}/issue` requires the administrative
transport/origin boundary, session, and CSRF. Under the `SecretStore` lock and one
transaction it reauthorizes the session, loads the resource, requires an enabled,
unexpired, never-issued and never-revoked identity, conditionally stores verifier
metadata, audits `agent_token.issue`, and commits. Concurrent issuance has one
winner. The `201` response is `no-store` and returns the bearer exactly once. A lost
response cannot be recovered; revoke the identity and create another. This endpoint
does not use `If-Match`: its one-time lifecycle condition is separate from the
declarative resource version.

`POST /api/v1/admin/agent-tokens/{name}/revoke` uses the same boundary and
transaction. Missing resources return 404. It sets `revoked_at` even before issue;
repeat calls return 204 without changing the original instant and only the first
transition is audited. No `If-Match` is needed because revocation is monotonic and
idempotent. Generic CRUD disabling has the same irreversible meaning.

## Safe administrative status

Administrative `AgentToken` representations may add `tokenStatus` with `state`
(`unissued`, `active`, `expired`, or `revoked`) and optional `prefix`, `issuedAt`,
`expiresAt`, and `revokedAt`. They never expose `verifier_sha256`. Revoked takes
precedence over expired; expiry is checked on every request.

## Data authentication

The data API accepts exactly one `Authorization: Bearer <token>` header. It checks
bounded canonical syntax before SQLite, hashes the token, and returns the same 401
for unknown, revoked, expired, disabled, or deleted identities. It then checks the
resolved route against `allowedRouteRefs`; missing permission returns 403. Identity
and route are captured before contacting a provider. Automatic rotation, multiple
bearers per identity, and non-route permissions are outside v1.

## Grouped acceptance

- canonical random bearer and one-time delivery;
- one concurrent issue winner and audit-failure rollback;
- no bearer in SQLite, logs, errors, DTOs, or exports;
- irreversible, idempotent revocation before or after issue;
- expiry and deletion reject new calls;
- uniform 401 for invalid lifecycle state and 403 for a disallowed route;
- complete separation between administrative sessions and agent bearers.
