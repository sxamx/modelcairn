# Milestone 2 — Secret cipher core checkpoint

[Español](hito-02-item-03-crypto-core.es.md)

Status: implementation checkpoint; delivery item 3 is **not accepted**.

Implemented: XChaCha20-Poly1305 sealing/opening with authenticated installation,
secret identity and versions; UTF-8 byte limits; purpose-separated local
fingerprints. These are internal helpers, not a usable secret-store API yet.

Validation on the Windows AMD64 development machine:

- `go test ./...`, `go vet ./...`, and `node scripts/validate-docs.cjs`: passed.
- Independent static QA found missing fingerprint/context and malformed-input
  coverage. Added a fingerprint vector calculated independently with .NET HMAC,
  an associated-data byte vector, distinct-secret comparison, and rejection tests
  for tampered nonce, invalid keys, oversized/truncated ciphertext and authenticated
  invalid UTF-8. QA did not evaluate persistence or rotation.
- `go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 -show verbose ./...`:
  no reachable vulnerability or imported-package finding. Module-level advisory
  [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) concerns the unused
  `golang.org/x/crypto/openpgp` package; ModelCairn imports `chacha20poly1305`, not
  OpenPGP. The scan is now configured in CI; CI execution is not claimed here.
- A 100-iteration `BenchmarkSecretSeal` smoke run reported 274 B/op for 32-byte
  secrets and 18,656 B/op for 16,384-byte secrets, both six allocations per
  operation. This is not VM RSS, a stable throughput result, or a milestone gate.

The next checkpoint adds a private versioned keyring, authenticated empty-store
bootstrap, verification of every persisted secret before secret-store readiness,
metadata-only create/update/list/delete operations, scoped plaintext use, and an
installation-wide exact-value redactor connected to service logs. Local tests
exercise missing, replaced, short and oversized keys; immutable publication;
orphan-version bootstrap; ciphertext tampering; reference-protected deletion; and
redactor registration across update, deletion and restart. Linux AMD64 and ARM64
cross-builds pass locally; Linux durability still requires runtime execution.
Independent checkpoint QA initially found broad/late structured-log leakage and
insufficient key ownership/ACL enforcement. The redactor now sanitizes messages,
attribute and group names, nested groups and arbitrary structured values at emit
time. Unix validates the effective UID; Windows applies and validates a protected
DACL and rejects unknown allow-capable ACE types. Focused re-review closed both
findings with no residual in this scope.

Still pending: transactional rotation and garbage collection, kill-point tests,
independent QA of the complete store, Linux runtime execution and the
representative VM resource gate. Nothing here establishes those properties.
