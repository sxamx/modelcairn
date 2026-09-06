# Phase 1 — Technical contracts

[Español](fase-01-contratos-tecnicos.es.md)

- Status: verifiable technical draft
- Depends on: ADR-0003, ADR-0004, and Phase 1 — Operable foundation
- Rule: this document defines behavior; code cannot change it silently

## 1. Initial HTTP contract

### Surfaces

- Data: `POST /v1/chat/completions`.
- Administration: `/api/v1/admin/*`, authenticated by session.
- Operations: `/healthz` checks the process; `/readyz` checks persistence, secrets,
  and active configuration; `/metrics` is not public by default.
- The data API uses agent tokens, separate from the administrative session.

Compatibility is declared by a versioned matrix. Phase 1 accepts text messages,
common parameters, SSE streaming, and function tool calls when declared by the
destination. Unknown parameters are never silently discarded: they are sent only
when supported by the adapter, otherwise a compatibility error is returned before
the first attempt.

### Configurable initial limits

The schema includes maximum body and header sizes, total and per-attempt timeouts,
maximum attempts, and maximum connections. Final values are set by testing; safe
defaults always exist.

## 2. Errors, retries, and fallback

| Situation | Default action | Fallback |
|---|---|---|
| Missing/invalid/revoked agent token | return `401` | never |
| Agent lacks route permission | return `403` | never |
| Invalid JSON, parameters, or capabilities | return `400` | never |
| Missing route or alias | return `404` | never |
| Body too large | return `413` | never |
| Provider returns `401` | block credential until test or manual rotation | yes, to another authorized credential |
| Provider returns `403` for verified account/model permission | mark combination incompatible | only when adapter safely classifies cause |
| Provider returns `403` for safety/policy or unknown cause | return rejection without penalizing credential | no |
| Provider returns `429` | record known scope and apply conservative cooldown | yes |
| Provider returns `408`, `5xx`, or connection fails | classify as transient | yes, within budget |
| Permanent provider request error (`400/404/422`) | return compatible error and record | not by default |
| Timeout before connection/body send | cancel attempt | yes, before stream commitment |
| Timeout after body send without confirmed response | indeterminate result | not by default; only with idempotency or explicit policy |
| Total timeout or attempts exhausted | end request | no |
| Client cancellation | cancel upstream and record | no |

This table is default, not universal; an adapter may refine it when a provider
documents different semantics. Retries are never unbounded. Valid `Retry-After`
and official rate-limit metadata are honored; otherwise conservative cooldown with
jitter and a configurable maximum is used.

Each `429` observation has scope: provider, provider account, credential,
connection, model, or a specific combination. Official headers and adapter rules
take precedence. If scope is unknown, the credential is cooled across all
combinations and provider probing is limited; the same credential is not
immediately tried through another egress. Credentials may be grouped under a
logical account to share known limits without storing that account's login credentials.

An indeterminate result means the provider may have processed the request even
though ModelCairn did not receive a response. Tracing warns of duplication risk;
automatic fallback never equates “timeout” with “not processed.”

Every attempt has an internal identifier. The provider request ID is retained when
available, with redaction. Clients receive a ModelCairn request ID for diagnostics.

## 3. Streaming contract

- ModelCairn sends neither `200` nor opens the stream until a destination accepts
  the request and a valid upstream response exists.
- The **commitment point** is successful response headers or the first SSE
  byte/event sent to the client, whichever occurs first.
- Before commitment, fallback is allowed under strategy and budget.
- After commitment, models are not changed and responses are not mixed. Failure
  ends the stream and is recorded as partial.
- Ordering, indices, content deltas, tool-call deltas, `finish_reason`, and `[DONE]`
  are preserved when used by the dialect.
- Client disconnection promptly cancels the upstream context.
- Metrics distinguish provider TTFT, gateway overhead, total duration, and partial response.

This avoids hard-to-detect hybrid responses. Compatibility is checked against
current official Chat Completions documentation before implementing each field.

## 4. Declarative configuration

### Flow

