# Milestone 3 — Identity and administrative plane

[Español](hito-03-plan-tecnico.es.md)

Status: implementation started; draft pull request. Prerequisite: accepted Milestone 2.
The [runtime contract](../contratos/admin-runtime-v1.md) defines transport,
authentication and HTTP mapping. The [settings proposal](../contratos/admin-settings-v1.md)
defines deployment settings and login auditing. Proposed defaults need validation;
login-history retention still requires an operator decision.

## Intended outcome

Current handoff: representative measurement and integrated acceptance. Online CLI
parity now reuses the administrative API for sessions, configuration, secrets,
and AgentToken. The
[AgentToken lifecycle](../contratos/agent-token-lifecycle-v1.md) and
[transactional CRUD](../contratos/resource-mutations-v1.md) are implemented.

An installation can bootstrap its administrator locally, authenticate sessions,
manage configuration and secrets over HTTP, and issue/revoke agent tokens.
The console belongs to Milestone 5; provider requests belong to Milestone 4.

## Delivery order

1. **Local identity.** Argon2id hashing and verification under ADR-0005; exclusive
   bootstrap without replacing an existing administrator; local password reset,
   audit and atomic session invalidation. Validate UTF-8 and password bounds before
   hashing; bound PHC parameters before allocating. Measure calibration on the VM
   without reducing approved minimums.
2. **Sessions and HTTP boundary.** Login, logout, current session, token hashes,
   absolute/idle expiry, auth_version, cookies, CSRF and Origin. Define trusted proxy
   behavior first. Bound memory, concurrency and request frequency before Argon2id.
3. **Resource administration.** Pagination, ETags, preconditions, validate/plan/apply,
   export, secret metadata and secret writes/deletes. Reuse Milestone 2 transactions.
   Invalid references, versions or audit writes must not leave partial updates.
   Online CLI uses the API when the service is active under ADR-0004; offline mode
   retains exclusive ownership.
4. **Agent tokens.** Random 32-byte issuance, one-time delivery, SHA-256 verifier,
   route permissions, expiry and irreversible revocation per identity. A lost
   response does not permit recovering the value; document revocation and creation
   of a new identity. Test complete separation from administrative sessions.
5. **Integration and acceptance.** Group HTTP identity, resource, secret, error,
   concurrency and restart tests. Measure authentication latency and RAM on the VM.
   Perform independent grouped review and resolve findings before milestone closure.

## Contract alignment

| Topic | Existing gap | Required resolution |
|---|---|---|
| Proxy HTTPS | Secure cookie without trust policy | Explicit public origin and trusted proxies; loopback-only plain HTTP |
| CSRF recovery | General CSRF requirement conflicts with /session/me | Authenticated recovery exception and same-origin checks; no caching |
| GET rotation | GET mutation ban conflicts with token rotation | Ban business mutations; explicitly permit defined security rotation |
| Login pressure | No numerical or storage bounds | One password derivation, bounded admission and counters, measured defaults |
| Client identity | Web Origin is not client identity | Separate web origin from trusted client IP; reject forged forwarding |
| HTTP plan | Internal token/changes differ from DTO | Issuer exposes expiry; map fields without parsing opaque tokens in handlers |
| HTTP apply | Manager currently returns only an error | Obtain changes/time from committed transaction result |
| Planned deletion | Plan lacked allowDelete | Same bound option for plan and apply |
| Export/metadata | Missing API endpoints | Add HTTP parity without exposing secret values |
| Identity audit | Resource audit is typed, identity actions missing | Typed minimal events, bounded unauthenticated statistics |

## Proportional verification

Demonstrate a single concurrent bootstrap winner, reset with atomic revocation,
uniform unknown-user/incorrect-password responses, PHC bounds, expiry after restart,
tampered cookie/CSRF rejection, multi-tab CSRF recovery and forged proxy headers.
Verify that increasing idleSeconds and restarting cannot revive an expired session,
and that a settings plan for document A cannot apply document B or be reused.
Login admission must stay memory-bounded with concurrent and cancelled requests.

