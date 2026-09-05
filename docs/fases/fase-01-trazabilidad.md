# Phase 1 — Requirements traceability

[Español](fase-01-trazabilidad.es.md)

| Requirement | Contract | Milestone | Completion evidence |
|---|---|---:|---|
| RF-001, RF-008 | compatibility and streaming | 4 | normal HTTP/SSE, tools, cutoff, and cancellation suite |
| RF-002 | sessions and AgentToken | 3 | login, separation, revocation, and expiration |
| RF-003 | JSON Schema, apply, and OpenAPI | 2–5 | end-to-end CRUD/CLI/web |
| RF-004 | ADR-0005 and SQLite schema | 2 | encryption, redaction, and key failure |
| RF-005 | Credential.egressRef | 2, 4 | persistence and no automatic reassignment |
| RF-006, RF-007 | error table and strategy | 4 | failure matrix and budget exhaustion |
| RF-009 | requests/attempts/observations | 4 | inspection proving absence of content |
| RF-010 | apply/OpenAPI/ADR-0006 | 2, 5 | CLI and web functional equivalence |
| RF-011 | health/readiness | 3, 6 | healthy and degraded dependencies |
| RF-012 | MCB1 and migrations | 2, 6 | interrupted failure and functional restore |
| RF-013 | ProviderConnection | 4 | SSRF, DNS/redirect, and private-exception suite |
| RNF-001–003 | benchmark/router contract | 1, 4, 7 | reproducible report, limits, and absence of loops |
| RNF-004 | audit_events | 2–7 | coverage of critical mutations |
| RNF-005 | installation/backup | 6 | clean VM, manual update, and restore |
| RNF-006 | compatibility matrix | 4 | declared/rejected capabilities |
| RNF-007 | ADR-0006 and onboarding | 5 | usability and basic accessibility tests |
| RNF-008 | architecture and network | 1–7 | test with no external telemetry destinations |

Each milestone updates this table with links to real tests. A row without evidence
prevents Phase 1 from closing even if the interface appears complete.
