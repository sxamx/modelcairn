# Initial Target Architecture for ModelCairn

- Status: architecture draft
- Scope: first operable version and planned extensions

## Objectives

- Provide a stable endpoint initially compatible with OpenAI Chat Completions.
- Run alongside the operating system on a 1 GB RAM VM.
- Keep configuration, secrets, and decisions on a main node.
- Use lightweight relays for alternative egresses without deploying the complete application or persisting secrets on them.
- Allow protocols, providers, and strategies to be extended through defined contracts.

## Component view

```text
Clients and agents
        |
        v
Compatibility API ──> authentication and policies
        |                         |
        v                         v
Router and strategies <── operational state/estimator
        |
        +──> direct egress ─────────────> provider
        |
        +──> encrypted tunnel ──> relay ────> provider

Console/PWA ──> administrative API ──> configuration and secrets
                                      └─> events, metrics, and audit
```

## Main node

The first implementation will favor a **modular monolith**: a single deployment with clear internal boundaries. This reduces memory, installation, and coordination without preventing components from being separated in the future.

Logical modules:

1. **Compatibility API:** validates and normalizes client requests.
2. **Identity and policies:** authenticates agents and determines permissions.
3. **Router:** executes a published version of the strategy.
4. **Adapters:** connect protocols and providers.
5. **Operational state:** cooldown, circuit breakers, health, and load.
6. **Estimator:** derives capacity, recovery, and confidence.
7. **Control plane:** administrative API for configuration and operation.
8. **Console/PWA:** interface for onboarding, routes, metrics, and diagnosis.
9. **Embedded persistence:** configuration, events, metrics, and audit.
10. **Secret store:** protects credentials and provides their value only to the component that creates the authorized connection.

The console will be built with TypeScript, React, and Vite. In production its static assets will be embedded in the Go executable, so Node.js will not be a resident process and will not consume RAM on the VM.

"Modular monolith" does not mean one giant file. It means the modules run together, but have separate contracts and responsibilities.

## Exit relay

The relay will be a service without a general public administrative interface. Its minimum contract will be:

- accept only authenticated connections from the main node;
- allow only authorized destinations to avoid an open proxy;
- transport the stream without terminating TLS by default;
- not persist bodies, credentials, or responses;
- publish health, version, and minimal metrics without content;
- enforce connection, time, and size limits;
- shut down safely if it loses its configuration or trust.

The concrete tunnel protocol will be selected by ADR after comparing an existing standard solution with a custom relay. Proprietary cryptography or transport will not be implemented if a small standard satisfies the case. Relay health will be exposed only through the channel restricted to the main node, not as an administrative API open to the Internet.

## Persistence

SQLite will be the main node's embedded database because it simplifies a personal installation and reduces resources. The decision must be validated with concurrency, streaming, long retention, backups, and migrations.

Secrets will not be treated as ordinary fields. The architecture will separate:

- queryable operational data;
- secret values encrypted at rest;
- master key or mechanism needed to decrypt them;
- exports without secrets;
- explicit backup of secrets with additional protection.

## Configuration source of truth

There will be a single persisted and versioned source of truth in SQLite. The console and administrative API will modify that source through validations and audit. Files will serve declarative import, export, and automation, not as a second copy continuously observed without precedence rules.

Every modifiable entity will have a schema shared by API, files, and web. Secrets and bootstrap parameters may have more restricted surfaces. The contract will be verified through apply, export, and round-trip cases.

## Request path

1. The client authenticates and sends a request to a logical route.
2. The API validates size, protocol, capabilities, and budget.
3. The router loads the published strategy and obtains eligible destinations.
4. The scheduler combines explicit policy, health, and estimation with uncertainty.
5. The main node creates the attempt through direct egress or encrypted tunnel.
6. The adapter preserves streaming and supported capabilities.
7. An error is classified before allowing wait, cooldown, or fallback.
8. The response returns to the client and events are recorded without content by default.

## Trust boundaries

- The client is not trusted until it authenticates and its request is validated.
- The console requires an administrative session independent from agent tokens.
- The relay is considered owned infrastructure but potentially compromisable; it does not receive readable secrets when tunnel mode permits it.
- The provider is external and necessarily receives the content and credential its API needs.
- Imported files, provider responses, and errors are untrusted inputs and must be validated or redacted.

## Security baseline

Security will be proportional to the risk of a self-hosted application:

- TLS for clients when accessed outside localhost and for external traffic;
- authenticated and encrypted channel between main node and relays;
- API keys encrypted at rest and never fully recoverable from the interface;
- passwords stored with a resistant password hash;
- secure administrative sessions and separation of agent tokens;
- centralized redaction of logs, errors, metrics, and exports;
- minimal file permissions, protected backups, and possible rotation;
- request limits and protection so a relay is not an open proxy.

Provider URLs are sensitive inputs even when configured by an administrator. Before allowing connections, the system must control schemes, DNS resolution, local/private addresses, redirects, and ports to avoid SSRF. Private endpoints will remain possible, but will require explicit and visible operator authorization instead of being accidentally allowed.

High availability, cryptographic hardware, distributed consensus, and military controls are not required for the first version.

## Availability and degradation

The main node is an accepted single point of failure for the first version. If a relay fails, destinations tied to that egress stop being eligible; their API keys do not automatically change egress. The request can only continue through another destination already authorized by the strategy.

Restarting the main node must preserve configuration, affinities, relevant cooldown, and confirmed data. Purely transient state can be rebuilt without presenting old estimates as current.

## Deferred decisions

- exact protocol or tool for the egress tunnel.
- detailed YAML contract and possible later GitOps flow;
- future process separation if measurements justify it.

The primitives and backup format were finalized in [ADR-0005](decisiones/0005-criptografia-y-formato-de-backup.md), and the console stack in [ADR-0006](decisiones/0006-stack-de-consola-web.md).
