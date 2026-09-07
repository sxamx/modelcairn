# Milestone 2 — Secret cipher core checkpoint

[Español](hito-02-item-03-crypto-core.es.md)

Status: partial implementation; delivery item 3 is **not accepted**.

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

Still pending: durable private keyring, installation bootstrap and startup key
verification, metadata-only repository, transactional rotation and garbage
collection, common output redaction, kill-point tests, Linux execution and the
representative VM resource gate. Nothing here establishes those properties.
