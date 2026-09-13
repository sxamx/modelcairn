# Milestone 5 — Console and PWA

[Español](hito-05-plan-tecnico.es.md)

Status: in progress since September 13, 2026.
Prerequisite: Milestone 4 merged and accepted.

## Outcome

Deliver a responsive, installable web console that completes the primary case —
sign in, register a provider secret, build and publish a route, and issue an agent
token—without editing files or code. It also shows health and operational activity
without exposing secrets, prompts, or responses.

The console is a TypeScript, React, and Vite SPA. Compiled files are embedded in
the Go executable; Node.js does not run in production. The administrative API
remains the sole authority and the UI does not keep a second configuration source.

## Boundaries

- Includes authentication, guided onboarding, resource CRUD, secret metadata,
  token lifecycle, basic diagnostics, responsive design, PWA, and accessibility.
- Excludes the visual strategy editor, relays, backup/restore, systemd installer,
  and prompt/response content for now.
- Secret values go only to the write endpoint, are cleared from form state after
  use, and never return from the server.
- The session uses an `HttpOnly` cookie; CSRF stays only in memory and is recovered
  through `/session/me` after reload.

## Delivery blocks

1. **Web foundation (implemented):** reproducible frontend workspace, OpenAPI-derived types,
   accessible shell, embedded assets, safe navigation fallback, manifest, and a
   minimal service worker.
2. **Access and session (initial implementation):** login, session recovery, logout, loading states, and
   uniform errors; no administrative route is visible while unauthenticated.
3. **Operational overview (initial implementation):** health/readiness, resource counts, route and
   destination status, recent requests and attempts through bounded DTOs.
4. **Guided onboarding (initial implementation):** provider → account/connection → secret → model →
   destination → strategy → route → token, with validation before confirmation.
5. **Daily administration (advanced implementation):** resource lists and forms, optimistic ETag updates,
   replace-only secrets, and one-time token display.
6. **Diagnostics and history (implemented):** filters and pagination for requests, attempts and
   errors; latency, TTFT, tokens, and fallback without content.
7. **Closeout (in progress):** frontend and contract tests, automated accessibility, reproducible
   build, bundle/RAM budget, mobile/PWA exercise, and grouped QA.

## Implementation decisions

- Use `fetch` with a generated typed client; do not add cache or routing libraries
  until a measured need justifies them.
- The server serves `/`, known SPA routes, and assets with distinct cache policies.
  `/api/*`, `/v1/*`, `/healthz`, and `/readyz` never receive HTML fallback.
- `index.html` is not durably cached; hashed assets may be immutable.
  Administrative responses remain `no-store`.
- The service worker caches only the static shell. It never intercepts or stores
  API, health, prompts, responses, or credentials.
- The interface keeps discreet, visible attribution to ModelCairn and its
  repository, consistent with `NOTICE` and the license decision.

## Exit criteria

1. A configured installation completes the primary case from the web.
2. Reload, logout, session expiry, and connectivity loss produce understandable,
   recoverable states.
3. No secret reappears in DOM, responses, logs, web storage, or caches.
4. The PWA is installable under HTTPS or localhost and keeps its shell available
   while the backend is temporarily unreachable, clearly showing offline state.
5. Keyboard navigation, focus, accessible names, contrast, and mobile sizing pass
   the agreed automated gate.
6. Initial compressed JavaScript remains below the 250 KiB target or documents a
   justification without exceeding 400 KiB.
7. Go, frontend, contract, embedded-build, and representative benchmark gates pass
   without sustained swap or open critical/high findings.

## Verification strategy

Each block includes focused tests. End-to-end tests use a real temporary server
with synthetic data, never real providers or secrets. Independent QA is grouped
after the functional blocks and before the benchmark, avoiding repeated reviews
for editorial changes.
