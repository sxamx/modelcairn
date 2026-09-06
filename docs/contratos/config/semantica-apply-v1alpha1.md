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

## Parsing and output safety

- v1alpha1 accepts exactly one YAML or JSON document of at most 8 MiB, with at
  most 64 nested containers. YAML aliases and custom tags are rejected before
  schema validation; decoded node and scalar counts remain bounded by the input
  and schema limits.
- Duplicate mapping keys and non-string YAML keys are rejected before defaults,
  canonicalization, digest calculation, or schema validation.
- `ProviderConnection.baseUrl` rejects user information, fragments, and query
  strings. Provider model IDs and descriptive fields remain data, never log or
  audit formatting instructions.
- A shared redactor replaces exact registered secret values in plans, exports,
  errors, logs, and test diagnostics. Errors never echo rejected scalar values.
- Audit `details_json` is built from an action-specific allowlist of typed IDs,
  versions, result codes, and counts. It never accepts arbitrary maps, URLs,
  descriptions, request headers, or upstream bodies.

## Secrets and tokens

`secretRef` references a secret already created through the console or with
`modelcairn secret set <name>`, which reads the value from stdin/TTY. YAML never
contains secret values. Deleting a Credential never deletes its Secret. Secret
deletion is a separate operation and fails while any Credential references it.
New tokens are created by a separate operation that returns the value once; YAML
manages their metadata and can revoke them, but does not recreate the same value.
Secret values contain at least 8 and at most 16,384 UTF-8 bytes.
The displayed secret fingerprint is `mc_fp_` plus base64url of the first 12 bytes
of HMAC-SHA-256 over the secret using a purpose-separated key derived from its
master-key version. It is an installation-local comparison aid, changes on master
key rotation, and is never used for authentication.

## Publishing

Applying a Strategy updates its draft. A `publish` operation validates the graph,
creates an immutable version, and atomically changes the active reference. Route
references the logical Strategy and uses only its published version.

Milestone 2 persists and validates Provider, Destination, Strategy, Route, and
other routing-resource drafts so configuration round trips are complete. It does
not publish a strategy, activate a route, contact a provider, or execute fallback;
those behaviors begin in Milestone 4.

## Concurrency

The API uses `ETag`/`If-Match` over `resourceVersion`. A plan token is
`base64url(payload).base64url(mac)`, where `mac` is HMAC-SHA-256 over the encoded
payload. Its key is derived from the active master key with HKDF-SHA-256 and the
purpose `modelcairn/plan-token/v1`. The canonical payload contains token version
and purpose, installation ID, active key version, configuration revision, desired
configuration SHA-256, ordered observed resource identities and versions (including
absence), random 256-bit nonce, issue time, and expiry. v1 tokens expire after ten
minutes and are rejected if issued more than 30 seconds in the future. Rotation
invalidates outstanding tokens.

Apply verifies the MAC before parsing trusted claims, checks every binding and the
current clock, and consumes SHA-256 of the nonce in the same transaction as the
configuration change. A token is single-use, including for a no-op. Apply fails
with `version_conflict`, `plan_expired`, `plan_already_used`, or `invalid_plan`
without changing configuration. ModelCairn never silently overwrites web changes,
and tokens never contain secret values.

In Milestone 2, `config plan` writes a redacted plan file when `--out` is supplied.
`config apply --plan <plan-file> <config-file>` verifies its authenticated token,
expiry, desired-configuration digest, and observed versions while holding the
installation lock. Interactive `config apply <config-file>` creates a fresh plan,
shows it, and requires confirmation in the same locked process; automation must
provide a plan file. The administrative HTTP representation introduced in
Milestone 3 carries the same token in `X-ModelCairn-Plan-Token`.
