# Milestone 6 evidence — installation and recovery

[Español](hito-06-instalacion-y-recuperacion.es.md)

Status on September 15, 2026: implementation complete; operational closeout in
progress. This evidence does not declare the milestone complete while the recovered
route and test-installation closeout remain pending on the VM.

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

## Remaining closeout

- complete route → backup → restore → route without secret re-entry;
- exercise wrong passphrase, truncated file, session invalidation, and rollback;
- decide and perform conservative cleanup of the test artifacts.
