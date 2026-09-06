# SQLite ownership and keyring durability v1

[Español](propiedad-y-llavero-v1.es.md)

## Process ownership

The configured data directory is private to the service identity. Both `serve` and
every stateful offline CLI command open `modelcairn.lock` in that directory without
following symbolic links and take the same non-blocking exclusive operating-system
lock **before** opening SQLite or key files. The service holds its descriptor for
its entire lifetime; a CLI command holds it through commit, sync, and close.

Failure to acquire the lock returns `installation_in_use` and touches neither
SQLite nor the keyring. The lock file may remain after exit, but the kernel releases
the lock when the descriptor closes or the process dies; file existence is never
used as lock state. Tests start service and CLI contenders in both orders and prove
that exactly one owner opens the database.

## Private keyring

The keyring directory has mode `0700`. Each immutable `v<version>.key` file has
mode `0600`, contains exactly 32 random bytes, and is created without following
links. A new key is published through these durable boundaries:

1. exclusively create a same-directory private temporary file;
2. write exactly 32 bytes, sync the file, and close it;
3. atomically rename it to the unused versioned filename;
4. sync the keyring directory;
5. in one SQLite transaction, re-encrypt every secret, update its `key_version`,
   recompute its display fingerprint, update the active version, and commit;
6. reopen and decrypt every row as verification;
7. remove an old key only after a committed query proves no row references it,
   then sync the keyring directory.

An interruption before database commit leaves an unreferenced new key; an
interruption after commit leaves both required and obsolete keys. Startup verifies
all referenced versions before readiness and may garbage-collect only unreferenced
versions. A missing referenced version fails closed. Kill-point tests cover every
numbered boundary, including directory sync and garbage collection, and verify
that ciphertext and fingerprint always correspond to the committed key version.
