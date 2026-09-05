# Declarative `apply` semantics v1alpha1

[Español](semantica-apply-v1alpha1.es.md)

The neighboring JSON Schema defines structure and types. This document defines effects.

## Identity and references

- Declarative identity is `(kind, metadata.name)`; it does not change when `displayName` changes.
- SQLite assigns an immutable UUID. `uid` and `resourceVersion` appear in exports,
  but are ignored on creation and, if supplied, must match on update.
- References use the resource name of the expected type. Ambiguous references and
  Strategy→Strategy cycles are not allowed in v1alpha1.
- The credential's `ProviderAccount` and the model's `ProviderConnection` for a
  Destination must belong to the same Provider. Validate, apply, API, and publish
  reject a mismatch with `provider_mismatch`; this prevents sending an API key to
  another provider's endpoint.
- In the API, the body `kind` must correspond to the plural `{kind}` in the route;
  a mismatch returns `400 kind_mismatch`.

## Operations

- `validate`: structure, references, capabilities, URLs, and rules; zero writes.
- `plan`: runs validate and returns create/update/noop/delete, always redacted.
- `apply`: single transaction. Creates or updates only declared resources.
- A resource absent from the file means **do not manage it**, not delete it.
- `state: absent` requests deletion and requires `--allow-delete`; if references
  exist, it fails or requires removing them first in the same transaction.
- No failure causes partial application.
- Declared arrays replace the entire array; they are not merged by position.
- Omitted fields receive defaults on creation and preserve their value on update,
  unless the schema explicitly allows `null` to clear them.

## Secrets and tokens

`secretRef` references a secret already created through the console or with
`modelcairn secret set <name>`, which reads the value from stdin/TTY. YAML never
contains secret values. Declaring `Credential state: absent` deletes ciphertext
only when no active destinations exist. New tokens are created by a separate
operation that returns the value once; YAML manages their metadata and can revoke
them, but does not recreate the same value.

## Publishing

Applying a Strategy updates its draft. A `publish` operation validates the graph,
creates an immutable version, and atomically changes the active reference. Route
references the logical Strategy and uses only its published version.

## Concurrency

The API uses `ETag`/`If-Match` over `resourceVersion`. CLI plan stores observed
versions in an opaque, short-lived `planToken` bound to the canonical desired
configuration and its expiry. The plan response returns this token and apply must
send it in `X-ModelCairn-Plan-Token`. Apply fails with `409 version_conflict` if an
observed version changed, the submitted configuration differs from the planned
configuration, or the token expired. The user must run plan again; ModelCairn does
not silently overwrite web changes. Tokens never contain secret values.
