# Milestone 6 — Installation and recovery

[Español](hito-06-plan-tecnico.es.md)

Status: completed on September 15, 2026.
Prerequisite: Milestone 5 merged and accepted.

## Outcome

Deliver reproducible native installation for Linux AMD64 and ARM64 plus complete
recovery capable of rebuilding a functional installation on a clean VM. During
installation the operator chooses automatic startup, may operate through localhost,
a private network, or Tailscale Serve, and can create/verify/restore an MCB1 backup
without exposing passwords or secrets.

## Boundaries

- `systemd` is primary; Docker remains outside this milestone.
- The installer does not open ports, modify firewalls, or install Tailscale.
- Tailscale Serve is documented and checked but remains operator-managed.
- MCB1 includes a consistent database and re-encryptable secrets, not TLS/SSH keys,
  system logs, or the original master key.
- Restore never replaces active state before full authentication, bounds, integrity,
  migrations, and permissions have passed.

## Delivery blocks

1. **Installation contract:** paths, identity, permissions, prerequisites, update,
   conservative uninstall, and access-mode matrix.
2. **Native installer:** idempotent script, local or published artifact, hardened
   `systemd` unit, and explicit automatic-start choice.
3. **Guided bootstrap:** terminal welcome, safe administrator creation, minimum
   settings, and web next step; passwords are never arguments.
4. **MCB1 backup:** consistent SQLite snapshot, streaming secrets, age/scrypt,
   bounded tar, private atomic output, and post-write verification.
5. **Generational restore:** full preflight, new master key, re-encryption, session
   invalidation, atomic activation, and verified rollback.
6. **Operations:** service diagnosis, health/readiness, logs, manual update, and
   localhost/private-network/Tailscale Serve guides.
7. **Closeout:** clean-VM install and restore, recovered route test, induced failures,
   RAM/disk budget, CI, and grouped QA.

## Verified progress

- [x] Installation contract and access matrix.
- [x] Native installer and `systemd` unit.
- [x] Guided bootstrap without secrets in arguments.
- [x] Localhost, private-network, and Tailscale Serve guide.
- [x] MCB1 backup and verification without restore.
- [x] Generational restore and rollback.
- [x] Complete operations, VM trial, benchmark, and grouped closeout.

## Initial decisions

- Recommended layout: `/usr/local/bin/modelcairn`, `/etc/modelcairn`,
  `/var/lib/modelcairn`, and `/etc/systemd/system/modelcairn.service`.
- The service has a dedicated system identity with no shell or home directory.
- Interactive and documented non-interactive modes accept no password/API key as a
  direct option.
- Automatic startup is recommended but enabled only from the captured choice;
  starting now and enabling at boot are separate decisions.
- Migration to the generational layout remains compatible with current data and
  never silently moves an existing installation.
- Backups are verified when created; verify-only does not write to the data dir.

## Exit criteria

1. A supported empty VM reaches a ready console through a guided flow.
2. Re-running the installer preserves settings/data and explains changes.
3. Permissions prevent unrelated users from reading key material.
4. All three access modes have tested instructions without firewall mutation.
5. Backup/restore obey MCB1 bounds and leave no secret plaintext on disk.
6. Wrong password, truncation, low space, or failed migration preserve active data.
7. Old admin sessions fail after restore and a selected route works without
   re-entering its API key.
8. Suite, CI, clean VM, benchmark, and grouped QA have no open critical/high issue.

## Verification order

Unit tests cover parsers, bounds, and permissions; integrations use temporary
directories and real processes. The VM is used once per batch for install, reboot,
restore, and measurement. Independent QA runs after functional blocks rather than
after each edit.
