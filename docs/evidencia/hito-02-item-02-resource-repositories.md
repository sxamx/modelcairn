# Milestone 2 evidence — resource repositories and audit

[Español](hito-02-item-02-resource-repositories.es.md)

- Scope: delivery item 2 and issue #9
- State: accepted; implementation, independent QA, and CI complete

## Implemented behavior

- One repository owns configuration resource envelopes, typed projections, and
  mutation audit records.
- Creates assign immutable UUIDs and version 1; updates require the exact current
  version and increment it once; reads and lists are deterministic.
- Envelope, typed projection, and success audit commit together. Typed/reference,
  affinity, uniqueness, or audit failure rolls the whole mutation back.
- A known rolled-back mutation records a typed failure audit in a second
  transaction. Commit errors are treated as indeterminate and never mislabeled by
  a later failure audit.
- Physical and JSON-backed references enforce safe deletion order. Credential
  deletion leaves its Secret untouched.
- AgentToken revocation is irreversible for an identity and preserves the original
  revocation time plus issued verifier metadata.

## Executable evidence

Automated tests round-trip all ten v1alpha1 resource kinds and inspect their typed
references. They exercise create/update rollback, stale and concurrent versions,
missing references, cross-provider destinations, physical and logical deletion
dependencies, explicit delete authorization, success-audit rollback, separate
failure audit, audit-output allowlists, and an unavailable audit database.

The transaction-sensitive subset passes ten repeated executions. AgentToken
revocation and timestamp preservation pass twenty repeated executions with the
test clock advanced between writes. Complete Go tests, `go vet`, documentation
validation, and diff hygiene pass locally.

Independent QA found two blocking consistency errors and one weak regression test:
an out-of-contract audit action, contradictory token reactivation, and a fixed
test clock that could hide timestamp replacement. All were corrected. The final
review reported no remaining functional blocker.

## Publication gate

Pull-request CI passed the race-enabled complete suite, documentation and schema
contracts, module-file check, resource smoke test, and Linux AMD64/ARM64 builds.
