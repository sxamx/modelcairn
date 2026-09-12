# Administrative runtime v1

[Español](admin-runtime-v1.es.md)

Status: implemented specification; final milestone verification pending.
This refines the existing session contract and ADR-0005.

## Transport and deployment

An explicit canonical `publicOrigin` identifies the browser-facing scheme, host,
and port. Reject credentials, paths other than `/`, queries, fragments and wildcard
hosts. Normalize default ports and compare parsed origins exactly, not by suffix.
Validate request Host against that origin. Never derive it from request headers.

HTTPS can terminate directly or at a reverse proxy. With proxy termination, only
connections from configured `trustedProxyCidrs` can reach administrative handlers;
the proxy must overwrite incoming forwarding headers and supply a single
`X-Forwarded-Proto: https`. Reject malformed or conflicting forwarding metadata.
Use one configured proxy hop in v1; multi-hop chains require a later explicit
contract. Tailscale is one deployment option, not a protocol dependency.

Client IP defaults to the socket peer. With a trusted single-hop proxy, accept one
syntactically valid `X-Forwarded-For` IP; reject lists. Do not infer trust merely
from a private address. HTTP development mode requires both a loopback listener
and a loopback public origin. HTTPS sessions always set Secure on the cookie.

## Browser boundary

Require exact Origin on login and all mutating administrative requests, including
logout. Missing, `null`, duplicate or foreign origins fail with 403. CLI online
requests supply the configured origin and use the same session/CSRF flow.

`GET /session/me` is the explicit CSRF-token recovery exception: require a valid
session, reject a supplied foreign Origin, and require either matching Origin or
`Sec-Fetch-Site: same-origin`. Return `Cache-Control: no-store` on all identity and
administrative responses. Do not enable CORS. This GET may rotate security state;
GET must not modify business resources.

Other authenticated requests require a session-bound CSRF token. Store hashes of
32-byte random CSRF values; compare in constant time. Retain exactly one previous
hash for at most 60 seconds. Three rapid reloads may displace an older tab's token;
on a CSRF rejection a client fetches `/session/me` and retries once. Rejected CSRF
requests perform no business mutation. Expired/revoked sessions return 401.

## Bounded authentication

Creation accepts 12 Unicode characters through 1024 UTF-8 bytes, without trimming
or normalization. Password input never appears in argv, logs, audit or errors.
Use ADR-0005 Argon2id minimums, a fresh 16-byte salt and a 32-byte hash. Reject
unsupported PHC versions, malformed encoding and out-of-policy parameters before
allocation. Initially support m=19456..65536 KiB, t=2..6, p=1; calibration must
stay within these bounds. Stored parameters outside policy fail closed.

Proposed configurable defaults, to be validated on the representative VM:

- One concurrent password derivation, including dummy verification; no waiting
  queue. Busy authentication returns 429 and Retry-After.
- Global token bucket: 30 attempts/minute, burst 5. Per trusted client IP:
  5 attempts/minute, burst 3. Both run before password derivation.
- At most 1024 client entries; expire after 15 minutes idle. When full, reject new
  client entries until expiry rather than evict active limits.
- Failed authentication imposes a per-client cooldown of 1, 2, 4, 8, 16, then
  30 seconds. Return Retry-After without keeping a sleeping request. Success clears
  cooldown but does not refill buckets. A restart resets this in-memory state;
  deployment perimeter limits are needed if restart-persistent throttling is desired.

Unknown users perform a dummy derivation with the current policy and return the
same status/body as an incorrect password. This is not a promise of identical
wall-clock timing. Bound login JSON at 16 KiB and reject duplicate/unknown fields,
invalid UTF-8 and non-JSON media types before processing credentials.

## Local identity and persistence

Bootstrap requires exclusive installation ownership, succeeds only if no admin
exists, and never overwrites an existing administrator. Reset requires local
service permissions and a stopped service, matching the offline lock contract.
Password replacement, auth_version increment, session invalidation and success
audit commit together. No HTTP unauthenticated bootstrap/reset endpoint exists.

Recheck auth_version when creating a session after password verification: hashing
occurs outside a write transaction and must not race a password reset. Every
authorized request rechecks revocation and expiration; rejected requests do not
extend inactivity. Store absolute expiry, last_seen and the idle duration captured
when issuing each session; compute effective expiry as the earlier of absolute
expiry and last_seen plus that captured duration. Settings changes affect new
sessions only, never revive expired sessions or extend existing session lifetimes.
Add the captured duration through a new migration; revoke any pre-existing rows
without a known duration instead of guessing their policy. Cleanup never
substitutes for checking expiration on access.

Deployment settings are separate from provider-resource YAML. They include
public origin, listener/TLS/proxy policy, session durations and login limits.
All are readable/editable through the future console and their documented file
representation. Listener/transport changes require restart; expose pending versus
effective values. Initial local setup supplies values needed to reach the console.
Exact configuration schema is an implementation prerequisite, not a hidden flag.

## Transaction and HTTP mapping

Add allowDelete to plan as well as apply. Expose token expiry from its issuer;
handlers do not decode opaque tokens. Return applied changes and appliedAt from
the transaction result, never by re-reading mutable state after commit.
Authentication runs before plan consumption. Resource versions still detect
concurrent writes from another authorized session.

Missing required If-Match returns 428; malformed values return 400; a stale value
returns 412. Referential conflicts return 409. ETags are strong quoted resource
versions; reject wildcard and lists on mutations. Creation cannot overwrite an
existing name. HTTP writes reuse graph validation and atomic persistence rather
than bypassing them through individual SQL writes.

Audit uses typed allowlisted actions for bootstrap, reset, session creation/logout,
and agent issue/revoke. Never include passwords, PHC strings, cookies, CSRF values,
bearer values, raw request bodies or arbitrary client text. Failed-login statistics
are aggregated into allowlisted minute buckets. The default 24-hour retention
bounds disk growth; an explicit unlimited operator setting trades that bound for
long-term history without increasing rows per minute.

Connection tests and strategy publication remain Milestone 4 endpoints. Until
implemented, their handlers must not claim success. Full backup remains Milestone 6.
