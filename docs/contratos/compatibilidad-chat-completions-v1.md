# Chat Completions v1 compatibility matrix

[Español](compatibilidad-chat-completions-v1.es.md)

- Status: Phase 1 contract
- Endpoint: `POST /v1/chat/completions`
- Schema contract: [Data API v1 OpenAPI](api/data-v1.openapi.yaml)

The executable parser caps the body at 1 MiB and JSON depth at 64 levels, rejects
duplicate keys at every level, and requires valid UTF-8. Unknown fields are errors
in this first delivery; no passthrough mode exists yet.
Upstream responses are subject to the same UTF-8, depth, and duplicate-key rules
before they can be committed.

## Request

| Field | Phase 1 | Rule |
|---|---|---|
| `model` | required | logical route alias, not a physical ID |
| `messages` | required | system, user, assistant, and tool roles; developer requires the `developer-role` capability |
| text content | supported | strings and text parts |
| images, audio, or files | initially rejected | capability error before calling the provider |
| `stream` | supported | `false` by default; `true` uses SSE |
| function `tools` | conditionally supported | only destinations with the `tools` capability |
| `tool_choice` | conditionally supported | validated against declared tools |
| `parallel_tool_calls` | conditionally supported | requires a declared capability |
| `temperature`, `top_p` | validated passthrough | the adapter declares ranges/support |
| `max_tokens`, `max_completion_tokens` | normalized | a conflict between them produces `400` |
| `stop`, `n`, penalties, `seed`, `logprobs` | future | rejected by the current parser |
| `response_format` | future | rejected by the current parser |
| unknown fields | rejected by default | future passthrough mode, explicit per connection |

A route is eligible only if it preserves every capability required by the request.
The interface will show why a destination was excluded.

`stream_options` requires `stream: true`; `tool_choice` and
`parallel_tool_calls` require a `tools` list. Supplying both token-limit variants
in one request is an error. An assistant message may contain content, tool calls,
or both, but it cannot omit both.

## Non-streaming response

ModelCairn preserves `id`, `object`, `created`, the requested alias in `model`,
`choices`, messages, tool calls, `finish_reason`, and `usage` when supplied by the
provider. ModelCairn IDs are distinguished from upstream request IDs. Permitted
additional fields are documented and do not alter the shape required by clients.

## SSE streaming

- `Content-Type: text/event-stream`.
- Every emitted frame follows `data: <json>\n\n` and ends with `data: [DONE]\n\n`
  on normal completion.
- The first complete valid JSON event is the commitment point. Fallback may occur
  before it; the destination never changes after it.
- Each event is capped at 1 MiB. Comments and non-`data` SSE fields are not
  forwarded, and an empty stream, invalid JSON, or termination without `[DONE]`
  is not a success.
- The `model` field in every chunk is normalized to the requested logical alias.
- Indices, `delta.role`, `delta.content`, tool-call deltas, and `finish_reason` are preserved.
- An error after commitment closes the stream; it does not insert an incompatible
  JSON response or continue from another destination.
- Usage is delivered in the stream only when the connection/adapter supports it
  and the request asks for it compatibly.
- The ModelCairn request ID is returned in `X-ModelCairn-Request-ID` before the SSE
  body starts.

ModelCairn persists only normalized `prompt_tokens` and `completion_tokens`
counters when upstream supplies both as non-negative values. For streaming it also
records TTFT from request reception to the first valid event. It does not persist
the event, prompt, or response that produced those metrics.

## ModelCairn error

Before committing the stream, the body uses:

```json
{
  "error": {
    "message": "Safe description",
    "type": "modelcairn_error",
    "code": "capability_not_supported",
    "param": "tools",
    "request_id": "req_..."
  }
}
```

Internal codes do not include private URLs, full provider responses, or secrets.
Official OpenAI documentation is a shape reference, not a claim of equivalence
for features outside this matrix.
