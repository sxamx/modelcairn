# Router failure matrix v1

[Español](matriz-fallos-router-v1.es.md)

Status: initial Milestone 4 contract. This matrix decides whether the router may
try the next destination. Every fallback obeys the attempt cap and total deadline;
it never changes Credential-to-Egress affinity.

| Outcome | Upstream contacted? | Fallback | Initial operational effect |
|---|---:|---:|---|
| invalid local JSON, authentication, or authorization | no | no | safe client error |
| unknown route or no compatible destination | no | no | safe `404` or `422` |
| DNS, refused connection, or TLS failure before send | yes | yes | `transport_error` attempt |
| timeout before body send | yes | yes | `timeout_pre_send` attempt |
| timeout or disconnect with indeterminate send | yes | no | `indeterminate` attempt |
| upstream `400`, `401`, `403`, `404` | yes | no | request/configuration error |
| upstream `408` | yes | initially no | observation without ambiguous retry |
| confirmed upstream `429` | yes | yes | cooldown; honor valid `Retry-After` |
| upstream `500`, `502`, `503`, `504` | yes | yes | transient destination failure |
| any other `4xx` or `5xx` | yes | no | terminal until explicitly classified |
| `2xx` with invalid media type or body | yes | yes before commitment | invalid upstream response |
| valid SSE before downstream headers | yes | not applicable | commits the stream |
| error after response or SSE commitment | yes | no | `partial` result; close connection |
| client disconnect or context cancellation | maybe | no | promptly cancel upstream |
| total deadline or attempt cap exhausted | maybe | no | bounded terminal error |

## Safety rules

- Only outcomes explicitly marked eligible may trigger fallback.
- `Retry-After` is a bounded signal, not permission to exceed the deadline.
- ModelCairn does not log bodies, Authorization, API keys, or credentialed URLs.
- The `internal/testupstream` simulator retains safe metadata only and can script
  statuses, headers, delays, SSE, and connection cuts.

The transport adapter will implement the exact distinction between “before send”
and “indeterminate.” Until then, no test may assume an ambiguous request is safe
to repeat.
