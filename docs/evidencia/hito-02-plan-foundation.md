# Milestone 2 — Plan authentication foundation

[Español](hito-02-plan-foundation.es.md)

Status: internal foundation reviewed; delivery item 5 remains in progress.
Recorded: 2026-09-08.

## Implemented

- Configuration resolution preserves omitted optional values on updates, applies
  creation defaults, replaces arrays, and checks supplied identity/version values.
- Read-only preparation reports sorted create/update/noop/delete operations and
  validates the resulting graph against the catalog.
- Plan tokens use HMAC-SHA-256 with an installation-bound, purpose-separated HKDF
  key. Tokens bind installation, key version, configuration revision, operation
  digest, observed resource identities/versions/absences, nonce and timestamps.
- The internal operation digest is a base64url-encoded SHA-256. Integration must
  hash canonical desired configuration plus operation options, including deletion
  permission; it must not hash redacted output or omit security-relevant options.
- Transaction execution locks key state through commit, verifies bindings, consumes
  the nonce and runs mutations in one SQLite transaction. A failed mutation rolls
  back nonce consumption too. Successful no-ops consume their token. An uncertain
  commit disables the open secret store until it is reopened.
- The verification clock is read after acquiring locks and reading the snapshot,
  so waiting cannot extend a token's lifetime.

## Verification

- New token tests pass locally: altered signatures; authenticated invalid claims;
  cross-installation/key-rotation rejection; clock boundaries; snapshot conflicts;
  rollback; no-op reuse; concurrent double consumption; expiry during snapshot.
- Local `go test ./... -skip '^TestKernelReleasesLockAfterOwnerProcessDies$'`
  and `go vet ./...` pass. The excluded existing Windows test encounters an
  access-denied error when terminating its helper process.
- The complete compiled storage test suite passes on the representative Linux VM,
  including that process test. Its documented schema fixture was supplied in an
  isolated temporary directory. `/usr/bin/time -v`: maximum RSS 30,852 KiB,
  swaps 0, elapsed 6.38 seconds. This measures tests, not production runtime.
- Independent static QA found the stale-clock issue; the correction and targeted
  regression tests were reviewed and accepted. QA did not independently execute
  tests because its Windows sandbox could not establish temporary-directory ACLs.

## Integration still required

This is not a completed configuration CLI or an accepted delivery item 5.
Connect real database snapshots and canonical operation digests to the executor;
implement multi-resource mutations, revision changes and audit within its
transaction; add plan files, interactive confirmation, bounded input, export,
secret commands and integration tests. Callbacks must use the supplied transaction
only, never open a second database operation or re-enter SecretStore. The executor
does not automatically build a snapshot, mutate resources or increment revisions.
CI and end-to-end CLI QA remain part of the subsequent delivery.
