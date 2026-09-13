# Milestone 4 representative benchmark

[Español](benchmark-hito-04-2026-09-13.es.md)

- Status: approved on the target VM.
- Date: September 13, 2026.
- Environment: Linux x86_64, 2 logical CPUs, 975,064 KiB total memory.
- Stripped binary: 13,316,256 bytes.
- Load: 20 batches at every concurrency level; 760 successful streams.
- Samples: 211, approximately every 50 ms.
- Average service RSS: 41,544 KiB.
- Peak service RSS: 56,584 KiB.
- Enforced maximum budget: 131,072 KiB.
- Peak service swap: 0 KiB.

| Concurrent streams | Successful streams | Average latency | Maximum latency |
|---:|---:|---:|---:|
| 1 | 20 | 0.057 s | 0.068 s |
| 2 | 40 | 0.067 s | 0.090 s |
| 5 | 100 | 0.106 s | 0.303 s |
| 10 | 200 | 0.128 s | 0.236 s |
| 20 | 400 | 0.191 s | 0.395 s |

## Scope

`scripts/verify-hito4.sh` ran the real binary against a deterministic loopback
HTTP upstream. The path included bootstrap, admin login, encrypted secret,
configuration publication, AgentToken issuance, one normal call, and streaming at
all five concurrency levels. Every response ended with `[DONE]`, preserved the
logical alias, and returned no ModelCairn error.

The gate also checked that prompt canaries did not occur in the data directory and
that the provider secret did not occur in logs. The binary internally verified
that it matched the evaluated revision before starting, but the public report
omits that identifier.

## Interpretation

The peak was approximately 55.3 MiB, well below the 128 MiB budget, with no swap.
Twenty concurrent streams completed successfully, so that concurrency is
demonstrated for this fixture on the target VM. The figures do not predict external
provider latency: networking was local and the upstream simulated a short
two-part delivery.

The public evidence omits hostname, IP, user, temporary paths, credentials, and
revision identifiers.
