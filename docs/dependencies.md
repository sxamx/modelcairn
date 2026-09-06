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
| `golang.org/x/sys` v0.47.0 | build and runtime from Milestone 2 | Native non-blocking file locks on Windows; already required transitively, promoted to a direct dependency, and BSD-3-Clause licensed |

Transitive modules are locked in `go.sum`; CI runs `go mod tidy` and rejects an
uncommitted module-file change. Milestone 2 links persistence into application
startup. `x/sys` can be removed if the Go standard library later exposes the same
portable non-blocking locking semantics. Product-linked cost is measured again at
the milestone's resource gate.

Cryptographic libraries will be added only with the milestone that exercises their
approved contracts.

For every dependency-changing milestone, CI verifies a clean `go mod tidy` and
runs a pinned `govulncheck ./...`; each direct module's license and purpose are
recorded here before acceptance.
