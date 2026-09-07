# Resource repository contract v1

[Español](resource-repositories-v1.es.md)

## Boundary

The repository is the only Milestone 2 code allowed to mutate resource envelopes,
their typed projections, and mutation audit records. It accepts canonical JSON
objects from the configuration validation layer. It still enforces identity,
optimistic versions, database references, provider affinity, and deletion safety;
full JSON Schema validation remains the configuration codec's responsibility.

Declarative identity is `(kind, name)`. A create request uses expected version
zero and receives a random immutable UUID plus resource version 1. An update must
provide the current positive resource version and increments it exactly once.
Reads return the stored canonical spec and lists are ordered by kind and name.

## Transactions

- Envelope and typed projection create or update in one SQLite transaction.
- A stale version, missing reference, provider mismatch, uniqueness conflict, or
  typed-table trigger rolls the entire mutation back.
- A success audit is inserted in that same transaction. Failure to insert it
  prevents the state change from committing.
- After rollback, a failure audit is attempted in a separate short transaction.
  If SQLite cannot write it, the caller receives both the original mutation error
  and a structured audit-unavailable error.

`Provider` and draft `Strategy` currently require no separate physical projection;
their canonical `spec_json` is authoritative. Every other configuration resource
uses its corresponding typed table from `schema-v1.sql`. Strategy destination
references and AgentToken route references are resolved and protected explicitly
because the physical schema stores those arrays as JSON.

AgentToken revocation is irreversible for one resource identity. Changing
`enabled` from false back to true is rejected as `invalid_resource`; issuing a new
usable token requires a new identity through the dedicated token operation. The
envelope can therefore never claim enabled while its typed row remains revoked.
Repeated disabled updates preserve the original revocation timestamp and issued
identity metadata.

## Stable result codes

- `already_exists`
- `invalid_actor`
- `invalid_resource`
- `not_found`
- `reference_not_found`
- `resource_in_use`
- `version_conflict`
- `delete_not_allowed`
- `provider_mismatch`

No code includes the rejected spec or scalar value.

## Deletion and secrets

Deletion requires explicit authorization and the exact resource version. Foreign
keys and logical-array checks reject unsafe order with `resource_in_use`. Deleting
a Credential never deletes its referenced Secret; secret deletion is a separate
secret-store operation.

## Typed audit details

Repository callers cannot supply arbitrary audit maps. The repository constructs
details internally from this fixed allowlist:

| Result | Allowed detail fields |
|---|---|
| success | `version` |
| failure | `code` |

Actions are restricted to `resource.create`, `resource.update`, and
`resource.delete`; actor type is `admin`, `cli`, or `system`.
