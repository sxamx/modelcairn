# Milestone 5 console representative benchmark

[Español](benchmark-hito-05-2026-09-13.es.md)

- Status: passed on the target VM.
- Date: September 13, 2026.
- Environment: Linux x86_64, 2 logical CPUs, approximately 1 GB RAM.
- Duration: 180 seconds; 90 samples at two-second intervals.
- Stripped binary with embedded console: 13,725,856 bytes.
- Average RSS: 14,640 KiB.
- Peak RSS: 14,640 KiB.
- Peak process swap: 0 KiB.
- Enforced peak budget: 131,072 KiB.

## Scope

`scripts/benchmark-console.sh` ran the actual Milestone 5 binary from a temporary
directory. Before sampling it checked `/healthz`, the SPA document, the PWA
manifest, and the no-cache policy on the main document. The process was stopped
afterwards and did not alter the permanent installation.

This benchmark measures the idle cost of the server with its embedded console;
it does not repeat the concurrent engine load already covered by Milestone 4.
Public evidence omits hostname, IP, user, temporary paths, and revision
identifiers.
