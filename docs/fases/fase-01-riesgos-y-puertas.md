# Phase 1 — Risks and quality gates

[Español](fase-01-riesgos-y-puertas.es.md)

- Baseline status: accepted as the gate set for Milestone 1. For current progress,
  see [Project Status and Lifecycle](../project-lifecycle.md).

## Active risks

| Risk | Signal | Mitigation | Decision if it occurs |
|---|---|---|---|
| Go/stack exceeds memory | steady-state RSS or peak outside budget | benchmark from Milestone 1, profiles, and limits | optimize or supersede ADR before continuing |
| SQLite accumulates contention | write queues/latency under events | short transactions, WAL, and benchmark | separate writes or revisit persistence |
| “OpenAI” compatibility loses fields | contract or real-client test fails | explicit matrix and capability rejection | expand adapter without silent passthrough |
| Fallback duplicates work | timeout after sending request | indeterminate result, no retry by default | enable only with idempotency/policy |
| API key leakage | secret appears in output or wrong endpoint | common redactor, affinity, and 6/6 tests | immediately block milestone |
| Web/YAML config diverges | resourceVersion conflict | single source, plan/apply, and ETag | reject and require a new plan |
| Backup does not recover | restore or route test fails | MCB1/age and atomic generations | do not close Milestone 6 |
| UI increases dependencies | bundle or audit exceeds budget | limit, deferred loading, and review | remove/replace dependency |

## Milestone entry gate

- Previous milestone accepted with no critical/high defects.
- Required contracts and ADRs available.
- Acceptance criteria and tests identified.
- No secrets, private data, or discarded history are incorporated.

## Definition of done

A milestone is complete only when:

1. included code fulfills its contract and has proportionate tests;
2. linters, build, and tests pass reproducibly;
3. documentation, migrations, and examples match behavior;
4. applicable budgets are measured, not assumed;
5. errors and secrets are reviewed in logs/exports;
6. an independent review has no unresolved critical or high findings;
7. project status links evidence and known limitations.

## Change policy

Implementation may reveal that a contract is not viable. In that case only the
affected work stops, evidence is recorded, the document or ADR is amended, and
tests are updated before continuing. Code is not used to silently change an
approved decision.
