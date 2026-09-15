# Milestone 6 evidence — installation and recovery

[Español](hito-06-instalacion-y-recuperacion.es.md)

Status on September 15, 2026: implementation, operational trial, and conservative
cleanup complete. Closeout is subject only to final CI and merge.

## Automated gates

- Complete Go suite, `go vet`, vulnerability analysis, and contracts: passed.
- React console: 14 tests, typed generation, build, and embedded bundle: passed.
- Linux AMD64 and ARM64 builds: passed.
- Milestones 2, 3, and 4 integration gates, streaming concurrency, and basic resource
  budget: passed in CI.
- Added MCB1 cases cover a concurrently appearing destination, the 4096-secret bound,
  and safe activation when the post-link `fsync` fails.

## 1 GB target VM

The test VM exposes 952 MiB of usable RAM. The AMD64 artifact was installed with a
dedicated user, automatic startup, and loopback listening. Repeating the installer
preserved state. Observed permissions match the contract:

| Element | Owner | Mode |
|---|---|---:|
| Binary | `root:root` | `0755` |
| Service directory | `root:modelcairn` | `0750` |
| Service environment | `root:modelcairn` | `0640` |
| Data directory | `modelcairn:modelcairn` | `0700` |
| Master key and SQLite | `modelcairn:modelcairn` | `0600` |
| systemd unit | `root:root` | `0644` |

The process consumed approximately 5.8 MiB immediately after installation. After a
VM reboot, `systemd` started it automatically and health and readiness responded. A
later 120-second window collected 59 samples while polling both endpoints: average
15.8 MiB, peak 28.7 MiB, and 0 KiB swap. The binary measured 13.4 MiB and configured
state approximately 312 KiB. The peak remains comfortably within the 128 MiB limit.

## Finding from the real installation

Another service already listened on the default port. The first installer version
could report success even though ModelCairn then exited because of the collision. It
now verifies that the process remains active, stops the restart loop, and returns a
diagnosis. Repetition confirmed that an occupied port is rejected and ModelCairn is
left stopped. The clean installation continued on a free loopback port without
altering the pre-existing service.

## Functional recovery

The configured installation passed the complete path:

- MCB1 creation and verification, with no secret canary found in the encrypted file;
- wrong passphrase and truncated file rejected without changing the generation;
- a Chat Completions route worked against a local upstream before restore;
- restore selected a new generation and invalidated the pre-restore admin session;
- the same route worked again without re-entering the provider secret;
- rollback selected the exact predecessor and recovered readiness.

Administrator and backup passwords were random, existed only in memory during the
test, and were not published. The upstream, API key, and prompts were test canaries.

## Conservative cleanup

The uninstaller removed the unit and binary, and only test-created temporary files
were deleted. Both development data directories remain owned by
`modelcairn:modelcairn` with mode `0700`; they were neither deleted nor published.
Final CI and merge are verified outside this VM evidence.