For resources, demonstrate stale ETags, invalid references, expired/reused plans,
HTTP/online-CLI conflicts and transaction-faithful responses. For tokens, verify
single concurrent issuance, per-route authorization and revocation for new requests.
Capture outputs, errors and audit with canaries, permitting intentional initial
token delivery only to the authorized client.

Each delivery includes targeted tests and documentation. Group independent QA by
substantial delivery; editorial edits do not trigger another review.

## Before authentication implementation

Finalize the bilingual settings contract and retention decision, align OpenAPI,
and define any new migrations without modifying published ones. Settings exposed
in files must also be represented in the future console. Clearly distinguish
initial settings needed to reach that console from effective runtime settings.

## Grouped design review — 2026-09-09

Independent review identified two concrete gaps: idle-policy changes could revive
expired sessions, and settings plans did not explicitly bind the desired document.
The runtime/settings contracts now capture session durations at issuance and bind
plans to the normalized desired-document digest with atomic single-use semantics.
These are documentation corrections, not implemented or tested runtime behavior.
The settings structural JSON Schema and GET/plan/apply OpenAPI definitions are now
drafted. Local validation passed 95 positive/negative schema cases, resolved 111
OpenAPI references and checked agreement between settings field lists. These checks
do not prove semantic validation, HTTP behavior or database behavior. The regular
documentation checker now checks settings field completeness, without adding a
runtime dependency or claiming full JSON Schema validation.
The [identity storage contract](../contratos/storage/admin-identity-v1.md) defines the
settings/session migration without modifying older migrations. Its executable
SQL contract passed 19 checks covering upgrade, bounds and transactional rollback.
Production migration 0003 is installed by the existing runner; Go lifecycle and
contract tests cover sequential upgrade, schema compatibility and rollback. HTTP
tests remain delivery work.
Remaining prerequisites: final contract integration and the operator's login-history
retention decision.

## Implementation handoff — 2026-09-10

The first bounded implementation delivery is now present in
`internal/adminsettings`: the shared administrative settings decoder, omission-aware
resolver, canonical origin normalization and pure semantic validator use the
settings/runtime contracts and JSON Schema above. It is independent of login-history
retention: it does not expose login or silently choose
that policy. This is dependency ordering, not removal of a Milestone 3 requirement.

Acceptance for this delivery: initial defaults versus preserved update omissions;
strict JSON/YAML decoding and 64 KiB bounds; all numeric and transport combinations;
canonical origin/listener/CIDR validation; deterministic resolved representation;
field-name-only errors without rejected values; and table-driven positive/negative
tests. The implementation reuses existing dependencies and keeps file accessibility
outside pure validation, so tests need neither a VM nor real credentials. Local
verification on 2026-09-10 passed package tests, `go vet ./...`, all Go tests except
the process-death lock test intentionally omitted in this Windows environment, the
19-check migration SQL contract and
the documentation checker. This is implementation evidence, not VM acceptance.
A single independent review should be grouped with the completed block rather than
repeated for each edit.

The grouped implementation review found two representation defects: an inherited
nil proxy list could serialize as JSON null, and normalizing publicOrigin before
checking its raw bound could accept oversized input. Resolution now always owns a
non-nil proxy slice and validates patch string bounds before normalization. Focused
regressions and the independent recheck passed; no finding remained in this block.

Settings persistence now reads only the exact canonical resolved representation,
validates it again and fails closed on missing or corrupt state. Transactional
revision-one insertion is available for composition with the future administrator
bootstrap and audit; it is not a standalone bootstrap. Tests cover absence,
validation, rollback, duplicate creation, canonical round trips and corruption.
Versioned settings updates now have cryptographic purpose isolation from provider
plans and an internal plan/apply service. Settings, nonce consumption and audit
commit together. Integration tests cover altered and stale plans, replay, no-op,
startup snapshots and reverting to effective values. The complete local Go suite
and `go vet ./...` passed for this delivery, including the process-death lock test.
HTTP/CLI integration is not yet implemented; these tests do not prove VM acceptance.

Independent review of the plan/apply block found that replay after a changed apply
returned version conflict instead of plan_already_used. The stale-revision path now
authenticates the token before looking up its consumed nonce. Regression tests
assert the exact replay error and reject invalid tokens. Independent recheck found
no further defect; the complete Go suite and static analysis pass.

