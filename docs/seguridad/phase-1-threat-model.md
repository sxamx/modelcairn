# Phase 1 threat model

[Español](modelo-amenazas-fase-1.es.md)

- Status: current for Milestone 7 validation
- Scope: self-hosted primary node, console, data API, SQLite, keyring, MCB1
  backups, and direct provider egress
- Out of scope: relays, adaptive estimator, and high availability

## Assets and objectives

Priority assets are provider API keys, administrative password and sessions,
AgentTokens, configuration/affinities, backups, and in-transit content. Secrets
require confidentiality and integrity; confirmed configuration and recovery require
reasonable availability.

ModelCairn does not claim to defend an instance from an already-compromised host
root. It must limit accidental disclosure, unauthenticated clients, hostile inputs,
malicious providers, and process failures.

## Trust boundaries

1. **Client → data API:** the client is untrusted until AgentToken, size, JSON,
   model, and capabilities are validated.
2. **Browser/CLI → admin API:** password, session, and CSRF are separate from an
   AgentToken. One role does not grant the other.
3. **Process → local storage:** SQLite and keyring require exclusive ownership and
   permissions; files, YAML, and imported backups are hostile inputs.
4. **Node → provider:** provider URL, DNS, redirects, headers, errors, and bodies
   are untrusted. The provider necessarily receives the prompt and credential used
   for that request.
5. **Operator → network:** loopback HTTP is local; private/public access requires
   documented TLS/proxy modes. ModelCairn does not make remote HTTP safe.

There is no relay boundary in Phase 1. Adding one requires reviewing this model
before implementing RF-101–RF-108.

## Required threats and controls

| Threat | Phase 1 control | Primary evidence |
|---|---|---|
| API key stolen from disk/output | Identity-bound XChaCha20-Poly1305, separate keyring, common redactor, write-only API | `secret_crypto`, `secret_store`, `redact` tests; ADR-0005 |
| Admin brute force | Bounded Argon2id, global/client admission, backoff, uniform error, aggregate statistics | `admin_login` and `failed_login_statistics` tests |
| Session theft/fixation and CSRF | Transport-appropriate cookie, idle/absolute expiry, CSRF window, atomic revocation | `admin_session` tests and session contract |
| Stolen/overbroad AgentToken | Hash at rest, one-time delivery, allowed routes, expiry, revocation | `agent_token` tests |
| SSRF or credential leakage | Controlled schemes/private network, validated resolution, rejected redirects, credential injection after validation | `parse`, `direct_transport`, `openaiadapter` tests |
| Retry/fallback amplification | Attempt/time budget, allowlisted classification, cancel, no fallback after stream commitment | `router` and `streaming` tests |
| Content/secret in history or logs | Content-free tables, zero flags, typed log routes, canaries, redaction | `operational`, `server`, `redact` tests |
| Concurrent/tampered configuration | Signed single-use plan, optimistic version, transaction, exclusive owner | `plan_token`, `config`, `lifecycle` tests |
| Interrupted migration/rotation/restore | Transactions, generational publication, authenticated key, pre-verification, rollback | `lifecycle`, `rotation`, `backupmcb1` tests; Milestone 6 evidence |
| Backup read or replacement | Passphrase authenticated encryption, no-replace creation, file bounds, new-generation restore | `mcb1` tests; MCB1 contract |
| Console/API abuse | Required authentication, input bounds, pagination, optimistic concurrency | HTTP, storage, and web tests |

## Privacy and data

- No built-in external telemetry exists.
- Prompts and responses are not persisted by default.
- Operational metadata remains local until an implemented policy deletes it; the
  retention map identifies controls that remain deferred.
- Exporting, backing up, or copying logs is an operator action and expands where
  those artifacts must be protected.

## Accepted residual risks

- The primary node is a single point of failure.
- Compromised root can read process memory or replace the binary.
- The provider sees sent content and may retain its traffic.
- Loopback HTTP relies on host security; remote exposure without TLS is forbidden,
  not magically repaired by the application.
- General configurable retention is RF-202; until then the operator must monitor
  disk and manage copies according to the runbook.

Any finding that exposes secrets, bypasses authentication/authorization, evades
network policy, or invalidates recovery blocks Phase 1 closeout.
