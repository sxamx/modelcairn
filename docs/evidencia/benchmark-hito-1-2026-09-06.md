# Milestone 1 representative resource benchmark

[Español](benchmark-hito-1-2026-09-06.es.md)

- Result: **passed**
- Recorded: September 6, 2026 (UTC)
- Product revision: `6c69032`
- Environment: Oracle Linux VM, AMD64, 2 logical CPUs, 975,064 KiB RAM
- Kernel: Linux 5.15.0 Oracle build
- Toolchain: Go 1.27.0, Linux AMD64
- Scenario: empty in-memory state, no provider routes, loopback health probe
- Duration and interval: 900 seconds, sampled every 5 seconds
- Samples: 180
- Average RSS: 6,328 KiB
- Peak RSS: 6,328 KiB
- Peak process swap: 0 KiB
- Enforced peak budget: 131,072 KiB
- SQLite initialization probe peak RSS: 10,288 KiB
- Empty-server binary size: 6,795,527 bytes
- SQLite test binary size: 11,168,761 bytes

The process used 4.8% of the enforced 128 MiB empty-server budget and 0.65% of
the VM's physical memory. The SQLite figure includes the Go test harness and is
therefore a conservative integration-spike measurement, not a product startup
measurement.

System-wide available memory changed from 599,164 KiB to 580,856 KiB during the
run. System swap free changed from 1,910,128 KiB to 1,910,640 KiB, while the
ModelCairn process itself reported zero swap. These system-wide values include
unrelated resident services and are recorded as context, not attributed to
ModelCairn.

The raw report was reviewed before publication. Hostname, IP address, username,
the full commit identifier, and other unique environment identifiers are omitted.
The executable script and pinned module files remain the reproducible source for
the measurement method.

## Decision

Milestone 1's representative empty-server resource gate passes. This result
validates the current skeleton only; persistence, routing, the console, sustained
load, retention, and egress relays require their own later budgets and gates.
