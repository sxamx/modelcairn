# Administrative integration evidence — Milestone 3

[Español](hito-03-integracion-admin.es.md)

- Date: September 12, 2026
- Status: accepted; local gate, representative VM measurement and increment CI passed

`scripts/verify-hito3.sh` builds a clean binary and uses a temporary installation.
It covers bootstrap, startup/readiness, login, session recovery, secret creation
and metadata, configuration plan/apply/export, AgentToken status/issue/revoke, and
logout. It confirms that logout removes the local session, password and API key do
not enter logs/export, and the bearer appears only at its intentional one-time
delivery boundary. Temporary artifacts are removed on exit.

The gate passed locally together with all Go tests, `go vet`, documentation
validation and the enforced 128 MiB memory budget. CI passed on Linux for the
published increment, including race detection and AMD64/ARM64 cross-builds. The
[representative measurement](benchmark-hito-03-2026-09-12.md) also passed.

The final independent review found and then verified corrections for login-history
retention, failed persistence retry, shutdown concurrency and the exact 24-hour
boundary. Migration 0004, focused regressions and the local retention map close
those findings. No prompt, response, password, address or bearer is stored in the
failed-login statistics.
