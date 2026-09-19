# Phase 1 operations runbook

[Español](runbook-fase-1.es.md)

This index guides a native Linux instance. Linked procedures are authoritative;
do not copy commands from historical evidence without reviewing them.

## Prepare and install

1. Review [network access](acceso-red-v1.md) and choose loopback, direct TLS, or
   proxy TLS before exposing ports.
2. Follow [installation and upgrade](linux-installation-v1.md).
3. Bootstrap the admin with a new password and configure the first route through
   the console or CLI.
4. Check `/healthz` and `/readyz`; readiness must identify a degraded dependency.

## Normal operation

- Use Activity for content-free requests and attempts.
- Watch RSS/swap, data-directory space, SQLite growth, 429/5xx errors, and cooldowns.
- Never move a credential's egress automatically. A deliberate change must be
  planned, warned, and audited.
- Keep HTTP on loopback. For Tailscale/another private network, use a documented
  HTTPS mode.

## Quick diagnosis

1. **Process down:** inspect `systemctl status modelcairn` and journal; an occupied
   port must fail rather than report a false successful start.
2. **Not ready:** inspect `/readyz`, then storage, configuration, and keyring.
3. **Route fails:** confirm AgentToken, published alias, eligibility, cooldown,
   classification, and budget. Do not paste bodies or credentials into an issue.
4. **Disk grows:** measure SQLite, backups, and journal separately. General
   retention is deferred; do not manually delete the active database.
5. **Possible leak:** revoke/rotate the affected secret, preserve redacted evidence,
   and use the private channel in `SECURITY.md`.

## Recovery and maintenance

- Follow [MCB1 backup and recovery](backup-recovery-v1.md). Verify a backup before
  relying on it and test restore in a temporary installation.
- Restore creates and activates a generation; rollback returns to the exact
  predecessor. Previous sessions are invalidated.
- Use the documented generational upgrade/rollback flow; never replace the active
  binary halfway through.
- Default uninstall preserves data. Delete it only as a separate decision after
  identifying the correct absolute path.

## Before asking for help

Record version/commit, architecture, network mode, UTC timestamps, probe state, and
minimal steps. Redact API keys, tokens, cookies, passwords, prompts, responses,
private IPs, backups, and personal paths.
