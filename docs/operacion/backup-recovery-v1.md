# MCB1 backup and recovery v1

[Español](backup-recuperacion-v1.es.md)

These are offline operations: stop ModelCairn so the CLI can acquire the exclusive
lock. No command accepts the passphrase as an argument.

## Create and verify

The destination directory must exist and the output file must not:

```sh
sudo systemctl stop modelcairn.service
sudo -u modelcairn /usr/local/bin/modelcairn backup create \
  --data-dir /var/lib/modelcairn \
  --out /private/path/modelcairn-2026-09-14.mcb.age
sudo -u modelcairn /usr/local/bin/modelcairn backup verify \
  /private/path/modelcairn-2026-09-14.mcb.age
sudo systemctl start modelcairn.service
```

The terminal requests and confirms the passphrase during creation. For automation,
standard input may be connected to an operator-managed secret descriptor; do not
use arguments, environment variables, or visible shell-history text.

`create` takes a consistent SQLite snapshot, encrypts the uncompressed tar, syncs
the file, and verifies it before renaming it to the destination. It never overwrites.

## Restore

First retain a verifiable backup of the current state. Then:

```sh
sudo -u modelcairn /usr/local/bin/modelcairn backup restore \
  --data-dir /var/lib/modelcairn \
  /private/path/modelcairn-2026-09-14.mcb.age
sudo systemctl start modelcairn.service
curl --fail http://127.0.0.1:8080/readyz
```

Restore authenticates everything before creating a generation. A second read
validates limits and checksums again, migrates the copy, creates a new identity and
master key, re-encrypts every secret, and removes administrative sessions. Only
then does it atomically switch `current`.

After readiness, sign in again and test one selected route. Agent tokens are
retained; earlier web sessions no longer work.

## Return to the previous generation

If readiness or the functional test fails:

```sh
sudo systemctl stop modelcairn.service
sudo -u modelcairn /usr/local/bin/modelcairn backup rollback \
  --data-dir /var/lib/modelcairn
sudo systemctl start modelcairn.service
curl --fail http://127.0.0.1:8080/readyz
```

Rollback selects the recorded predecessor and deletes no generation. ModelCairn v1
does not remove generations automatically either.

## Safe failures

- A wrong passphrase, truncation, unknown format, or bad checksum stops during
  preflight and creates no generation.
- An incompatible database, mismatched secret, or insufficient space removes only
  the still-inactive generation.
- An existing backup destination is never replaced.
- `installation_in_use` means another process owns the directory: stop the service
  and retry without deleting the lock file.

