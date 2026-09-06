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

The SQLite driver is the only direct third-party Go module. Its transitive modules
are locked in `go.sum`; CI runs `go mod tidy` and rejects an uncommitted module-file
change. The Milestone 1 spike compiles and tests the driver and records a
conservative test-artifact size and peak RSS, while the executable will not link it
until persistence becomes part of application startup. Product-linked cost will be
measured again in that milestone.

Cryptographic libraries will be added only with the milestone that exercises their
approved contracts.
