# Administrative identity storage — Milestone 3

[Español](admin-identity-v1.es.md)

Production migrations `0003_admin_settings.sql` and
`0004_failed_login_statistics.sql` implement this contract through the existing
transactional runner and checksum/schema-compatibility checks. Published
migrations 0001 through 0003 remain unchanged.

## Settings

`admin_settings` has at most one row, keyed by 1. Store the fully resolved `spec`
only, not the API envelope: `resource_version` is the sole persisted version.
Reconstruct apiVersion/kind on export. SQL checks object shape and a 64 KiB byte
bound; the shared validator enforces all fields, types, defaults and semantic
relationships before persistence and again on startup. SQL is not a substitute
for that validator. Initial bootstrap commits administrator, settings revision 1
and typed success audit together. An existing administrator without settings is
an incomplete installation: fail closed and require offline repair, not another
administrator or an insecure default listener.

Updates compare the observed revision and persist the new spec/version and audit
with plan consumption in one transaction. A no-op preserves revision but consumes
its plan. Use existing consumed_plan_tokens storage and key-rotation behavior,
with a settings-specific authenticated purpose/digest. Do not introduce a second
consumption mechanism. Settings update audit contains versions and changed field
names, never values. Effective settings are the validated startup snapshot in
process memory, not a second mutable database row.

Runtime persistence decodes only byte-for-byte canonical resolved JSON and validates
it again. Revision-one insertion is a transaction primitive for bootstrap: its
caller remains responsible for creating the administrator and typed audit in that
same transaction. No public workflow may commit only one of those records.

The local identity service now composes that bootstrap: it validates username,
settings and password before writing, derives Argon2id before opening the write
transaction, then commits the administrator, settings and `admin.bootstrap`
together. Exactly one concurrent attempt may win; partially formed installations
fail closed for offline repair. `admin reset-password` takes exclusive ownership
through the installation lock, reads persisted Argon2id parameters, derives outside
the transaction, then commits the password, auth_version increment, all-session
revocation and `admin.password_reset` together. Audit failure rolls everything back.
Passwords enter through a confirmed TTY prompt or stdin, never argv.

`UpdateAdminSettingsTx` now validates the resolved spec, checks the observed version,
updates the singleton and inserts typed `admin_settings.apply` audit within its
caller's transaction. Changed-field names come from validated settings; values are
excluded. No-op preserves revision and updated_at while recording the operation.
The service must compose it with `ExecuteSettingsPlan` and roll back on any error.
Regression tests inject an audit failure and prove both settings and nonce roll
back, followed by successful retry. The public plan/apply service remains pending.

## Sessions

The persistent session layer generates independent 32-byte session and CSRF values
and stores only SHA-256 hashes. Issuance rechecks id, username and auth_version in
the transaction after password verification, captures idle_seconds and absolute
expiry, and commits `admin.session_create` with the row. A reset race leaves no
valid session.

Every use checks the row, revocation, current auth_version, absolute expiry and the
captured idle policy before advancing last_seen. Clock rollback never moves
last_seen backwards. CSRF comparison is constant-time; rotation stores only the
current hash and one previous hash for 60 seconds. Logout authenticates session and
CSRF, then commits revocation with `admin.session_logout`. This layer does not decide
Origin, cookies, login admission or HTTP responses.

The internal login service applies global and trusted-client-IP token buckets,
progressive backoff and the configured bounded/expiring client map before Argon2id.
A capacity-one permit rejects concurrency without a queue and covers every real or
dummy derivation. Missing users and wrong passwords both derive and return
`invalid_credentials`; PHC or persistence errors return unavailable. Transactional
session creation rechecks auth_version after hashing. Success clears backoff without
refilling buckets. The service exposes the login endpoint and records only
aggregate failed-login minute/reason counters through migration 0004.

Migration 0003 revokes every pre-existing live session before adding idle_seconds;
their original idle policy is unknown. Historical revoked rows may retain NULL.
Authentication rejects NULL regardless of any other field. Newly issued sessions
must capture a non-NULL idle duration and absolute expiry from effective settings;
the SQL check prevents an unrevoked row without a duration. Changing settings never
changes a session's captured policy. No passwords or bearer values are migrated.

Migration failure rolls back both revocation and DDL. Once applied successfully,
existing sessions must log in again. Go lifecycle tests cover the migration ledger;
HTTP enforcement requires later implementation tests.

## Audit and acceptance

Identity success actions reuse audit_events with typed action-specific details;
no new broad free-form audit API is introduced. Migration 0004 stores only UTC
minute, allowlisted reason and saturating count. Effective settings apply 24 hours
by default, any configured finite duration, or zero for no time expiry.

Run `node docs/contratos/storage/admin-identity-v1.contract.test.cjs` to exercise
upgrade of a seeded pre-Hito-3 database, revocation, settings bounds and rollback.
Before runtime acceptance additionally test concurrent bootstrap, session policy
capture, expired-session non-revival, transactional plan/audit failures, checksum
compatibility and a real restart through the production runner. Go lifecycle tests
already cover sequential upgrade, checksum/schema compatibility and repeat startup;
HTTP enforcement remains future work; local bootstrap and reset now cover
concurrency, rollback and secret-safe input.
