# Milestone 4 — Vertical router

[Español](hito-04-plan-tecnico.es.md)

Status: proposed implementation plan; implementation not started.
Prerequisite: Milestone 3 merged and accepted.

## Outcome

Deliver one real, deterministic path from `POST /v1/chat/completions` to an
OpenAI-compatible upstream and back. The path authenticates an AgentToken,
resolves its route alias, selects only compatible destinations from one immutable
strategy snapshot, executes bounded fallback, preserves tool calls and SSE, and
records content-free operational events.

This milestone proves the router core. It does not deliver the visual editor,
adaptive estimator, remote ModelCairn relay protocol, or every provider dialect.
Those features must extend these contracts rather than bypass them.

## Fixed boundaries

- Data API: `POST /v1/chat/completions`; administration remains under
  `/api/v1/admin/*`.
- Authentication uses `Authorization: Bearer` and the existing AgentToken verifier.
- `model` is a logical Route alias, never an upstream model identifier.
- Phase 1 accepts strict known fields only and never silently drops unsupported
  request parameters.
- Prompt and response content is forwarded but not persisted or logged.
- A credential remains pinned to its configured Egress. Fallback never changes
  that affinity implicitly.
- One request has one immutable configuration/strategy snapshot even if an
  administrator publishes changes concurrently.
- Budgets are finite: total timeout, per-attempt timeout, maximum attempts and
  body size are checked before work and during execution.
- No fallback occurs after the response commitment point.

## First supported adapter and egress

The first adapter is `openai-chat-completions`. It joins the canonical connection
base URL with `/chat/completions`, injects the referenced API-key secret into the
upstream Authorization header, maps the logical alias to the selected physical
model and validates the response media type and shape.

The first executable egress is `direct`. HTTP CONNECT/SOCKS and the authenticated
remote ModelCairn relay require separate transport contracts before activation.
The resource graph already preserves stable Credential-to-Egress affinity so those
transports can be added without changing router selection semantics.

## Request pipeline

1. Apply bounded HTTP parsing and create a ModelCairn request ID.
2. Authenticate the AgentToken and authorize the requested Route.
3. Parse and normalize the strict Chat Completions request.
4. Derive required capabilities, such as tools, parallel tools or response format.
5. Load the Route and one published immutable Strategy snapshot.
6. Filter disabled, incompatible or cooling destinations before selection.
7. Execute destinations sequentially within attempt and time budgets.
8. Classify each result before deciding return, cooldown or fallback.
9. Commit a valid non-stream response or validated SSE upstream response.
10. Persist the Request, Attempts and normalized observations without content.

## Error and commitment invariants

The classification table in the Phase 1 technical contract is normative. In
particular, authentication/configuration/capability errors never call upstream;
confirmed `429`, connection failures and eligible `5xx` may fallback; request
errors and indeterminate post-send timeouts do not fallback by default.

For streaming, ModelCairn waits for a successful upstream status and valid SSE
media type before sending successful headers. Before that point fallback is
allowed. After headers or the first downstream event, whichever comes first, a
failure terminates the stream and records `partial`; it never splices another
provider response into the same stream.

## Delivery blocks

1. **Router contracts and simulator:** exact data OpenAPI, strict request parser,
   deterministic upstream fixture and failure matrix.
2. **Snapshot and eligibility:** SQL snapshot loader, route authorization,
   capability filtering, stable sequential order and exclusion reasons.
3. **Non-stream adapter:** secret-safe transport, response normalization,
   cancellation and connection validation.
4. **Budgets and fallback:** classifier, attempt state machine, cooldown writes,
   Retry-After parsing and finite deadlines.
5. **Streaming:** SSE validation/relay, commitment boundary, tool-call deltas,
   `[DONE]`, disconnect propagation and partial outcomes.
6. **Operational persistence:** atomic Request/Attempt lifecycle and normalized
   rate-limit observations with no prompt/response bodies.
7. **Acceptance:** grouped QA, race suite, simulated fault matrix and VM resource
   benchmark at 1, 2, 5, 10 and 20 concurrent streams.

Each block receives focused tests. Independent QA is grouped at the state-machine
boundary and at final acceptance rather than repeated for every small edit.

## Acceptance criteria

- A configured AgentToken can complete normal and streaming calls through a local
  deterministic OpenAI-compatible simulator.
- Revoked, malformed or route-forbidden tokens return the contracted status
  without contacting upstream.
- Tool calls round-trip only through destinations declaring every required
  capability.
- The complete failure matrix proves exactly when fallback occurs and that budgets
  cannot loop indefinitely.
- Client cancellation cancels upstream promptly.
- No test, response, log, export or persisted operational row exposes API keys,
  session/bearer values, prompts or model responses.
- The resource benchmark records RSS, swap, latency and successful concurrency;
  unsupported capacity is reported rather than advertised.
- Documentation, Go tests, race tests, vulnerability scan, cross-builds and the
  vertical integration gate pass before the milestone is accepted.
