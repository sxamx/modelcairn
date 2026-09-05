# ModelCairn Baseline Requirements

- Status: prioritized draft
- Convention: `MUST` mandatory, `SHOULD` recommended, `MAY` optional

## Priority P0 - first useful and safe route

- **RF-001 MUST:** expose `POST /v1/chat/completions` with non-streaming and streaming requests, within an explicit compatibility matrix.
- **RF-002 MUST:** authenticate each client through a revocable agent or integration identity.
- **RF-003 MUST:** allow creating provider, provider connection, model, credential, egress, destination, logical route, and strategy without modifying code.
- **RF-004 MUST:** store API keys only on the main node, encrypted at rest and redacted in all outputs.
- **RF-005 MUST:** bind each credential to an egress and not change that affinity automatically.
- **RF-006 MUST:** select a compatible destination and execute fallback only when the error policy allows it.
- **RF-007 MUST:** apply a total attempt and time budget per request.
- **RF-008 MUST:** preserve streaming semantics; do not silently restart on another destination after content has been delivered to the client.
- **RF-009 MUST:** record decisions, timings, and errors without storing prompts or responses by default.
- **RF-010 MUST:** allow configuring the first route from an understandable web console and from a documented declarative interface.
- **RF-011 MUST:** provide differentiated health and readiness and basic diagnosis.
- **RF-012 MUST:** perform verifiable backup, restore, and migration.
- **RF-013 MUST:** validate provider endpoints and block unauthorized network access, allowing private destinations only through explicit policy.

## Priority P1 - adaptive and multinode operation

- **RF-101 MUST:** register relays and check identity, health, version, and egress.
- **RF-102 MUST:** transport connections through a relay without persisting secrets or content on it.
- **RF-103 MUST:** show topology, load, providers, destinations, and number of associated credentials without revealing secrets.
- **RF-104 MUST:** observe `429`, timeouts, latency, tokens, and recovery by destination.
- **RF-105 MUST:** estimate capacity and recovery with a confidence level and an accessible explanation.
- **RF-106 MUST:** prioritize official limits and operator rules over statistical inferences.
- **RF-107 SHOULD:** distribute concurrent requests among eligible destinations without deliberately overloading a single one.
- **RF-108 MUST:** allow the operator to reassign an egress with warning, validation, and audit.

## Priority P2 - experience and extensibility

- **RF-201 MUST:** provide an installable PWA adaptable to mobile.
- **RF-202 MUST:** offer historical metrics, detailed events, filters, and configurable retention, including indefinite retention, with protection and warnings about disk growth.
- **RF-203 SHOULD:** provide a visual strategy editor with validation, simulation, publication, and rollback.
- **RF-204 MUST:** allow additional adapters without rewriting the router.
- **RF-205 SHOULD:** incorporate an Anthropic-compatible endpoint through a translation contract and verified capabilities.
- **RF-206 SHOULD:** allow local content storage only through a future explicit policy separate from metrics.

## Non-functional requirements to finalize with measurements

- **RNF-001:** the reference installation must run on a 1 GB Linux VM without sustained swapping or out-of-memory termination.
- **RNF-002:** the relay must use a small and measurable fraction of the main node's resources.
- **RNF-003:** no connectivity loss can cause unlimited retries.
- **RNF-004:** critical administrative operations are auditable.
- **RNF-005:** installation, upgrade, and restore must be reproducible.
- **RNF-006:** the API must not promise compatibility for untested capabilities.
- **RNF-007:** the basic console must be usable without understanding the internal architecture, and advanced controls must remain accessible.
- **RNF-008:** there is no built-in external telemetry.

The numerical thresholds for memory, added latency, concurrency, disk, and recovery time will be set after building a representative benchmark. Choosing them without defined load and hardware would produce fictitious precision.

## Out of initial scope

- Hosting or running AI models.
- Becoming a general agent platform.
- Guaranteeing quality equivalence between different models.
- Circumventing provider limits, suspensions, or terms.
- Automatic high availability of the main node.
- Synchronizing API keys between independent installations.
- Storing prompts or responses by default.
