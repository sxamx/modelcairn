# Administrative identity storage — Milestone 3

[Español](admin-identity-v1.es.md)

Production migration `0003_admin_settings.sql` implements this contract through
the existing transactional runner and checksum/schema-compatibility checks.
Published migrations 0001 and 0002 remain unchanged. The executable contract test
reads migration 0003 directly so a second SQL copy cannot drift from production.

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

## Sessions

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
no new broad free-form audit API is introduced. Failed-login aggregate storage is
intentionally deferred until its retention policy is decided. Migration 0003 neither
creates that table nor silently approves a 24-hour cap.

Run `node docs/contratos/storage/admin-identity-v1.contract.test.cjs` to exercise
upgrade of a seeded pre-Hito-3 database, revocation, settings bounds and rollback.
Before runtime acceptance additionally test concurrent bootstrap, session policy
capture, expired-session non-revival, transactional plan/audit failures, checksum
compatibility and a real restart through the production runner. Go lifecycle tests
already cover sequential upgrade, checksum/schema compatibility and repeat startup;
HTTP enforcement and installation bootstrap remain future delivery work.
