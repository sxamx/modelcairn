# Milestone 7 — System and security validation evidence

[Español](hito-07-validacion-de-sistema.es.md)

- Date: September 19, 2026
- Tested revision: `0cdcf02` (PR #26 branch)
- Result: automated and representative-VM blocks passed; Phase 1 closeout remains
  pending the grouped independent QA review.

No hostname, IP, SSH user, credential, personal path, or run ID is published. Keys,
passwords, sessions, and AgentTokens used by the run were temporary.

## Automated gates

CI passed on the tested revision:

- Linux AMD64 and ARM64;
- `go vet`, race detector, coverage, and the complete Go suite;
- reachable-vulnerability scanning;
- 14 console tests, build, and reproducible embedded assets;
- documentation contracts, SQLite schemas, and installation scripts;
- Milestone 2, 3, and 4 integration gates and resource/concurrency smokes.

`go test ./...`, all 14 web tests, and the 20-entry Phase 1 manifest also passed
locally. The race detector could not run on Windows because local Go lacked CGO;
the Linux CI run is authoritative evidence.

## Covered induced failures

| Risk | Executable evidence |
|---|---|
| 429/5xx, classification, cooldown | `TestExecuteReturnsSafeNonSuccessMetadataWithoutBody`, `TestOperationalRecorderNormalizesRateLimitAndCreatesCooldown` |
| timeout/cancel/disconnect | `TestScriptedDelayHonorsCancellation`, `TestScriptedDisconnectsAndExhaustionIsExplicit`, `TestEngineStopsImmediatelyWhenCallerIsCancelled` |
| stream before/after commitment | `TestStreamEngineFallsBackBeforeCommitment`, `TestStreamEngineNeverFallsBackAfterCommitment`, `streaming` suite |
| SSRF, private network, redirect, invalid destination | `TestForbiddenAddressPolicy`, `TestExecuteRejectsRedirectInvalidResponseAndPublicLoopback`, private parsing tests |
| total attempt bound | `TestEngineHonorsAttemptCapAndRejectsStreaming` |
| concurrent/stale configuration | `TestExecutePlanConcurrentConsumption`, `TestManagerRejectsStaleAndOptionChangedPlans` |
| interrupted migration/drift | `TestInterruptedInitialMigrationLeavesNoCommittedSubset` and `lifecycle` suite |
| interrupted rotation | `TestRotationSurvivesProcessKillAtEveryBoundary` |
| storage capacity | `TestRequireFreeSpaceAcceptsSmallWriteAndRejectsImpossibleWrite` |
| corrupt/wrong-passphrase backup and restore | `backupmcb1` suite and Milestone 6 functional evidence |

All passed in CI. Canary coverage is deliberately split by surface:

- the Milestone 2 gate scans its complete temporary directory after parser,
  errors, plan, apply, export, metadata, SQLite, and logs;
- the Milestone 3 gate covers the admin API/export and rejects password/secret in
  logs;
- the Milestone 4 gate rejects prompt, response, or secret in SQLite and service
  logs;
- web tests verify the secret only crosses its write request, is cleared from the
  field, and never enters browser storage;
- the Milestone 6 suite/evidence verifies encrypted backup and no plaintext canary.

No surface found the canary. HTTP responses that necessarily return requested
content are not considered disclosure.

## Validated retention

Failed-login statistics support the 24-hour initial value, arbitrary duration up
to 100 years, and unlimited (`0`). Tests covered pruning without a new failure, the
inclusive boundary, saturation, and safe transition retry. The console exercised
plan/apply for unlimited retention.

General request, attempt, observation, and audit retention remains RF-202. This
validation neither claims it is implemented nor makes it a Phase 1 blocker.

## Representative 1 GB VM

The integrated Milestone 4 harness was extended with sustained load. It uses the
real binary, bootstrap, admin session, published configuration, AgentToken, direct
adapter, persistence, and a deterministic local HTTP upstream. For 600 seconds it
held 10 concurrent streams and queried the overview once per second.

| Measurement | Result |
|---|---:|
| VM total memory | 975,064 KiB |
| Successful sustained streams | 15,284 |
| Successful admin queries | 358 |
| Average/peak RSS | 25,540 / 57,336 KiB |
| RSS budget | 131,072 KiB |
| Peak process swap | 0 KiB |
| Process CPU | 173.580 s; 28.786% of one logical CPU on average |
| Sustained average/maximum latency | 0.198811 / 0.987245 s |
| Final data directory | 18,931,464 bytes |
| Final SQLite/WAL | 14,733,312 / 4,165,352 bytes |
| Linux AMD64 binary | 14,094,496 bytes |

The run exited zero. The process stayed below budget, used no swap, and the report
contains metadata only. Prior test data under `/var/lib` retained its owner and
mode; this run used an isolated temporary directory.

## Security and privacy

The [threat model](../seguridad/phase-1-threat-model.md) links assets, boundaries,
controls, and residual risks. Suites covered authenticated encryption, fail-closed
keyring, redaction, sessions/CSRF, login limits, AgentTokens, permissions, SSRF,
single-use plans, and generational recovery. Prompts/responses were not persisted
and no built-in external telemetry exists.

## Evidence limits

- The upstream is local and deterministic: it measures ModelCairn, not a public
  provider's latency or availability.
- RSS excludes kernel page cache; CPU covers the ModelCairn process only.
- The run does not validate relays or the adaptive estimator, both outside Phase 1.
- Concurrent console load queried overview; other views are covered by web/HTTP
  tests rather than this 10-minute load.
- Final acceptance depends on grouped QA and resolution of any blocking findings.
