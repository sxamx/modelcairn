# Resource benchmark procedure

[Español](benchmarks.es.md)

Milestone 1 uses `scripts/benchmark-idle.sh` to measure the empty Linux server.
The script builds a stripped binary, waits for `/healthz`, samples `VmRSS`, writes
a Markdown evidence report, and fails when the configured peak budget is exceeded.

CI runs a ten-second smoke measurement with a 128 MiB ceiling. This deliberately
uses half of the provisional 256 MiB steady-state product budget, reserving room
for later modules. It detects large skeleton regressions but is **not**
representative evidence for the 1 GB VM target.
Milestone 1 closes only after the default 15-minute run succeeds on the documented
reference VM:

```sh
MAX_RSS_KIB=131072 bash scripts/benchmark-idle.sh
```

The generated `benchmark-results/` directory is intentionally ignored. A reviewed,
redacted representative report will be copied into documentation when accepted.
Hostnames, IP addresses, usernames, and provider credentials must never appear in
the published report.

Sampling reads the process RSS and swap from `/proc`; measurements have the
granularity of the configured interval and do not include kernel page cache or
child processes. The script also records memory and swap before/after sampling,
the largest resident processes before startup, the exact binary version and the
empty-server configuration. It also compiles a SQLite-linked test artifact and
records its size and peak RSS during schema initialization. That SQLite number is
a conservative spike measurement because it includes the Go test harness; the
product-linked cost will be measured again when persistence enters startup.
