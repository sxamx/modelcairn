# Milestone 2 — Integration and resources

[Español](hito-02-integracion-recursos.es.md)

Status: item 6 and Milestone 2 accepted. Recorded: 2026-09-09.

## Closure coverage

The automated suite and `scripts/verify-hito2.sh` jointly cover:

- clean, repeated, interrupted, checksum-drifted, discontinuous-history, future,
  and logically incompatible migrations;
- resource and audit rollback, optimistic conflicts, and atomic plan consumption;
- exclusive service/CLI ownership in both acquisition orders;
- rotation interruption at every durable boundary and fail-closed behavior for
  absent, incorrect, or malformed keys and ciphertext;
- a real ten-resource configuration, canonical export, no-op re-apply, and stale,
  altered, or reused plan rejection;
- secret canaries across parsing, errors, logs, plans, exports, metadata, audit,
  temporary artifacts, and the persistent data directory;
- formatting, race detector, coverage, vet, dependencies, reachable
  vulnerabilities, documentation contracts, and Linux AMD64/ARM64 builds in CI.

The product gate creates its installation through the published CLI rather than
internal fixtures. It measures the server after persisting the graph and secret;
it also attempts an offline command while the service owns the installation and
confirms that configuration did not change.

## Representative 1 GB VM

A 120-second run on Linux x86_64 with 2 logical CPUs and 975,064 KiB RAM used a
prebuilt binary because the production VM does not install Go:

| Measurement | Result | Enforced limit |
|---|---:|---:|
| Time to `/healthz` | 86 ms | informational |
| Average RSS | 11,768 KiB | 131,072 KiB |
| Peak RSS | 11,768 KiB | 131,072 KiB |
| Process swap | 0 KiB | 0 KiB expected |
| Stripped binary | 11,624,608 bytes | informational |
| Configured SQLite database | 270,336 bytes | within data limit |
| Directory before/after | 278,560 / 278,560 bytes | 16,777,216 bytes |

The run collected 119 samples. Round-trip, no-op, exclusive ownership, and canary
scanning passed. The directory did not grow while the configured idle service ran.
This is a representative startup and idle measurement with ten resources; it does
not claim routing or concurrent-load performance, which belongs to Milestone 4.

## Limits and reproducibility

CI runs the same gate for five seconds to catch regressions on every change. The VM
run lasts 120 seconds. Limits are explicit parameters and execution fails when they
are exceeded. The report retains no secret values, IP, hostname, credentials,
personal paths, or VM identifiers.

The measured binary declares public commit
`db87b3e474b6087b37fe84e89869b8dc2e3f325e`; its SHA-256 is
`3e2eb2746fffbf7f7af765279e6a3cf9547089c451ea9f932308c5071919d41e`.
These fingerprints make the artifact reproducible and verifiable; they are not
credentials or private identifiers.

Grouped QA found one high gap: `stderr` could escape canary scanning. It also found
incomplete binary traceability, stale status documentation, and two possible false
positives. All five observations were corrected as one block; focused verification
closed them with no critical or high defects. CI then passed the complete suite and
both cross-builds again.
