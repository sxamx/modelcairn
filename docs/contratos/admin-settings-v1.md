# Administrative settings and login audit v1

[Español](admin-settings-v1.es.md)

Status: proposed implementation contract; grouped design review recorded in the milestone plan. Retention decision pending.

## Settings source and lifecycle

Use one versioned SQLite settings record, separate from provider resources.
The initial `modelcairn admin bootstrap` imports a bounded settings document;
later file updates use explicit plan/apply with installation ownership offline
or the administrative API online. Merely editing an exported file changes no
running state. Export, API and the future console share the same settings model.

The document has `apiVersion: modelcairn.io/v1alpha1`, `kind: AdminSettings`,
`resourceVersion` (positive integer for updates, omitted for initial creation),
and `spec` with the fields below. Unknown fields, duplicate keys, YAML aliases,
nulls, multiple documents and files over 64 KiB are rejected. Omitted fields
preserve values on update and use the listed defaults on creation. Updates are
atomic and require the observed version. This is a separate document from the
existing Configuration envelope and does not silently extend its resource kinds.

The [structural schema](config/admin-settings-v1alpha1.schema.json) separates
initial, update and fully resolved documents. Defaults are annotations: the shared
decoder must resolve them explicitly, never replace omitted update fields with
defaults. Semantic validation then checks idle <= absolute, client rate/burst <=
global, canonical origin/CIDRs, a literal listen IP with port 1..65535, and transport
combinations. loopback-http requires HTTP and loopback origin/listener; both TLS
modes require an HTTPS origin. proxy-tls requires nonempty trustedProxyCidrs;
other modes require an empty list. direct-tls requires both nonempty TLS paths;
other modes require empty TLS paths. Switching modes must explicitly clear old
mode-specific fields. TLS paths must be absolute local paths without NUL bytes;
validate file accessibility at startup without returning contents or raw OS errors.

GET `/settings` returns desired/effective fully resolved documents and
restartRequired, computed from differing effective specifications, not merely
different revision numbers. `/settings/plan` and `/settings/apply` accept update
documents, never initial creation. Successful no-op apply consumes the plan but
does not bump the settings revision or require restart. Plan returns the resolved
desired document (with the observed revision), changed field names, token and
expiry. Apply returns the committed settings snapshot and appliedAt; it does not
re-read settings after releasing the transaction. GET is also the export source.

| spec field | Type / initial value | Validation |
|---|---|---|
| publicOrigin | required string | Canonical origin under admin-runtime-v1 |
| listen | string, 127.0.0.1:8080 | Literal IP and port, no hostname resolution |
| transport | enum, loopback-http | loopback-http, direct-tls, proxy-tls |
| trustedProxyCidrs | string array, empty | Up to 32 canonical CIDRs; required only for proxy-tls |
| tlsCertificatePath | string, empty | Required for direct-tls; service-readable local certificate |
| tlsPrivateKeyPath | string, empty | Required for direct-tls; private service-readable key file |
| idleSeconds | integer, 1800 | 300..86400 and at most absoluteSeconds |
| absoluteSeconds | integer, 43200 | 300..604800 |
| globalAttemptsPerMinute | integer, 30 | 1..120 |
| globalBurst | integer, 5 | 1..20 |
| clientAttemptsPerMinute | integer, 5 | 1..30, no greater than global rate |
| clientBurst | integer, 3 | 1..10, no greater than global burst |
| maxClientEntries | integer, 1024 | 64..4096 |
| clientIdleSeconds | integer, 900 | 60..3600 |
| argonMemoryKiB | integer, 19456 | 19456..65536 |
| argonIterations | integer, 2 | 2..6 |

Argon parallelism and concurrent verification are fixed at one for this version.
Cooldown progression remains fixed by admin-runtime-v1. Raising hash parameters
affects new password hashes; existing supported hashes remain verifiable. Reading
a stored PHC never permits allocations beyond the supported hard bounds.

For simplicity all settings changes become effective on explicit restart. HTTP
returns both desired/effective versions and restartRequired. Reject invalid
combinations before persistence. A successful update cannot change the current
listener or invalidate the connection used to save it mid-response. On startup,
validate all desired settings and certificate/key accessibility before listening;
failure requires correction through the offline CLI, not fallback to insecure HTTP.
TLS key contents are never exposed by settings export or API.
Session durations are captured at issuance: after restart the new durations apply
only to newly issued sessions, not to existing ones. An already expired session
cannot become valid because an operator increases idleSeconds.

The [storage draft](storage/admin-identity-v1.md) specifies a new migration
containing an `admin_settings` singleton with a version,
validated JSON document and updated timestamp. Never edit published migrations.
Implement GET/plan/apply settings endpoints with the existing session/CSRF policy;
settings plan tokens bind the settings revision and a distinct operation purpose.
They also bind the digest of the fully resolved, normalized desired document,
including preserved values and defaults. Use the existing configuration-plan
expiry limit and single-use transaction semantics. Apply must match that document
and revision, reject expired or consumed tokens, and consume the token atomically
with the settings update and audit. Failed transactions do not consume the plan.
Provider configuration tokens cannot authorize settings updates or vice versa.

## Failed login audit

Persist aggregate minute buckets rather than one event per rejected login. Each
bucket contains only UTC minute, fixed reason enum and a saturating integer count.
Reasons: invalid_credentials, throttled, malformed, unavailable. No username, IP,
Origin, headers, password or free-form error text is stored. Keep four counters in
memory per active minute and flush at most once per minute using upsert. At shutdown
flush remaining counts best-effort. A crash can lose the last unflushed minute;
these are operational statistics, not a durable forensic event stream.

Bound this dedicated table to 1440 minute buckets per reason (5760 rows). Prune
expired buckets in the flush transaction. A large forward clock jump prunes old
buckets; backwards clock changes cannot expand the row cap. Disk-full errors do
not buffer an unbounded retry queue; retain only bounded current counters and
report an aggregated unavailable status. Login success and administrative mutations
still require their normal transactional audit record. This operational window
does not change provider-event retention or its indefinite-retention option.

Before implementing this policy, retain it as a proposal: the operator must approve
this fixed login-statistics window or choose configurable retention if desired.
