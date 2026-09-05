# Chat Completions v1 compatibility matrix

[Español](compatibilidad-chat-completions-v1.es.md)

- Status: Phase 1 contract
- Endpoint: `POST /v1/chat/completions`

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
| `stop`, `n`, penalties, `seed`, `logprobs` | conditional | never silently ignored |
| `response_format` | conditional | requires the corresponding capability |
| unknown fields | rejected by default | future passthrough mode, explicit per connection |

A route is eligible only if it preserves every capability required by the request.
The interface will show why a destination was excluded.

## Non-streaming response

ModelCairn preserves `id`, `object`, `created`, the requested alias in `model`,
`choices`, messages, tool calls, `finish_reason`, and `usage` when supplied by the
provider. ModelCairn IDs are distinguished from upstream request IDs. Permitted
additional fields are documented and do not alter the shape required by clients.

## SSE streaming

- `Content-Type: text/event-stream`.
- Every emitted frame follows `data: <json>\n\n` and ends with `data: [DONE]\n\n`
  on normal completion.
- Indices, `delta.role`, `delta.content`, tool-call deltas, and `finish_reason` are preserved.
- An error after commitment closes the stream; it does not insert an incompatible
  JSON response or continue from another destination.
- Usage is delivered in the stream only when the connection/adapter supports it
  and the request asks for it compatibly.

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
