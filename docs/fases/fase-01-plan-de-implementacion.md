# Phase 1 — Implementation plan

[Español](fase-01-plan-de-implementacion.es.md)

- Status: accepted; Milestones 0 and 1 complete, Milestone 2 next
- Delivery rule: every milestone includes code, tests, and documentation; a visual
  demonstration alone does not make it complete

## Milestone 0 — Executable contracts, no product

Status: **completed and approved by independent QA on September 5, 2026**.

- JSON Schema for YAML with fields, types, references, absences, and validations.
- Administrative OpenAPI with errors, concurrency, and each resource lifecycle.
- Initial SQLite model with keys, constraints, indexes, and transactions.
- Exact Phase 1 Chat Completions matrix.
- Session, cookie, CSRF, revocation, and reset contract.
- Cryptographic ADR and backup/restore format and atomicity.
- Console stack selection and artifact/resource budget.

Output: reviewed contracts that can generate fixtures and tests. Spikes are
discarded; no persistent product code is written yet.

## Milestone 1 — Skeleton and verified decisions

Status: **completed on September 6, 2026**. The representative 1 GB VM run
passed its 15-minute resource gate; see the
[reviewed benchmark evidence](../evidencia/benchmark-hito-1-2026-09-06.md).

- Modular Go structure, CLI, and minimal HTTP server.
- Reproducible Linux AMD64/ARM64 build.
- Empty benchmark and automated budgets.
- SQLite and streaming integration spikes against approved contracts.
- Dependency registry and verification.

Output: technical decisions proven on the VM or corrected before code accumulates.

Prerequisite: Milestone 0 accepted. Every later milestone requires the previous
one to meet its output and retain no critical or high blockers.

Common rules are defined in [Risks and quality gates](fase-01-riesgos-y-puertas.md).

## Milestone 2 — Persistence, configuration, and secrets

- SQLite migrations and repositories.
- YAML schema, validate/plan/apply/export.
- Master key, encrypted credentials, and common redaction.
- Auditing and round-trip/safe-failure tests.

Output: complete configuration without router or interface, operable by CLI and tests.

## Milestone 3 — Identity and administrative plane

- Bootstrap, administrative password, and local reset.
- Sessions, CSRF where applicable, expiration, and login rate limit.
- Revocable agent tokens and permission separation.
- Versioned administrative API.

Output: secure control plane covered by HTTP integration.

## Milestone 4 — Vertical router

- OpenAI-compatible adapter and connection validation.
- Model, destination, alias, route, and sequential strategy.
- Normal Chat Completions, tool calls, and streaming.
- Error classification, cooldown, budgets, and fallback.
- Operational events without content.

Output: first end-to-end call and deterministic contract suite.

## Milestone 5 — Console and PWA

- Onboarding from provider to published route.
- Resource and secret management without revealing them.
- Health, request, and attempt diagnostics.
- Responsive design, manifest, PWA installation, and basic accessibility.

Output: primary case completed from the web without editing code.

## Milestone 6 — Installation and recovery

- systemd installer with automatic-start choice.
- Localhost and private-network modes, and Tailscale Serve guide.
- Export, encrypted backup, and functional restore.
- Health/readiness, logs, and diagnostics.

Output: empty installation and recovery verified on a clean VM.

## Milestone 7 — Validation and closeout

- Complete RF/RNF/test acceptance matrix.
- Load and retention benchmark.
- Induced failures, interrupted migrations, and secrets review.
- Operator and contributor documentation.
- Independent QA and finding remediation.

Output: Phase 1 accepted or an explicit blocker list; never partial closeout.

## Commit policy

Commits are made when a unit is coherent and verifiable, not to increase the
count. Every commit has a single purpose, associated tests, and no secrets.
Milestones may contain several small commits; milestone closeout is recorded in
status documentation.
