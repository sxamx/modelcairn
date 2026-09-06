# Phase 1 — Operable foundation

[Español](fase-01-fundacion.es.md)

- Baseline status: scope and planning approved for Milestone 1. For the current
  project status, see [Project Status and Lifecycle](../project-lifecycle.md).
- Objective: prove a complete, secure, installable, and measurable path before adding adaptive learning, relays, or the visual editor

## Phase outcome

A person installs ModelCairn on a Linux VM, completes bootstrap, opens the console,
registers an OpenAI-compatible provider, credential, and model, publishes a logical
route, and uses it through streaming `POST /v1/chat/completions`. They can diagnose
it, restart the service, and restore a backup without losing configuration.

## Included

- Initial packaging for one reference Linux platform.
- Administrative account and access-mode bootstrap.
- Bootstrap choice for automatic service startup; recommended but optional.
- Administrative login and session.
- Responsive console and installable PWA structure from the first phase.
- Embedded persistence and versioned migrations.
- Central encrypted API-key store and output redaction.
- Web and administrative CRUD for an OpenAI-compatible provider, its connection,
  credentials, models, destinations, alias, and a basic sequential strategy.
- Revocable token for an agent or integration.
- Non-streaming and streaming Chat Completions.
- Versioned compatibility matrix, including tools when the destination declares and proves that capability.
- Sequential fallback before delivering content to the client.
- Minimal error classification, timeouts, and attempt budget.
- Local events and metrics without prompts or responses.
- Health, readiness, structured logs, and resource diagnostics.
- Configuration export without secrets.
- Documented and tested local backup and restore.
- Provider endpoint validation and explicit authorization for private networks.
- Reproducible memory measurement on a target VM.

## Excluded and deferred

- Egress relays and multinode topology.
- Complete adaptive estimator; only events needed later are captured now.
- Visual graph editor; a simple sequential strategy will be used.
- Anthropic compatibility, Responses API, and complex translations.
- Multiple administrators, RBAC, and 2FA.
- Prompt or response storage.
- Automatic updates from the console.
- High availability for the main node.

## Minimum routing contract

- The strategy contains an ordered list of compatible destinations.
- Every request has total and per-attempt timeouts and a maximum attempt count.
- Client authentication errors and invalid requests never cause fallback.
- Provider errors use explicit classification rules; exact codes close before router implementation.
- A `429` may set cooldown and permit the next destination while budget remains.
- A timeout permits fallback only before content is delivered.
- After part of a stream is sent, ModelCairn never mixes continuation from another model.
- Streaming specifies a commitment point covering headers, SSE opening, and first event; after it, failure explicitly ends the stream without invisible fallback.
- The outcome records attempts and the reason for every transition.

## Acceptance criteria

1. A clean installation can be completed using only documentation.
2. The operator creates and tests a route from the web without editing code.
3. The route responds normally and by streaming in contract tests; the matrix declares and tests tools and other included capabilities.
4. An invalid credential, `429`, and timeout yield traceable outcomes and honor configured policy.
5. No log, admin API, export, or error test finds a complete API key.
6. Restart preserves published configuration and does not corrupt events.
7. Backup and restore are tested on an empty installation.
8. Reference load stays within the memory budget agreed before phase closeout.
9. Migrations are tested from every schema version included in the phase.
10. Independent review leaves no critical or high defects open.
11. Revocation blocks new agent calls; agent tokens cannot access administration and admin sessions cannot substitute for them.
12. Cookies, session expiration, CSRF where applicable, login rate limiting, and secure persistence across restart pass integration tests.
13. Unauthorized local endpoints, cloud metadata, schemes, ports, redirects, and resolution changes are blocked; private endpoints require an explicit, auditable exception.
14. Interrupted migration fails safely. The initial strategy is roll-forward with mandatory prior backup and tested restore, not improvised down migrations.

## Test strategy

- Unit tests for validation, redaction, error classification, and budgets.
- Contract tests against a deterministic simulated provider.
- Real HTTP integration for administration, login, API, and streaming.
- End-to-end from empty installation to a functional call.
- Induced `401`, `429`, `500`, timeout, cut stream, constrained disk, and restart failures.
- Focused security: output secrets, session, file permissions, body limits, malformed input, SSRF, redirects, private endpoints, and resistant admin-password hash verification.
- Performance: idle, gradual concurrency, and event retention.
- Recovery: backup, restore, and migration.

## Gate to begin implementation

Closed decisions: Go modular monolith; Linux AMD64/ARM64 with the real reference VM
first; systemd with installer startup choice; SQLite as source of truth and YAML by
explicit apply/export; local master key, secret-free exports, and full encrypted
backup; local CLI recovery; private endpoints blocked by default with explicit exceptions.

Technical deliverables developed and QA-approved: initial error table and detailed
streaming contract; benchmark scenario, load, and threshold; configuration and
administrative API contracts; initial physical model and migration plan; verifiable
encryption design and backup format.

Milestone 0 also produced the JSON Schema, OpenAPI, SQLite schema, session contract,
compatibility matrix, and frontend ADR required to implement without inventing
their shape in code. See [Phase 1 technical contracts](fase-01-contratos-tecnicos.md)
and [Phase 1 implementation plan](fase-01-plan-de-implementacion.md). Milestone 0
was approved with no pending critical or high findings. Milestone 1 may begin after
preparing the clean repository.

The benchmark must state hardware and architecture, memory available after OS and
base services, concurrency, duration, load pattern, peak RSS, swap activity, and
reserved margin. The threshold is approved from those data, not one idle reading.

## Initial traceability matrix

| Requirement | Primary evidence |
|---|---|
| RF-001, RF-008 | Normal, streaming, cutoff, and tools contract tests |
| RF-002 | Token integration, revocation, and permission separation |
| RF-003, RF-010 | Console end-to-end flow and declarative round-trip |
| RF-004, RF-005 | Store, redaction, and persistent-affinity tests |
| RF-006, RF-007 | Error, fallback, and budget-exhaustion cases |
| RF-009 | Automated inspection of content-free events |
| RF-011 | Health/readiness tests with degraded dependencies |
| RF-012 | Migration, interruption, backup, and restore |
| RF-013 | SSRF suite and auditable private-endpoint exception |

`health` means the process is alive. `readiness` requires usable persistence,
secret storage, and active configuration; one provider's outage affects its
destinations but does not make the whole instance unready while an operable
administrative route exists.
