# ADR-0003: Technical foundation and initial deployment

[Español](0003-base-tecnica-y-despliegue.es.md)

- Status: accepted
- Date: 2026-09-05
- Owners: primary maintainer and technical design

## Context

ModelCairn must run on a Linux VM with 1 GB of RAM, handle streaming HTTP
traffic, install with little effort, and be distributed for AMD64 and ARM64. The
previous prototype does not determine the final technology.

## Decision

- The backend and system tools will be implemented in **Go**.
- The initial architecture will be a **modular monolith** distributed as an
  executable.
- The compiled web console may be included in the server distribution.
- **systemd** will be the primary execution method on Linux.
- The installer will ask whether startup after machine reboot should be enabled.
  Enabling it will be the recommended and default option.
- Docker will be offered as an alternative after the native path is validated.
- Linux artifacts will be published for AMD64 and ARM64.
- The primary 1 GB measurement will first run on the actual reference VM, followed
  by representative validation on the other architecture.

## Rationale

Go produces executables and supports Linux AMD64 and ARM64 targets. Its concurrency
model and HTTP library fit routing and streaming, while a single process simplifies
resource use, installation, and diagnostics. systemd provides supervision and
startup during boot without requiring a container runtime.

The choice must pass the Phase 1 memory, concurrency, and streaming benchmark. An
insufficient result requires this decision to be reviewed or replaced through
another ADR.

## Consequences

- The previous prototype serves as a functional reference, not a required base.
- Boundaries between modules must be verified through packages and tests.
- The project will maintain builds and tests for both architectures.
- Docker installation may have documented operational differences from systemd,
  but no intentional functional differences.

## Technical sources

- [Official Go compilation documentation](https://go.dev/doc/tutorial/compile-install)
- [Go operating-system and architecture targets](https://go.dev/doc/install/source#environment)
