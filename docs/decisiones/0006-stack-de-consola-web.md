# ADR-0006: Web console stack

[Español](0006-stack-de-consola-web.es.md)

- Status: accepted
- Date: 2026-09-05

## Decision

The console will be an SPA/PWA built with TypeScript, React, and Vite. The static
output will be embedded in the Go executable for production; Node.js will be a
build and development dependency, not a resident process on the VM.

Runtime dependencies will initially be limited. The OpenAPI-generated client will
share types with forms; backend contracts will not be manually duplicated.
Navigation, forms, and server cache may use small libraries only after their cost
is justified. The future visual editor will be evaluated separately.

## Rationale

- TypeScript reduces divergence between the API and forms.
- React supports a complex, composable console, including the future canvas.
- Vite produces static assets and maintains a simple development workflow.
- Embedding assets avoids consuming RAM with a separate frontend server.

## Initial budgets

- Initial compressed JavaScript: target below 250 KiB, provisional maximum of
  400 KiB before requiring splitting or justification.
- Lazy loading for heavy metrics and future editors.
- No SSR in Phase 1.
- Lighthouse and automated accessibility checks as signals, not substitutes for
  real mobile and screen-reader testing.

## Sources

- [Using TypeScript with React](https://react.dev/learn/typescript)
- [Official React guide for an application with Vite](https://react.dev/learn/build-a-react-app-from-scratch)
