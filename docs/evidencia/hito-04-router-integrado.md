# Milestone 4 integrated router evidence

[Español](hito-04-router-integrado.es.md)

- Status: Blocks 1–6 implementation and QA verified; final acceptance pending.
- Date: September 12, 2026.
- Surface: `POST /v1/chat/completions`, normal and SSE streaming.

## Executable evidence

The automated suite demonstrates:

- AgentToken authentication and route authorization before revealing or contacting
  destinations;
- logical alias resolution through one published immutable snapshot;
- capability, status, and cooldown filtering;
- stable Credential-to-Egress affinity;
- upstream physical-model substitution and response alias restoration;
- bounded sequential fallback for `429`, transient failures, and eligible invalid
  responses;
- no fallback for terminal errors or ambiguous sends;
- SSE validation before commitment and no destination splicing afterward;
- cancellation propagation and a `partial` outcome after committed interruption;
- transactional Request, Attempt, rate-limit observation, and cooldown persistence
  without prompt, response, or secret content.

The `scripts/verify-hito4.sh` gate starts the real binary and a deterministic
loopback upstream. It performs bootstrap, login, configuration publication,
AgentToken issuance, one normal call, and batches of 1, 2, 5, 10, and 20 streams.
It also measures RSS, swap, and latency and searches local storage for content
canaries and logs for secrets.

The integrated HTTP matrix induces `429`, `503`, invalid JSON response,
terminal error, timeout, and cancellation. It checks attempts, fallback reasons,
cooldowns, and persisted outcomes. It also proves SSE fallback only before
commitment, partial interruption without a second destination, and tool-call
round-trip.

## Grouped independent QA

The independent review found shallow tool-call validation, disconnected token/TTFT
metrics, and ambiguous upstream JSON. The findings were fixed with structural
validation, metric propagation through SQLite, and rejection of duplicate keys,
invalid UTF-8, and excessive depth in upstream responses. Their regressions are
part of the suite.

## Current results

- The complete Go suite, `go vet`, documentation validators, reachable
  vulnerability scan, and race-enabled tests pass in CI.
- Linux AMD64 and ARM64 cross-builds pass.
- The full Linux integration gate passes in CI.
- One additional local run completed all five concurrency levels with a peak near
  59 MiB RSS; this does not replace the representative VM measurement.

## Remaining before milestone acceptance

- run the gate for a representative duration on the target VM;
- publish a redacted report omitting hostname, IP, user, paths, credentials, and
  revision identifiers;
- rerun every gate on the final revision.

This evidence does not yet claim remote relay, HTTP CONNECT, SOCKS, visual editor,
adaptive estimator, or additional protocol dialect support.
