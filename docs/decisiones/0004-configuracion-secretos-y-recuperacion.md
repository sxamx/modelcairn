# ADR-0004: Configuration, secrets, and recovery

[Español](0004-configuracion-secretos-y-recuperacion.es.md)

- Status: accepted
- Date: 2026-09-05
- Owners: primary maintainer and technical design

## Context

Configuration must be manageable both through files and through the console
without creating two contradictory states. API keys are the highest-priority local
asset, and a restoration must be able to recover a working installation.

## Decision

### Source of truth

- A local, versioned SQLite database will be the primary node's source of truth.
- The console and CLI will modify state through the same validation contract.
- YAML will be the human-readable format for importing, applying, exporting, and
  automating configuration.
- The first version will not continuously watch or reload an editable file. A
  change is applied explicitly through the CLI or API and produces an audit event.
- SQLite has one owning process. During Milestone 2, stateful CLI commands acquire
  the installation lock and require the service to be stopped. Milestone 3 moves
  online mutations behind the administrative API; the CLI then uses that API when
  the service is running. Read-only validation that needs no installed state does
  not acquire the lock.

The exact lock order and failure behavior are defined in
[SQLite ownership and keyring durability](../contratos/storage/propiedad-y-llavero-v1.md).

### Secrets during operation

- API keys will be encrypted before they are persisted.
- The installer will generate the first random master key and store it in the
  private versioned keyring accessible only to the service identity.
- The master key will not be shown in the console or included in normal exports,
  logs, or metrics.
- A missing or incorrect master key will cause the secret store to fail closed:
  routes that depend on values that cannot be decrypted will not start.

This protection reduces the impact of copying only the database, but does not
promise to protect secrets from a person with full administrative control of the
VM. Raising that level would require a different operating model.

### Export and backup

- Normal exports will contain configuration without secrets and may be versioned.
- A full backup may include configuration, data, and API keys.
- Before including secrets, the full backup will be encrypted with an
  operator-chosen password distinct from the administrative password.
- Restoration with an incorrect password will fail before applying data.
- The restoration test must demonstrate that a recovered route can make a call
  again without re-entering its API key.
- An active installation will not be overwritten without preflight checks, a prior
  backup, and explicit confirmation.

### Administrative recovery

The administrative password may be reset only through a local CLI run with
sufficient permissions on the VM. It will not depend on email or an external
service.

## Consequences

- Copying YAML does not constitute a full backup.
- Losing the VM, master key, and backup password at the same time may make secrets
  unrecoverable; this must be explained during onboarding.
- SQLite must be tested with the expected event volume, write pattern, and
  retention. If it does not meet requirements, it may be replaced through another
  ADR.
- Backup tools must avoid exposing secrets in process arguments, shell histories,
  or temporary files.

## Technical sources

- [Official SQLite description](https://www.sqlite.org/about.html)
- [Appropriate uses and concurrency limits of SQLite](https://www.sqlite.org/whentouse.html)
