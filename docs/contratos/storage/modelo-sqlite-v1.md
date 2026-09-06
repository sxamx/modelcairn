# SQLite model v1

[Español](modelo-sqlite-v1.es.md)

The [schema-v1.sql](schema-v1.sql) file is the initial physical contract. It will
not be copied directly as a migration: Milestone 2 will convert it into versioned
migrations and tests.

## Transactional rules

- Creating or updating a resource and its typed table occurs in one transaction.
- Publishing a strategy inserts an immutable version and changes the route in one transaction.
- Recording a request and its attempts may use separate short transactions; no
  transaction remains open during a network call or stream.
- Affinity lives in `credentials.egress_id`; the router never modifies it.
- The SQLite writer will be coordinated within the process and WAL will be tested.
  There will be no direct access from other processes or a network filesystem.
- Retention deletes in small batches and never disables disk limits.
- Configuration changes and their success audit event commit together. After a
  mutation rolls back, its failure event is attempted in a separate short
  transaction and cannot claim that state was applied. If SQLite itself cannot
  record that event, a structured secret-free operational error is emitted.
- A plan-token nonce is consumed in the same transaction as apply. Reuse returns
  `plan_already_used`; expired nonce rows are maintenance data and may be pruned.

## Deliberately absent content

There are no columns for prompts, messages, tool outputs, or responses. The
`content_stored = 0` contract makes the Phase 1 policy visible. Persisted errors
are normalized classes; upstream bodies and sensitive headers are not stored.

## Evolution

Resources retain `spec_json` for round-trip and typed tables for invariants and
critical queries. A migration must keep both representations consistent. If this
duplication does not pass tests, it will be simplified through an ADR before a
stable version is published.