`modelcairn config validate file.yaml` validates without changing state.
`modelcairn config plan file.yaml` shows redacted differences.
`modelcairn config apply file.yaml` applies one transaction and audits the change.
`modelcairn config export` produces YAML without secrets.

### Rules

- The document includes `apiVersion` and `kind`.
- Resources use stable IDs and mutable human labels.
- References must exist or be created in the same transaction.
- Secrets are referenced by ID; values are never exported.
- Invalid apply never partially changes configuration.
- The console uses the same schemas and validators as CLI/API.
- Publishing a strategy creates an immutable version; editing creates a new draft.

Phase 1 resources: `Provider`, `ProviderAccount`, `ProviderConnection`, `Credential`,
`Egress`, `Model`, `Destination`, `Strategy`, `Route`, and `AgentToken`.

## 5. Administrative API

- Versioned CRUD for configuration resources.
- Separate validate, test, publish, pause, and archive operations.
- Secrets are accepted on creation or rotation but never returned in full.
- Mutations use optimistic version control to prevent overwrites.
- Every mutation emits a secret-free audit event with actor, action, resource, date, and result.
- Private endpoints require an explicit flag and confirmed warning; redirects and every DNS resolution are revalidated.
- Connection tests have timeouts and limits and cannot turn the server into a network scanner.

## 6. Physical model and migrations

SQLite is exclusively owned by the main process and is not placed on a network
filesystem. Initial groups are: versioned configuration and published versions;
encrypted credentials and affinities; agent identities and token hashes; requests,
attempts, and decisions; observations and metric aggregates; administrative
sessions and auditing; schema version and maintenance jobs.

Service and offline CLI ownership, plus the interruption-safe keyring protocol,
are specified in [SQLite ownership and keyring durability](../contratos/storage/propiedad-y-llavero-v1.md).

Every migration has a monotonic ID, checksum, and transaction where SQLite allows.
A destructive migration requires a created and verified backup first. On failure,
the service is not ready and displays a recovery instruction; it does not continue
with a partially compatible schema. The initial strategy is roll-forward or backup
restore, not improvised down migrations.

## 7. Cryptography and backups

No custom algorithms are designed.

- Administrative passwords use Argon2id with ADR-0005 minimums and calibration.
- API keys use XChaCha20-Poly1305 with a unique nonce and context bound to secret ID and version.
- The master key comes from the system CSPRNG, lives outside SQLite, and has service-exclusive permissions.
- Agent tokens are random, shown once, and only SHA-256 of the 256-bit token is stored.
- Full backup uses streaming MCB1 over age v1 and restores atomically to a generation with a new master key.
- Passwords and keys are not passed as visible process arguments or written to unprotected temporaries.

Complete decisions are in ADR-0005 and the MCB1 specification. Benchmarking may
raise Argon2id parameters but never lower them below the documented minimum.

## 8. Resource benchmark

### Recorded environment

- Linux distribution/kernel, architecture, vCPU, and total memory;
- resident services and available memory before ModelCairn starts;
- exact executable version and configuration;
- configured swap, though success cannot depend on sustained swap.

### Scenarios

1. Idle for 15 minutes.
2. Console and metric queries.
3. 1, 2, 5, 10, and 20 concurrent streams against a simulated provider.
4. Fallback with timeout and `429`.
5. Sustained event writes and compaction.
6. Backup and restore.

Measurements include steady and peak RSS, CPU, goroutines, descriptors, SQLite
size/latency, swap activity, added latency, and loss. The initial design budget,
subject to benchmark correction, is at most 256 MiB steady RSS and 384 MiB peak
under reference load, reserving the remainder for the OS, cache, and other
services. Twenty streams are not promised if the VM cannot sustain them; actual
measured capacity is published.

## 9. Evidence to close Phase 1

- compatibility matrix and contract suite;
- unit, HTTP integration, and end-to-end tests;
- secret-leakage and SSRF tests;
- reproducible benchmark report;
- functional restoration from encrypted backup;
- clean-install and non-destructive uninstall guide;
- independent QA with no pending critical or high findings.
