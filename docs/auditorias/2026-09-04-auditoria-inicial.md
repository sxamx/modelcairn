# Initial audit — September 4, 2026

[Español](2026-09-04-auditoria-inicial.es.md)

## Scope

Read-only review of the repository, its documentation, implementation, tests, and
history; comparison with the available backup; and independent evaluation before
continuing design or implementation.

## Evidence

- Versioned prototype content matches the backup; verification identifiers are retained locally.
- With Node.js `v24.18.0` on Windows, `npm test` completed 19 tests: 19 passed and 0 failed.
- `package.json` declares no external dependencies.

## Main findings

- The foundation is promising, but it is an accelerated prototype, not a production-validated release.
- Tests do not yet demonstrate load, RAM use, end-to-end streaming, restarts,
  panel security, or a real mesh between VMs.
- Architecture and roadmap contradict each other about completed phases.
- License, full authentication and roles, formal migrations, anti-replay protection,
  and measured non-functional requirements are missing.
- Provider preflight requires an SSRF-resistant design.
- The statistical quota model is an initial heuristic and needs an explicit
  evidence, uncertainty, and update contract.

## Decision

The initial decision was to retain the prototype and its history as reference.
After reviewing publication preferences, a new history with an initial
documentation commit was recommended. That migration was later completed through
a clean clone and allowlist. A discovery and definition stage was opened before
deciding, module by module, which implementation to retain, correct, or replace.

## Independent review

Two independent reviews were performed without editing files:

1. Repository and initial-conclusion review. It agreed on retaining the base,
   pausing implementation, and formalizing product, threats, architecture,
   resource limits, and acceptance criteria.
2. Review of the first stage-register version. It found a missing hash and
   reproducible method for the backup, mixed facts and hypotheses about GitHub,
   no persistent report, duplicated licensing, and spelling errors. These findings
   were corrected before delivery closed.

A final independent verification confirmed that those findings were resolved and
found no critical or high blockers. Minor editorial observations were incorporated
at closeout without opening a cycle of purely clerical reviews.

No known critical or high findings remained untreated in this audit's
documentation. This does not approve the product for production; it only permits
continuation to the next documentation stage.

## Documentation privacy

Post-audit update: the owner confirmed deletion and recreation of the remote
repository. The prototype remains in a separate local folder and the ModelCairn
repository was cloned empty; the first commit is prepared exclusively from the
documentation allowlist.

Public documentation retains methods, results, and decisions. Personal paths,
account identities, backup hashes, and historical references remain in a local
directory excluded from Git. Exclusion prevents ordinary staging; it does not
encrypt files or protect against forced staging. Publication review must include
the content prepared for commit.
