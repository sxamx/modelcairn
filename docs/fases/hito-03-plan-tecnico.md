# Milestone 3 — Identity and administrative plane

[Español](hito-03-plan-tecnico.es.md)

Status: technical draft; implementation pending. Prerequisite: accepted Milestone 2.
The [runtime contract](../contratos/admin-runtime-v1.md) defines transport,
authentication and HTTP mapping. The [settings proposal](../contratos/admin-settings-v1.md)
defines deployment settings and login auditing. Proposed defaults need validation;
login-history retention still requires an operator decision.

## Intended outcome

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
drafted. Local validation passed 95 positive/negative schema cases, resolved 103
OpenAPI references and checked agreement between settings field lists. These checks
do not prove semantic validation, HTTP behavior or database behavior. The regular
documentation checker now checks settings field completeness, without adding a
runtime dependency or claiming full JSON Schema validation.
The [identity storage draft](../contratos/storage/admin-identity-v1.md) defines the
settings/session migration without modifying published migrations. Its executable
SQL contract passed 19 checks covering upgrade, bounds and transactional rollback.
It is not installed by the runtime; Go runner and HTTP tests remain delivery work.
Remaining prerequisites: final contract integration and the operator's login-history
retention decision.

## Implementation handoff — 2026-09-10

The first bounded implementation delivery is now present in
`internal/adminsettings`: the shared administrative settings decoder, omission-aware
resolver, canonical origin normalization and pure semantic validator use the
settings/runtime contracts and JSON Schema above. It is independent of login-history
retention: it does not expose login, activate the draft migration or silently choose
that policy. This is dependency ordering, not removal of a Milestone 3 requirement.

Acceptance for this delivery: initial defaults versus preserved update omissions;
strict JSON/YAML decoding and 64 KiB bounds; all numeric and transport combinations;
canonical origin/listener/CIDR validation; deterministic resolved representation;
field-name-only errors without rejected values; and table-driven positive/negative
tests. The implementation reuses existing dependencies and keeps file accessibility
outside pure validation, so tests need neither a VM nor real credentials. Local
verification on 2026-09-10 passed package tests, `go vet ./...`, all Go tests except
the process-death lock test intentionally omitted in this Windows environment, the
19-check draft SQL contract and
the documentation checker. This is implementation evidence, not VM acceptance.
A single independent review should be grouped with the completed block rather than
repeated for each edit.

The grouped implementation review found two representation defects: an inherited
nil proxy list could serialize as JSON null, and normalizing publicOrigin before
checking its raw bound could accept oversized input. Resolution now always owns a
non-nil proxy slice and validates patch string bounds before normalization. Focused
regressions and the independent recheck passed; no finding remained in this block.

The complete milestone remains unfinished. Outstanding implementation includes
identity/session services, production migrations, HTTP handlers, CLI parity, tokens,
VM measurements and integration acceptance. Login aggregate retention remains an
explicit operator decision. The current worktree is a local draft, not a published
or merged release. The design work now supports this concrete implementation
handoff; it does not justify claiming the whole security design is accepted.
