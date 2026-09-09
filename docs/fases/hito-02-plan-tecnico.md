# Milestone 2 — Technical delivery plan

[Español](hito-02-plan-tecnico.es.md)

- Status: implementation in progress
- Outcome: complete persisted configuration without routing execution or web UI,
  operable through CLI and tests
- Prerequisite: Milestone 1 accepted

## Progress

- Delivery item 1 — database lifecycle and migrations: accepted. Evidence is in
  [Milestone 2 database lifecycle evidence](../evidencia/hito-02-item-01-storage-lifecycle.md).
- Delivery item 2 — resource repositories and audit: accepted locally under the
  [resource repository contract](../contratos/storage/resource-repositories-v1.md),
  with [executable evidence](../evidencia/hito-02-item-02-resource-repositories.md).
- Delivery item 3 — master key and secret store: accepted. It includes encryption,
  a private keyring, startup verification, shared redaction, and recoverable
  rotation. See the [local, CI, and representative evidence](../evidencia/hito-02-rotation.md).
- Delivery item 4 — configuration parsing and validation: accepted. It includes
  limits, a typed model, graph validation, redacted export, QA, CI, and
  [representative measurement](../evidencia/hito-02-config-parser.md).
- Delivery item 5 — plan and atomic apply CLI: accepted. It includes transactional
  integration, the offline CLI, representative execution, CI, and grouped QA.

## Delivery order

Item 6 is the final pending part of Milestone 2. The item 5
[plan and authentication foundation](../evidencia/hito-02-plan-foundation.md) is accepted.

1. **Database lifecycle and migrations.** Replace the spike schema loader with
   embedded, monotonic migrations; verify checksums, WAL, integrity, exclusive
   ownership, interrupted startup, and clean-database bootstrap.
2. **Resource repositories and audit.** Implement transactional resource and typed
   records, optimistic versions, references, provider-affinity invariants, ordered
   deletion, and secret-free audit events.
3. **Master-key and secret store.** Create the private versioned keyring, encrypt
   and rotate secrets with XChaCha20-Poly1305 and bound associated data, expose
   metadata only, and fail closed on missing or invalid key material.
4. **Configuration codec and validation.** Parse bounded YAML, apply defaults,
   canonicalize documents, validate structure and cross-resource references, and
   produce deterministic redacted exports.
5. **Plan and atomic apply CLI.** Produce deterministic create/update/noop/delete
   plans, authenticated expiring plan files, interactive confirmation, conflict
   detection, all-or-nothing apply, and audit records under the installation lock.
6. **Integration and resource gate.** Exercise round trips, rollback, corruption,
   rotation interruption, concurrency conflicts, redaction, and the product-linked
   SQLite memory/disk cost on the representative VM.

Each item is independently reviewable, but it is accepted only after all earlier
items it depends on are complete. Backup container creation and restoration remain
Milestone 6 work; Milestone 2 must preserve the contracts they will consume.

## Milestone acceptance

- A clean installation migrates to the current schema and repeated startup is a
  no-op; changed migration checksums and incompatible schemas fail closed.
- Configuration survives export and re-apply without semantic drift or secret disclosure.
- Invalid references, provider mismatch, stale plans, and partial operations are
  rejected with the documented result and no committed subset.
- Secret values never appear in YAML, plan files, exports, errors, logs, audit
  details, or test snapshots.
- Killing rotation at each durable boundary leaves a state that restarts with the
  old or new key material and never loses decryptability.
- CLI stateful operations cannot race the running service for SQLite ownership.
- A successful mutation and success audit commit together; a rolled-back mutation
  records failure separately without claiming application.
- YAML/JSON byte, depth, document, alias, and schema limits are exercised at their
  boundaries. Canary secrets are absent from every output surface and typed audit
  details reject unapproved fields.
- Relevant tests, documentation validation, cross-builds, dependency review, and
  the representative resource measurement pass.
- Dependency review means clean `go mod tidy`, `govulncheck ./...` with no known
  reachable vulnerability, and a recorded license check for each direct module.
- Independent QA has no unresolved critical or high finding.

## Explicit deferrals

Administrative sessions and HTTP mutation endpoints are Milestone 3. Provider
calls, strategy publication behavior, and fallback are Milestone 4. The console is
Milestone 5, and full encrypted backup/restore is Milestone 6.
