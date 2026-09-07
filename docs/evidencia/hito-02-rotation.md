# Milestone 2 — Master-key rotation

[Español](hito-02-rotation.es.md)

Status: implemented and locally tested; full issue #10 acceptance remains pending.

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

Limits: killing processes does not simulate power loss. Linux runtime and race
validation are assigned to CI; representative VM execution requires Tailscale
reauthentication. VM memory measurements and the complete issue acceptance gate
are not claimed by this checkpoint. CLI exposure belongs to the later CLI item.
