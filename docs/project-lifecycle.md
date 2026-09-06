# Project Status and Lifecycle

This document records which stage ModelCairn is in, what evidence allowed that stage to be closed, and what conditions must be met before advancing. It does not replace the functional roadmap: it governs the process used to define, build, and publish the product.

## Advancement rule

A stage is closed when its deliverables are documented, its critical doubts are resolved, and an independent review has no remaining blocking findings. Any unresolved critical or high finding is blocking, as is a medium finding that invalidates evidence required for the exit criterion. Existing code does not automatically turn an idea into an approved requirement or a partial feature into a completed phase.

## Stage A - Context recovery (completed)

Objective: locate existing sources without changing the product. Execution happened before this documentation; its evidence was recorded during Stage C.

Evidence obtained on September 4, 2026:

- The available prototype was compared with its backup: the versioned content matches.
- Archive integrity was checked and an isolated copy was inspected.
- Provenance details and identifiers are kept only in local evidence excluded from Git.
- The backup contains unversioned material: recovered history from the previous conversation, a SQLite database, and interface previews.
- That material is untrusted and potentially private reference. It is not incorporated into the repository without a specific review.

## Stage B - Initial audit (completed)

Objective: determine whether the baseline should be kept before designing or programming.

Results:

- The prototype is kept as a separate local reference. The owner deleted and recreated the remote repository; the new clean clone contains only reviewed documentation and legal files.
- The 19 existing tests pass, but they do not yet certify real load, complete streaming, restarts, panel security, or multinode operation.
- The project declares no external npm dependencies, an initial advantage for the goal of a VM with 1 GB of RAM.
- Existing documentation contains contradictory states between architecture and roadmap; a single source of truth is needed.
- Authentication, authorization, migrations, replay protection, explainable routing, and quota estimation remain product work, even if partial implementations exist.
- At the time of the audit, a license had not yet been chosen; Apache-2.0 with NOTICE was later adopted and added.

The scope, evidence, findings, and independent review decision are kept in [Initial audit](auditorias/2026-09-04-auditoria-inicial.md).

## Stage C - Discovery and definition (completed for Phase 1)

Objective: turn the vision into a coherent product contract before modifying code.

First closed decision: [ADR-0001 adopts ModelCairn as the product name](decisiones/0001-nombre-del-producto.md). An independent review detected saturation of the root `Cairn` in software and AI. After comparing a second round of alternatives, that risk was accepted because there is no known exact collision, the metaphor represents the product, and the name works for its technical audience.

Second closed decision: [ADR-0002 chooses Apache-2.0](decisiones/0002-licencia-y-atribucion.md). `LICENSE` and `NOTICE` are ready for the first commit. Attribution will be part of the official interface; brand assets and sponsorship will be managed separately.

[ADR-0003](decisiones/0003-base-tecnica-y-despliegue.md) selects Go, modular monolith, systemd, and Linux AMD64/ARM64 artifacts. [ADR-0004](decisiones/0004-configuracion-secretos-y-recuperacion.md) defines SQLite as the source of truth, explicitly applied YAML, custody of secrets, and full encrypted backup.

The [ModelCairn Project Charter](project-charter.md) was approved as the basis for Phase 1. It separates confirmed vision, proposals, hypotheses, and future decisions.

On September 5, a first accelerated definition package was prepared:

- [Glossary and domain model](glossary-and-domain-model.md);
- [Initial target architecture](target-architecture.md);
- [Prioritized baseline requirements](baseline-requirements.md);
- [Phase 1 - Operable foundation](fases/fase-01-fundacion.md).

The four documents were reviewed, corrected, and approved as the basis for Phase 1.

The grouped decisions were approved on September 5. The [Phase 1 technical contracts](fases/fase-01-contratos-tecnicos.md) and its [implementation plan](fases/fase-01-plan-de-implementacion.md) complete the pre-development package. Milestone 0 produced its executable schemas, SQLite model, and technical ADRs; independent QA approved it with no critical or high findings.

