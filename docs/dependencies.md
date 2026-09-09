# Dependency policy and inventory

[Español](dependencies.es.md)

ModelCairn minimizes runtime dependencies to protect the 1 GB VM target and the
cross-compilation path. Every direct dependency requires a concrete capability,
maintenance and license review, resource-impact evidence, and a removal plan.

## Current inventory

| Dependency | Scope | Purpose |
|---|---|---|
| Go standard library | build and runtime | CLI, HTTP lifecycle, logging, synchronization, and tests |
| `modernc.org/sqlite` v1.58.0 | build and runtime from Milestone 2 | CGO-free SQLite driver; enables Linux AMD64/ARM64 cross-builds and is BSD-3-Clause licensed |
| `golang.org/x/sys` v0.48.0 | build and runtime from Milestone 2 | Native non-blocking file locks on Windows; already required transitively, promoted to a direct dependency, and BSD-3-Clause licensed |
| `golang.org/x/crypto` v0.56.0 | secret encryption from Milestone 2 | Official Go XChaCha20-Poly1305 implementation; BSD-3-Clause; avoids implementing the cipher ourselves |
| `golang.org/x/term` v0.46.0 | secret CLI from Milestone 2 | Portable no-echo terminal input for secrets; BSD-3-Clause; prevents API keys from being displayed during interactive entry |
| `gopkg.in/yaml.v3` v3.0.1 | configuration parsing from Milestone 2 | YAML parser with an inspectable syntax tree; allows aliases, tags, duplicate keys, and non-string keys to be rejected before canonicalization; MIT/Apache-2.0 |
| `golang.org/x/vuln` v1.7.0 | CI and development only | Pinned `govulncheck` for reachable vulnerability analysis; BSD-3-Clause; not linked into the server |

Transitive modules are locked in `go.sum`; CI runs `go mod tidy` and rejects an
uncommitted module-file change. Milestone 2 links persistence into application
startup. `x/sys` can be removed if the Go standard library later exposes the same
portable non-blocking locking semantics. Product-linked cost is measured again at
the milestone's resource gate.

The encrypted store and its rotation are accepted with representative measurement.
`x/crypto` can be removed if a standard-library implementation provides the same
XChaCha20-Poly1305 format without invalidating persisted secrets. `yaml.v3` remains
encapsulated in `internal/config` and can be replaced without changing the internal
model; the hostile-input and round-trip suite defines the behavior to preserve.
`x/term` remains confined to the CLI boundary and can be removed if the standard
library gains portable no-echo password input.

For every dependency-changing milestone, CI verifies a clean `go mod tidy` and
runs a pinned `govulncheck ./...`; each direct module's license and purpose are
recorded here before acceptance.
