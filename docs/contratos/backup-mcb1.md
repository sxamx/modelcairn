# MCB1 backup profile over age

[Español](backup-mcb1.es.md)

- Status: Phase 1 contract
- Objective: portable complete restoration and streaming without designing cryptography

## Outer format

MCB1 invents neither a cipher nor binary framing. It is an application profile of
the standard **age v1** format with one password recipient (`scrypt`). The file
begins with the normal `age-encryption.org/v1` header; the recommended extension is
`.mcb.age`. ModelCairn uses the version-pinned `filippo.io/age` Go library and its
streaming interfaces.

The reader accepts only:

- binary age v1, without armor;
- exactly one `scrypt` stanza, without plugins or other recipients;
- a work factor within the range allowed by the pinned version and local policy;
- a payload authenticated by the age format.

ModelCairn v1 creates files with `logN=16` and accepts only values from 15 through
18. This range bounds work and memory imposed by an untrusted file; changing it
requires a benchmark and a compatible profile review.

There is no circular checksum or cryptographic parameters defined by a custom
header. The age library limits and interprets its KDF; ModelCairn rejects a file
before creating a generation if its header, recipient, or physical size violates
this policy.

## Streaming payload

The age plaintext is an **uncompressed** POSIX tar in Phase 1. Avoiding compression
reduces complexity, RAM spikes, and expansion attacks. It permits only:

- `manifest.json` (maximum 1 MiB);
- `database.sqlite` (default maximum 64 GiB and limited by free space);
- `secrets.jsonl` (maximum 64 MiB and 4096 records, one record per line);
- `checksums.json` (maximum 1 MiB).

At most four entries and 65 GiB of plaintext are allowed. Reading is sequential
with bounded buffers; absolute paths, `..`, duplicates, links, devices, negative
sizes, overflow, and undeclared trailing bytes are rejected. SHA-256 for each
entry is computed while writing and compared with `checksums.json` after age has
authenticated the complete stream.

`checksums.json` declares `algorithm: SHA-256` and hexadecimal hashes for
`manifest.json`, `database.sqlite`, and `secrets.jsonl`. It does not hash itself,
which would create a circular reference. The manifest lists all four entries, in
that order, with their exact sizes.

Each `secrets.jsonl` line is a strict JSON object containing `id`, `name`,
`resourceVersion`, `createdAt`, `updatedAt`, and `value`. JSON represents `value`
as base64. ID, name, version, and timestamps must match the snapshot row, preserving
credential references. MCB1 v1 accepts values from 1 through 16,384 bytes.

Secrets are decrypted one by one during creation and enter the age writer directly;
they are never written to a plaintext temporary file. During restore they are read
one by one from age and re-encrypted directly with the new master key.

## Consistent creation

1. Check space, limits, and password through TTY or a secure descriptor.
2. Open a SQLite snapshot with its backup API.
3. Create the tar through the age writer in a private temporary file.
4. Sync, close—which finalizes authentication—and rename on the same filesystem.
5. Perform a verification read before reporting success.

The destination must not exist. The encrypted file is created with mode `0600` in
a temporary file in the same directory and published by rename; failure never
replaces an earlier backup.

## Generational restoration

Active data lives in `data/generations/<id>/` with `database.sqlite` and
`master.key`; `data/current` points to a complete generation.

The implementation stores that key as `keys/v1.key` inside the generation. An
older installation without `current` retains its flat layout until the operator
explicitly restores. The first generation records `legacy` as its predecessor;
later ones record the prior generation ID. Rollback changes only `current` and
deletes no data.

Restoration decrypts into a new private generation, generates another master key,
migrates the database, and re-encrypts secrets. After authenticating through EOF,
verifying checksums, SQLite integrity, and permissions, it changes `data/current`
by atomically renaming a temporary link on the same filesystem. The previous
generation is retained until readiness and a route test pass.

Before activating the generation, every row in `admin_sessions` is deleted. No
cookie captured before the backup can authenticate the restored installation.
Agent-token verifiers, expiration, and revocation are retained because they are
explicit operational credentials independent of the master key; the operator may
revoke them before or after restore.

An incorrect password, disallowed header, truncated stream, insufficient space,
failed migration, or invalid secret leaves the active generation untouched.

## Compatibility

`manifest.json` contains `profile: modelcairn-backup`, `profileVersion: 1`, the
application and schema versions, UTC date, sizes, and exact entry list. Unknown
profiles are rejected. An incompatible evolution uses another `profileVersion`;
it does not reinterpret MCB1.

## Source

- [age: tool, format, and Go library](https://github.com/FiloSottile/age)
- [age v1 specification](https://github.com/C2SP/C2SP/blob/main/age.md)