[ADR-0005](decisiones/0005-criptografia-y-formato-de-backup.md) fixes Argon2id, XChaCha20-Poly1305, master key handling, and restore with re-encryption. [ADR-0006](decisiones/0006-stack-de-consola-web.md) selects TypeScript, React, and Vite with assets embedded in the Go executable.

Planned deliverables:

1. Vision, problem, users, and legitimate use cases.
2. Scope, exclusions, and responsible-use principles.
3. Glossary and domain model.
4. Prioritized and traceable functional requirements.
5. Measurable non-functional requirements: RAM, CPU, disk, latency, concurrency, availability, and recovery.
6. Threat model, trust boundaries, and handling of secrets.
7. Exact routing, errors, cooldown, fallback, and streaming contract.
8. Statistical model for limits and confidence level of its predictions.
9. Common configuration contract for file, API, and web panel.
10. Target architecture and architecture decision records (ADR).
11. Migration, compatibility, and rollback strategy.
12. Master test plan and acceptance criteria.
13. License decision and justification, governance, and open source contribution.
14. Matrix deciding which current modules are kept, corrected, or replaced.

For Phase 1, the points that affect its implementation were completed or sufficiently contracted. The detailed statistical model, relay protocol, final community governance, and contracts for later phases will be documented just before they are developed; they do not block Milestone 1.

Exit criterion: the documents do not contradict each other, each important requirement has acceptance criteria, and the architecture fits reasonably within the declared resource limits. It must then pass an independent review.

## Stage D - Phase 1 technical planning (completed)

Objective: transform the approved contracts into small milestones, dependencies, risks, migrations, and tests. This stage decides implementation order; it does not implement features.

Result: seven implementation milestones plus a documentation Milestone 0, RF/RNF traceability, risks, entry gates, and definition of done. Milestone 0 was approved by independent QA with no pending critical or high findings.

## Stage E - Incremental implementation (active)

Each increment must include code, tests, operational documentation, resource measurement, and independent review. Large batches of features without an intermediate quality gate are not accepted.

Milestone 1 began with the modular Go workspace, minimal CLI and HTTP lifecycle,
health/readiness probes, pinned CI actions, Linux cross-builds, and a dependency
inventory. Contract spikes now verify SQLite behavior and SSE commitment,
cancellation, and disconnect handling. Milestone 1 was completed on September 6
after CI passed and the reviewed 15-minute run on the representative 1 GB VM
reported 6,328 KiB peak RSS and zero process swap. Milestone 2 is next.

Milestone 2 was prepared on September 6 with an ordered technical plan and exact
contracts for process ownership, interrupted key rotation, plan tokens, bounded
configuration parsing, redaction, auditing, and routing-resource drafts.

## Stage F - System and security validation (pending)

Includes load on a VM equivalent to Oracle Free Tier, network failures, restarts, migrations, backup recovery, multinode tests, privacy, panel abuse, and review of the threat model.

## Stage G - Open source preparation and publication (pending)

Includes incorporating the license chosen in Stage C, completing the contribution guide and security policy, publishing versions and release notes, and verifying reproducible installation, upgrade, and rollback.

## Stage change log

| Date | Change | Evidence |
|---|---|---|
| 2026-09-04 | Stage A executed and completed | Repository and backup compared; technical evidence kept locally |
| 2026-09-04 | Stage B executed and completed | Local audit, 19 tests, and independent review |
| 2026-09-04 | Stage C started | Explicit implementation pause and deliverables list |
| 2026-09-05 | Stage C closed for Phase 1 | Charter, architecture, requirements, contracts, and ADRs approved |
| 2026-09-05 | Stage D completed for Phase 1 | Milestones, risks, traceability, and independent QA |
| 2026-09-05 | Stage E and Milestone 1 started | Go skeleton, tests, CI, cross-builds, and independent code review |
| 2026-09-06 | Milestone 1 completed | SQLite and streaming spikes, green CI, cross-builds, and reviewed representative resource benchmark |
| 2026-09-06 | Milestone 2 prepared | Persistence, configuration, secret-store contracts, delivery order, and independent QA |
