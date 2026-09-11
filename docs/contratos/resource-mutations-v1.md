# Individual resource mutations v1

[Español](resource-mutations-v1.es.md)

Status: implementable Milestone 3 design; mutation endpoints remain outstanding.
Extends the [shared semantics](config/semantica-apply-v1alpha1.md).

## Service and boundaries

Implement `config.Manager.MutateResourceSession` with a typed create/update/delete
operation, route kind/name, decoded resource where applicable, expected version,
session and CSRF. HTTP must not supply Actor. Return the committed resource for
create/update; delete has no body. Never reread after commit to build its DTO/ETag.

Handlers check Origin/transport, content type, size, method, route/body identity
and If-Match syntax. The service validates typed arguments again. Reuse the strict
parser via a single-resource Configuration envelope, preserving Presence and
duplicate-key detection. Do not first decode into a map that discards duplicates.
The existing input limit is 8 MiB. POST/PUT accept present or omitted state; reject
absent. DELETE constructs absent internally and accepts no additional body changes.
PUT preserves omitted fields under the shared contract; explicit arrays replace
arrays. Do not introduce different default semantics under the name replace.

## One operation, one transaction

Add an internal SecretStore executor with ExecutePlan's lock order: store mutex,
BeginTx, callback, commit, unlock. Individual CRUD does not create artificial plans
or consume nonces. Only internal callbacks are allowed. Reject unavailable stores,
roll back on error and mark unavailable on uncertain commit outcomes.

Inside the callback:

1. Call AuthorizeAdminMutationTx after waiting for locks, obtaining the live Actor.
2. Read the current kind/name through tx. Creation requires absence (409 otherwise).
   Update/delete require existence (404) and matching version (412). Also check
   body uid/resourceVersion when provided.
3. Use prepareTx with a single-resource document, allowDelete only for deletion.
   Read catalog and secrets through this tx. Prepare/Validate check the entire
   effective graph, including dependents omitted from the request.
4. Execute ApplyConfigTx: it already audits typed mutations and advances
   config_revision on graph changes. Never call Repository.Put/Delete or any
   method opening another transaction inside the callback.
5. A noop update preserves version/timestamp but inserts a typed noop operation
   audit within tx. Do not use a fake mutation to force a revision increment.
6. Read the resulting resource inside tx and capture its DTO. Return it only after
   successful commit. Any failure rolls back last_seen and audit as well.

Two real changes with the same ETag: one succeeds, the next receives 412.
Reset/logout committed first invalidates authorization. Deleting and recreating a
name creates another identity: numeric If-Match checks only the current identity's
version. Do not claim protection across recreation; an optional body uid adds that
check. Consider identity-bearing ETags in a future contract revision before
promising that guarantee for DELETE.

## HTTP mapping

POST returns 201 and PUT 200 with committed-version ETag; DELETE returns 204 with
no-store. Missing PUT/DELETE If-Match: 428. Weak tags, wildcard, lists, signs, leading
zeros, duplicate headers and overflow: 400. Accept only `"[1-9][0-9]*"`.
Route/body kind or name mismatch: 400. Dependency and provider mismatch: 409.
Never expose SQL errors or rejected values. Apply the shared redactor to DTO
strings before serialization, not to serialized JSON text.

AgentToken CRUD edits metadata only: no bearer issuance, token_hash replacement or
revoked_at revival. Preserve the existing irreversible upsert; issue/revoke use
dedicated services. Never implement publish/test as fabricated success.

## Grouped acceptance

Cover duplicate creation, missing update/delete targets, malformed/stale ETags and
two concurrent writes. Reject deletion of referenced resources and provider changes
that invalidate a Destination absent from the request. Verify stable audited noops,
audit-failure rollback, session revocation and secret-free DTOs. Group QA with the
implementation block; this design does not claim delivered endpoints.