The next delivery adds canonical, pre-bounded Argon2id PHC handling plus local
bootstrap and password reset. Bootstrap commits identity, settings and audit in one
transaction and admits one concurrent winner. Reset atomically increments
auth_version, revokes sessions and audits. Commands accept passwords only through a
confirmed TTY or stdin and validate settings before creating installation state.
The HTTP login/session boundary remains unimplemented.

Local identity review corrected rejected-argument disclosure in CLI errors and
moved username validation before installation creation. Password reader errors
use a fixed code. Regressions ensure a password canary in rejected arguments never
appears in stdout/stderr and an invalid username creates no installation state.
This does not replace VM measurement of Argon2id.

Next implementable block: session storage before exposing HTTP login. Generate
32 random bytes for session and CSRF values, persist only hashes and recheck
auth_version in the transaction after password verification. Capture idleSeconds
and absolute expiry at issuance; reject revocation, expiry, invalid auth_version
or invalid CSRF before touching last_seen. CSRF rotation retains one previous hash
for 60 seconds. Test concurrent reset, audit rollback and non-revival after an idle
policy increase. Login admission must cover real and dummy derivation with one
non-queuing permit plus the specified global/per-client limits. Keep HTTP login
unexposed until failed-login retention is decided.

The persistent session and internal login layers now implement that handoff. Tests
cover non-persistence of bearer values, use/logout, the CSRF window, expiry without
activity advancement, backwards clocks, reset races, audit rollback, uniform
failure, backoff, token buckets, bounded clients and one concurrent derivation.
A grouped security review remains before building the HTTP boundary.

Subsequent review corrected login admission: the exclusive permit is acquired
before querying SQLite and retained until the attempt finishes. This prevents
attempts from queuing for the connection before the concurrency limit. A regression
holds SQLite and verifies that a busy login is rejected before waiting for it.
Session tokens are limited to 43 characters before decoding.
HTTP integration and its review remain pending.

The internal HTTP boundary now checks transport, Host, origin, trusted single-hop
proxy and unique session cookie; handler wiring remains outstanding. Malformed
headers remain visible for rejection rather than being treated as absent.
HTTP settings mutations must use `AdminSettingsService.ApplySession`: it checks
the live session, auth_version and CSRF in the same transaction as activity, plan
consumption, settings and audit. The audit identity comes from that session.
`Apply` is reserved for the local CLI holding the installation lock. Earlier
middleware validation can reject early but cannot authorize a later write. Time
is read after acquiring locks. If reset commits first, the mutation is rejected;
if mutation commits first, reset subsequently invalidates its session. Failures
also roll back activity and nonce consumption. Regression coverage forces reset
while apply waits, and checks audit-failure rollback and subsequent plan reuse.
Plan/State still require caller authentication; a plan never replaces apply-time
authorization. Future protected mutations must use the same transaction pattern,
without calling DB or opening nested transactions from callbacks.

The server now exposes the initial administrative flow: login, session/CSRF
recovery, logout, and settings read/plan/apply. Login accepts only JSON up to 16
KiB with unique known fields; settings accept JSON/YAML up to 64 KiB. Cookies use
HttpOnly, SameSite Strict, root Path and transport-dependent Secure. Responses are
no-store and logs record route templates rather than bodies or credentials. An
installation awaiting bootstrap exposes health and readiness without registering
administrative routes. An integration test covers login, CSRF rotation, read,
plan, apply, transactional audit, logout and subsequent rejection. Grouped review,
the login aggregate decision/implementation and the remaining milestone API are
still outstanding.

Integration correction: startup now uses persisted `listen` and rejects a
different `--listen` after bootstrap. Before bootstrap the flag still controls
health-only serving. direct-tls loads certificate/key before binding and wraps
the listener with TLS (minimum 1.2); failure stops startup with a fixed error code.
A real HTTPS test explicitly trusts the local certificate. Session cookies omit
Expires/Max-Age: SQLite enforces absolute and sliding expiry on each request,
avoiding premature browser logout at the initial idle deadline. Invalid CSRF
returns 403, invalid sessions 401, internal session failures 503. Logout is no-store.

