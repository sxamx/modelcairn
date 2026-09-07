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

### Secret encoding

The v1 cipher's associated data starts with the ASCII bytes
`modelcairn/secret/v1` followed by a zero byte. Next come installation ID and
secret ID, each encoded as a four-byte unsigned big-endian byte length followed
by the ID bytes. Resource version and key version follow as eight-byte unsigned
big-endian integers. IDs are nonempty and at most 128 bytes; versions are positive.
Changing this encoding requires an explicit format migration.

The fingerprint key uses HKDF-SHA-256 with the 32-byte master key as input,
installation ID bytes as salt, `modelcairn/secret-fingerprint/v1` as info, and
32 output bytes. HMAC-SHA-256 authenticates the secret bytes; its first 12 bytes
are encoded as unpadded base64url with prefix `mc_fp_`. This is a display-only
identifier, not a password verifier or an authentication token.

`installation_state.key_check` is HMAC-SHA-256 under the active master key over
the ASCII domain `modelcairn/master-key-check/v1`, a zero byte, and the
installation ID bytes. It lets an empty installation reject a substituted key;
it is neither key material nor an exportable credential.

### Publication

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

On Linux, boundaries 3–4 use a no-replace rename followed by directory `fsync`.
Windows uses a no-replace `MoveFileEx` with `MOVEFILE_WRITE_THROUGH` because its
filesystem API does not generally expose directory `fsync`; the temporary key
file is flushed before either publication mechanism.

On Unix, the data directory, keyring, and key files must be owned by the effective
service UID in addition to their required modes. On Windows, ModelCairn applies a
protected DACL granting full control only to the service identity, Local System,
and built-in Administrators, verifies the key owner against the service identity,
and rejects any additional allow entry before reading key bytes.

An interruption before database commit leaves an unreferenced new key; an
interruption after commit leaves both required and obsolete keys. Startup verifies
all referenced versions before readiness and may garbage-collect only unreferenced
versions. A missing referenced version fails closed. Kill-point tests cover every
numbered boundary, including directory sync and garbage collection, and verify
that ciphertext and fingerprint always correspond to the committed key version.
