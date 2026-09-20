# Milestone 7 — Validation and closeout technical plan

[Español](hito-07-plan-tecnico.es.md)

- Status: **completed and approved on September 19, 2026**
- Stage: F — system and security validation
- Objective: decide with reproducible evidence whether Phase 1 can be accepted;
  otherwise record concrete blockers without declaring partial closeout.

This milestone validates the product already built. It does not add the adaptive
estimator, egress relays, new compatible protocols, or visual editor: those are
later features and are not Phase 1 closeout criteria.

## Principles

1. The RF/RNF matrix will separate Phase 1 from deferred work and link every claim
   to a concrete test, contract, or evidence artifact.
2. Destructive tests will use a temporary installation and credentials.
3. Prompts and responses remain unpersisted by default. Review uses canary secrets,
   never the operator's real credentials.
4. Representative validation is grouped into one run on the 1 GB VM; a full
   benchmark is not repeated for every small change.
5. One grouped independent QA review runs after the blocks are complete.

## 1. Inventory and executable traceability

- Review RF-001–RF-013 and RNF-001–RNF-008 against actual state.
- Add direct links to tests, contracts, and representative evidence.
- Mark RF-101 onward and RF-201 onward as deferred, not missing.
- Record each deviation as fixed, explicitly accepted, or blocking.

Output: a complete matrix and CI-verifiable acceptance manifest.

## 2. System and induced failures

- Cover 429, 5xx, timeout, unreachable DNS/upstream, and client cancel.
- Validate streaming before/after committed headers, bounded fallback, and absence
  of unsafe retries.
- Test restart, concurrent configuration, version conflicts, interrupted
  publication/migration, insufficient capacity, and the last valid generation.
- Revalidate backup, verification, restore, rollback, and session revocation.

“Multinode” is limited here to architectural boundaries and ensuring the primary
node does not assume relays exist. Relay protocol and traffic (RF-101–RF-108) belong
to a later phase.

## 3. Load, resources, and retention

- Run at least 10 minutes of mixed load on the 1 GB VM: normal requests, SSE, and
  console traffic.
- Measure average/peak RSS, swap, CPU, binary, SQLite growth, and latency against
  previously defined RNF budgets.
- Seed synthetically aged login statistics to test their arbitrary retention—hours,
  months, years, and unlimited—without waiting in real time.
- Verify their batched pruning and characterize operational-history growth and
  queries. Configurable retention for requests, attempts, observations, and audit
  is RF-202 and is not implemented as part of this closeout.

Output: a reproducible bilingual report with environment, commands, and results.

## 4. Proportionate security and privacy

- Update the threat model and trust boundaries.
- Test authentication, logout/revocation, CSRF, authorization, and attempt limits.
- Search for canary secrets across logs, audit, plans, exports, errors, API, UI, and
  backups. A plaintext secret is blocking.
- Verify permissions, encryption at rest, transport by network mode, no external
  telemetry, and no prompt/response persistence by default.

The goal is practical security for a self-hosted service, not controls outside its
threat model.

## 5. Operations and contribution

- Consolidate the install, diagnosis, update, backup, restore, rollback, and
  uninstall runbook.
- Add a contribution guide and private vulnerability-reporting policy.
- Correct stale status and keep Spanish and English equivalent.
- Reserve the public release and release notes for Stage G.

## 6. Acceptance

1. Run CI and the complete representative validation.
2. Perform one grouped independent QA review using its own criteria.
3. Fix critical/high findings and medium findings that invalidate evidence.
4. Publish the final report and update traceability, lifecycle, and board.

Phase 1 is accepted only when every in-scope RF/RNF has valid evidence, CI is green,
the VM meets its budgets, and no security, privacy, recovery, or operational blocker
remains. Otherwise verifiable blockers are documented; it is never “almost done.”

## Result

All 20 in-scope requirements were accepted. CI, the representative 600-second
load, and [independent grouped QA](../evidencia/qa-agrupado-hito-07.md) passed after
all blockers were corrected. Phase 1 is accepted; this does not constitute a
public release.
