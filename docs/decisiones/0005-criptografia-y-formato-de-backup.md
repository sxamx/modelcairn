# ADR-0005: Cryptography and backup format

[Español](0005-criptografia-y-formato-de-backup.es.md)

- Status: technically accepted; parameters subject to benchmarking
- Date: 2026-09-05

## Decision

- Administrative passwords: Argon2id with a unique 16-byte salt. Initial minimum
  of `m=19456 KiB`, `t=2`, `p=1`; the installer may increase the cost if the
  VM meets the 100–500 ms target without exceeding the memory budget.
- API-key encryption: XChaCha20-Poly1305 with a 256-bit key, random 24-byte nonce,
  and associated data including version, installation ID, and credential ID and
  version.
- Master key: 32 bytes from the system CSPRNG, in a file separate from SQLite,
  mode `0600`, owned by the service. Versioned, transactional rotation.
- Agent tokens: 32 random bytes encoded for transport; the value is shown once and
  SHA-256 of the complete token is stored. Their 256 bits of random entropy avoid
  dependence on the master key and allow their verifiers to survive rotation and
  restoration. User-chosen tokens are not accepted.
- Full backup: the MCB1 profile over the streaming age v1 format with an scrypt
  password recipient. The internal manifest and checksums supplement, but do not
  replace, authentication provided by the age format.
- The backup password is requested interactively or by file descriptor, never as
  a command-line argument.

## Restoration

1. Read and validate limits, version, and structure without extracting files.
2. Derive the backup key and authenticate all content before mutating state.
3. Restore into a temporary directory on the same filesystem with private
   permissions.
4. Generate a new master key for the destination installation.
5. Decrypt each secret from the container and re-encrypt it with the new key.
6. Run migrations and integrity checks on the copy.
7. Stop writes, create a recovery backup, and perform an atomic replacement.
8. Start, check readiness, and test a route selected by the operator.
9. On any failure, retain the previous installation and securely remove temporary
   files when the system permits.

The original master key is not restored. This avoids reusing the same key
indefinitely across machines and permits recovery on another VM using only the
backup and its password.

## Logical MCB1 format

The age profile, limits, entries, and generational replacement are defined in the
[MCB1 specification](../contratos/backup-mcb1.md). It does not include system
logs, TLS keys, or SSH credentials. Size and entry-count limits are validated
before memory or disk is allocated.

## Consequences

- An attacker with root access while the system is running can access the master
  key; this ADR does not claim otherwise.
- Losing the password to the only full backup makes its secrets unrecoverable.
- Argon2id parameters are stored alongside the hash/ciphertext to allow gradual
  strengthening without invalidating older data.
- Rotating the master key re-encrypts API keys but does not change sessions or
  agent tokens during normal operation. Restoration removes all administrative
  sessions and retains agent-token verifiers and revocation metadata.
- Libraries will be pinned by version and undergo dependency analysis.

## Sources

- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [Official Go XChaCha20-Poly1305 package](https://pkg.go.dev/golang.org/x/crypto/chacha20poly1305)
- [age library and format](https://github.com/FiloSottile/age)
