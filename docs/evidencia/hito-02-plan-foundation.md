# Milestone 2 — Plan authentication foundation

[Español](hito-02-plan-foundation.es.md)

Status: CLI and persistence integration implemented; delivery item 5 under verification.
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
- Planning now reads the configuration revision, resource identities and versions,
  observed absences, and secret-name catalog in one SQLite snapshot. Its operation
  digest covers canonical desired configuration and deletion permission.
- Applying recomputes that snapshot under the transaction, rejects stale or
  option-changed plans, orders dependency writes before dependent-first deletions and
  increments the global configuration revision once. A true no-op consumes its
  token without changing the configuration revision.
- Full export reconstructs every persisted configuration resource and applies the
  shared redactor at the output boundary.
- The CLI provides `config validate/plan/apply/export` and
  `secret set/metadata/rotate/delete`. Automated apply requires a plan file;
  interactive mode retains the installation lock from planning through
  confirmation. Secret input uses a no-echo terminal or bounded stdin.

## Verification

- New token tests pass locally: altered signatures; authenticated invalid claims;
  cross-installation/key-rotation rejection; clock boundaries; snapshot conflicts;
  rollback; no-op reuse; concurrent double consumption; expiry during snapshot.
- Persistence integration tests cover creation and export of the documented
  ten-resource example, stale and option-changed plans, no-op revision behavior,
  complete dependency-ordered deletion, reference migration before dependency
  deletion, redacted plan serialization, rollback of
  an unsafe intermediate relation, and preservation of historical destination IDs.
- Command tests cover validation without state creation, exclusive plan-file
  creation, apply and reuse, export, confirmation and cancellation, invalid plans
  before state opening, and the complete secret lifecycle without disclosure.
- The local gate passes `go test ./...` (apart from the process-termination test
  reserved for Linux), `go vet ./...`, validation of 86 documents, and `diff --check`.
- The Linux AMD64 binary completed the secret, validation, plan, apply, export,
  metadata, and rotation cycle on the representative VM. The canary did not occur
  in artifacts or the data directory. The `plan` operation measured 13,148 KiB
  maximum RSS, zero swaps, and 0.02 seconds; this is a single-operation measurement,
  not a load test.
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

This is not yet an accepted delivery item 5. Full-block CI and final grouped QA
remain. Callbacks must use the supplied transaction
only, never open a second database operation or re-enter SecretStore. The executor
is connected through `config.Manager`; direct callers still must not construct
unvalidated storage mutations. CI and end-to-end CLI QA remain part of the
subsequent delivery.

v1alpha1 preserves SQLite's relationship guards throughout apply. Projection
updates omit unchanged relational columns, so ordinary metadata, status and
capability edits remain possible. A coordinated
relationship change or a swap between values protected by a UNIQUE constraint can
therefore be valid as a final graph but impossible as an intermediate SQLite state;
the transaction rejects and rolls back it. Operators can express a replacement as
an explicit replacement with a distinct constrained value, reference migration,
and dependency-ordered deletion, which assigns a new identity
where deletion was requested. Supporting identity-preserving swaps requires a
separately reviewed transition design; this foundation does not weaken persistent
constraints or rewrite historical destination relationships.
