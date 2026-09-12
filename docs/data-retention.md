# Local data and retention map

[Español](retencion-de-datos.es.md)

This inventory explains every log or history class separately. Self-hosting means
all application data remains in the operator's installation unless the operator
exports or backs it up. A retention setting controls deletion by age; it does not
send data anywhere.

| Data class | Purpose | Content retained | Initial retention | Configurability | Delivery status |
|---|---|---|---|---|---|
| Failed-login statistics | Diagnose login errors and possible attacks | UTC minute, allowlisted reason, aggregate count | 24 hours | Any duration up to 100 years or unlimited (`0`) | Implemented in Milestone 3 |
| Administrative audit | Explain privileged changes and their actor | Typed action, resource identifiers, result, allowlisted details, timestamp | No automatic expiry yet | Policy/UI to be designed before the audit console | Storage implemented; retention control pending |
| Request history | Inspect gateway traffic and outcomes | Route/alias, timestamps, status, token and latency metrics; no prompt/response by default | 30 days | Planned finite or unlimited retention | Schema reserved; router recording pending |
| Attempt/fallback history | Explain which destination was tried and why routing continued | Sequence, destination, outcome, provider status, error class, fallback reason | Follows request history | Planned finite or unlimited retention | Schema reserved; router recording pending |
| Adaptive rate-limit observations | Learn provider limits and recovery behaviour | Scope, normalized limit/reset values, source and timestamp; no raw response body | Long-lived operational history, exact default pending benchmark | Planned configurable, including unlimited | Schema reserved; estimator pending |
| Active cooldowns | Prevent sending work to a destination known to be unavailable | Scope, reason, start/end and supporting observation | Until cooldown ends or is superseded | Behavioural state, not a user archive | Schema reserved; router pending |
| Process logs | Diagnose service startup and runtime failures | Structured events with route templates; no credentials or request bodies | Controlled by systemd/container log policy | Configured outside ModelCairn initially | Implemented output; installer policy pending |
| Backups/exports | Operator-controlled recovery and portability | Explicit snapshot/export selected by the operator | Until the operator deletes it | Entirely operator-controlled | Partial formats exist; complete backup is a later milestone |

## Rules shared by every class

- Prompts and model responses are not stored by default.
- Passwords, API-key values, session/CSRF tokens and agent bearer values are never
  history fields.
- The future web console must show the estimated disk consequence before enabling
  unlimited retention and must distinguish “unlimited” from “disabled”.
- Changing a retention setting must use the same file/API/web settings model and
  prune in small transactions rather than blocking request handling.
- Retention controls apply only to their named class. Deleting login statistics,
  for example, must not delete provider performance history.

This map is the source of truth for future retention controls. A later milestone
may refine pending defaults after representative storage benchmarks, but it must
record that decision here and in the corresponding executable contract.