### Technical handoff for resources and secrets

- Implemented `storage.AuthorizeAdminMutationTx` derives Actor from the current
  session and updates activity within the caller transaction. Never reuse a
  middleware Actor or accept third-party callbacks. Roll back everything on any
  error. Lock order: SecretStore lock, SQLite transaction, authorization,
  preconditions/validation, write/audit, commit, unlock.
- Configuration: add `Manager.ApplySession` using this helper at the start of the
  ExecutePlan snapshot callback. Retain prepareTx and ApplyConfigTx for full graph
  validation. Return changes/appliedAt captured in that operation only after
  commit; obtain expiresAt from the issuer without decoding opaque tokens in HTTP.
- Resource CRUD: build operations through the existing configuration validator,
  checking name/kind and ETag in the same transaction. Do not wire Repository
  Put/Delete directly to HTTP: they do not enforce the entire graph contract.
  Creation requires absence; update/delete require the observed version.
- Secrets: add authenticated entry points retaining the existing SecretStore lock
  across authorization, write, audit, commit and redaction registration. Reuse
  putTx and extract deleteTx; never call Put/Delete inside transaction callbacks.
  If post-commit registration fails, make the store unavailable and report an
  uncertain result rather than claiming a rollback that did not happen.
- Paginated reads: SQL filtered by kind, `id > cursor`, id ordering, and a limit
  of 1..200 plus one row for nextCursor. Bound cursors and bind them to the queried
  kind. Do not load List then paginate in memory; no cross-page snapshot guarantee.
- ETags: one quoted positive version; missing 428, malformed 400, stale 412.
  Plans retain 409 conflicts. Agent tokens require their dedicated service;
  generic CRUD must not issue or revive verifiers.

This is an implementation handoff, not delivered functionality. Group acceptance
around graph rules, preconditions, revocation, audit rollback and secret-free
responses/logs. Login retention remains undecided; this milestone is not accepted
for final deployment.

The first handoff item is now implemented: Configuration validate, plan, apply
and export are connected to the administrative API. Plan returns issuer expiry
and redacted changes; apply returns changes/appliedAt captured by the operation.
`Manager.ApplySession` authorizes inside ExecutePlan before snapshot preparation,
committing session, nonce consumption, complete graph and audit in one transaction.
A revoked session does not consume the plan; regression coverage reuses that same
plan with a fresh session and confirms success. The integrated HTTP flow validates,
plans, applies and exports a resource without reflecting its token. Individual CRUD
secrets, agent tokens, and online CLI are connected.

Resource and secret-metadata reads now expose individual GET and paginated lists.
Queries filter by kind and `id > cursor`, request limit plus one and never load the
whole catalog; opaque cursors are bounded and scope-bound. Pages do not promise a
cross-request snapshot. Individual GETs return strong resourceVersion ETags. The
secret representation exposes only name, non-reversible fingerprint, version and
update time; it omits value, internal id, key version and creation time. Mutations
are now connected.

Secret writes now implement PUT and DELETE with Origin, session/CSRF and strong
preconditions. Omitting If-Match creates; an existing name then requires 428.
Replacement/deletion require one quoted version; malformed input returns 400 and
a stale version 412. `SecretStore.PutSession/DeleteSession` hold the store lock
through authorization, encryption/deletion, audit and commit. The actor is derived
from the session, and a revoked session writes nothing. JSON rejects duplicate or
unknown fields and clears the value buffer after use. The API returns metadata and
ETag only. An exceptional redaction-registration failure after commit closes the
SecretStore so operation cannot continue with incomplete output protection.

The automated integration gate now covers the complete administrative plane and is
included in CI. The milestone still requires VM measurement and final independent
review. Identity, sessions, HTTP administration, access tokens, and CLI parity are
implemented. Login aggregate retention remains an
explicit operator decision. The branch is published as draft PR #20, not a merged
release. The design work now supports this concrete implementation
handoff; it does not justify claiming the whole security design is accepted.
