# Milestone 2 — Master-key rotation

[Español](hito-02-rotation.es.md)

Status: implemented; local tests, CI, and representative execution passed.

`RotateMasterKey` publishes a private key durably, re-encrypts one row at a time
inside a transaction, updates fingerprints and installation authentication, and
commits the success audit atomically. It verifies persisted data before removing
unreferenced keys. `CollectUnusedKeys` is retryable and also removes abandoned
temporary key files. Secret logical versions and timestamps remain stable.

The Windows development test run covers twelve real process termination points:
temporary creation, file sync, rename, directory sync, a partially re-encrypted
row, before commit, after commit, verified rows, old-key unlink, GC directory
sync, temporary unlink and final cleanup sync. Every restart decrypts the stored
values, checks fingerprints and audit consistency, and retains the required key.

Additional tests cover transaction rollback when audit insertion fails, empty
installations, nonactive referenced keys, missing authentication evidence,
post-commit corruption and missing keys, canceled commit, and safe reentrant
secret callbacks. Detection of invalid key state disables further operations
until reopening, including when discovered before rotation starts.

Independent static QA identified missing recovery tests, abandoned temporaries,
callback deadlock and writes after detecting a missing active key. Regression
tests and implementation changes address those findings. The earlier key-file
permission error shadowing was also corrected. Full local Go tests, vet and
documentation/schema validation passed during this work.

CI reran tests with the race detector, validated documentation and schema, built
Linux AMD64/ARM64, and found no reachable vulnerabilities. On the representative
998,465,536-byte RAM VM on September 8, 2026:

- the server remained up for 120 seconds at 11,476 KiB average and peak RSS with
  no swap; `/healthz` returned 200 and `/readyz` returned the expected 503 without
  applied configuration;
- targeted commit, rollback, twelve-interruption, empty-installation, and
  fail-closed tests completed successfully at 16,496 KiB maximum RSS and zero
  swap;
- the test binary represented commit `6873aa9` and was checksum-verified after
  transfer before execution.

Limits: process termination does not simulate power loss. CLI exposure belongs to
the CLI delivery item. This evidence accepts the master-key and secret-store item,
not Milestone 2 as a whole.
