# Milestone 2 evidence — database lifecycle

[Español](hito-02-item-01-storage-lifecycle.es.md)

- Scope: delivery item 1 and issue #8
- State: accepted; implementation, independent QA, CI, and representative Linux
  execution complete

## Implemented behavior

- The service acquires the installation's non-blocking operating-system lock
  before opening SQLite and holds it until SQLite closes.
- The lock file is opened without following symbolic links on Unix and reparse
  points on Windows; the private data directory and lock permissions are enforced.
- Embedded monotonic migrations use an immutable SHA-256 ledger and one
  transaction per migration.
- Startup rejects unknown versions, checksum changes, non-contiguous history,
  physical corruption, and logical schema drift.
- Logical compatibility is checked before pending migrations and again after
  startup against a transient schema built from the embedded migration set.

## Executable evidence

The automated suite covers clean and repeated startup, WAL configuration,
documented-schema parity, changed checksums, future versions, ledger gaps,
transaction rollback after interrupted bootstrap, schema-object removal,
service/offline contention in both orders, cross-process contention, and lock
recovery after the owner process is killed. The storage concurrency-sensitive
subset also passes repeated execution.

Local gates passed: complete Go tests, `go vet`, documentation validation,
contract-schema validation, diff hygiene, and test-binary cross-compilation for
Linux AMD64, Linux ARM64, and Windows AMD64. Race execution remains a Linux CI
gate because the Windows development host has CGO disabled.

Independent QA initially found one high and three medium findings. The
implementation corrected all four; the second review reported no remaining
critical or high finding. A suggested future-version safeguard was also
implemented before publication.

## Publication gates

The first pull-request run exposed a timing-dependent shutdown test: under the
race detector, a fixed delay cancelled startup while migration 1 was still
running. The test was changed to wait for the observable listening state before
cancellation and passed 50 local repetitions. The replacement CI run passed the
race suite, documentation checks, and both Linux cross-builds.

On the representative 1 GB Linux AMD64 VM, the corrected revision passed the
complete race suite and ten repetitions of the lock, migration, drift, rollback,
and interrupted-owner subset. The VM checkout was then returned cleanly to its
main branch.
