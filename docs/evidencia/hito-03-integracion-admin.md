# Administrative integration evidence — Milestone 3

[Español](hito-03-integracion-admin.es.md)

- Date: September 12, 2026
- Status: local gate passed; CI and VM measurement pending

`scripts/verify-hito3.sh` builds a clean binary and uses a temporary installation.
It covers bootstrap, startup/readiness, login, session recovery, secret creation
and metadata, configuration plan/apply/export, AgentToken status/issue/revoke, and
logout. It confirms that logout removes the local session, password and API key do
not enter logs/export, and the bearer appears only at its intentional one-time
delivery boundary. Temporary artifacts are removed on exit.

The gate passed locally together with all Go tests, `go vet`, and documentation
validation. CI runs it on Linux from this increment. This evidence does not replace
representative RAM/latency measurement on the small VM or the final independent
milestone review.
