# Dependency policy and inventory

[Español](dependencies.es.md)

ModelCairn minimizes runtime dependencies to protect the 1 GB VM target and the
cross-compilation path. Every direct dependency requires a concrete capability,
maintenance and license review, resource-impact evidence, and a removal plan.

## Current inventory

| Dependency | Scope | Purpose |
|---|---|---|
| Go standard library | build and runtime | CLI, HTTP lifecycle, logging, synchronization, and tests |

There are currently no third-party Go modules. SQLite and cryptographic libraries
will be added only with the milestone that exercises their approved contracts.
