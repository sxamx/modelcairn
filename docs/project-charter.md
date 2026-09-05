# ModelCairn Project Charter

[Español](project-charter.es.md)

- Status: approved as the basis for Phase 1; will evolve through ADRs and phases
- Opening date: 2026-09-05
- Stage: discovery and definition
- Audience for this document: project maintainers, contributors, and reviewers

## How to use this document

This Project Charter defines why ModelCairn exists and what outcome it aims to achieve. It does not yet define the complete technical solution. Confirmed statements form the project baseline; proposals require owner review; open questions must not become requirements or implementation tasks.

## Summary

ModelCairn is a project intended to be published as open source to build a lightweight, self-hosted, configurable AI provider gateway. It sits between clients or agents and providers, applies routing and fallback strategies defined by the operator, observes each route's capacity and recovery, and provides a web console to administer the system.

The project aims for clients to depend on a stable endpoint even when the available providers, models, or operating conditions change. Routing decisions must be visible, explainable, and compatible with the policies and accounts the operator is authorized to use.

## Problem

AI providers and models change frequently. A direct integration forces each client to know credentials, endpoints, availability, limits, errors, and replacements. Existing gateways may consume too many resources, include unnecessary features for small installations, or limit how the operator defines routes and fallback.

ModelCairn aims to concentrate that complexity in a layer operable from files and from a web interface, capable of running on small infrastructure and growing toward multiple nodes without losing traceability.

## Confirmed vision

- Project intended to be published on GitHub under an open source license.
- Intermediate gateway between AI clients and authorized providers.
- Stable API to avoid reconfiguring each client when a route changes.
- Configurable providers, models, credentials, execution locations, and authorized egresses.
- Centralized custody of credentials on the main node. Exit nodes relay requests without persisting API keys, and each credential maintains stable affinity with an authorized egress.
- Configurable routing and fallback, including an advanced visual representation.
- Identification of clients or agents to apply explicit policies.
- Capacity- and policy-aware distribution of concurrent requests.
- Adaptive estimation of capacity and recovery based on observations.
- Operational configuration manageable through a common schema from files and the web console. Secrets, bootstrap, and recovery will have explicit boundaries to avoid exposure or contradictory sources of truth.
- Lightweight architecture with a target installation on a 1 GB RAM VM.
- Compatibility with private networks such as Tailscale without depending exclusively on a single networking technology.
- Development divided into complete, documented, and verifiable phases.
- Independent QA on relevant deliveries to reduce implementer bias.
- Precise and attractive public communication to support adoption, contributions, and project sponsorship.

## Confirmed principles

### Documentation before implementation

Product and architecture decisions are documented before developing the phase that depends on them. Documentation must reduce ambiguity, explain consequences, and allow early course correction.

### Complete phases

Each phase will have scope, acceptance criteria, tests, and documentation. A phase is not considered finished because it contains a partial demonstration or a visual interface.

### Operator control

The operator defines providers, routes, priorities, fallbacks, affinities, and policies. Automatic decisions must be explainable and must respect configured boundaries.

### Measurable efficiency

The goal of running on a 1 GB VM will be validated with reproducible measurements and load tests. Technology choices must be justified against that budget.

### Extensibility

Adding providers, models, nodes, or strategies should not require rewriting the core. Extension contracts will be part of the public documentation.

### Privacy and security

Credentials, tokens, local paths, and sensitive data must not appear in documentation, logs, or public exports. Security is designed as part of each phase, not as a final correction.

The target level will be proportional to a self-hosted administrative application, not to military or highly classified infrastructure. The priority will be protecting API keys; then preventing unauthorized access to the console and protecting communication in transit between the main node, relays, and providers. The design will include encryption in transit, protection of secrets at rest, log redaction, authentication between nodes, and secure backups, without adding complexity that does not respond to a real project risk.

### Self-hosting without external telemetry

Each installation belongs to whoever operates it and keeps its configuration and operational data locally. ModelCairn will not send telemetry, analytics, prompts, responses, or identifiers to the maintainer or to third parties. Local metrics needed for routing, diagnosis, and capacity estimation do not imply a central ModelCairn service.

### Bilingual documentation

Discussion and first drafts will be written in Spanish to facilitate maintainer review. After each document is approved, a canonical public version in English and an official Spanish translation will be maintained. The documentation process must define how to detect and avoid divergence.

## Identified stakeholders

- **Main maintainer during the initial stage:** facilitates the vision and approves scope decisions while future governance is being defined.
- **Operator:** installs ModelCairn, configures infrastructure, providers, and policies, and responds to incidents.
- **Client or agent user:** uses the gateway endpoint and expects stable and understandable behavior.
- **Contributor:** studies the documentation, proposes changes, and implements a part without needing to reconstruct decisions from private conversations.
- **Reviewer:** evaluates quality, security, performance, design, or communication with independent judgment.

## Priority user and experience

The initial experience will be designed for a person with basic or intermediate technical knowledge who understands concepts such as API, provider, model, and VM, but should not need advanced systems administration experience.

The user's technical knowledge does not justify a complex interface. The console must apply **progressive disclosure**: show common decisions first, explain their effects, and keep advanced controls available without imposing them on the basic flow.

The system must also be useful for expert operators and evolve toward small teams, but those needs must not degrade the personal installation or turn the basic interface into a hostile panel.

## Initial compatibility

The first compatible surface will be `POST /v1/chat/completions`, including the necessary streaming and tool-call behaviors that each route supports. This priority fixes the implementation order; it does not limit the complete vision.

Each logical route may expose a model alias different from the provider's real identifier. Aliases are part of general routing and do not by themselves imply compatibility with another protocol.

The architecture must allow incorporating other protocols through adapters. Future cases include an Anthropic-compatible interface for clients such as Claude Code. Compatibility will not be considered complete if a translation silently loses tools, reasoning, content, or semantics required by the client.

Responses API, Anthropic Messages, and other protocols will be evaluated in their own compatibility contract before being assigned to a phase.

## Installation and onboarding

The target experience starts with one installation command. Then an interactive terminal assistant performs only the necessary bootstrap:

- check operating system, architecture, and resources;
- install or validate the required runtime;
- create the service and data directory;
- create the initial administrative account or password;
- choose how the console will be accessed;
- start ModelCairn and show the URL and next steps.

Operational configuration of providers, models, credentials, and strategies will happen afterward from the web console or through files. The assistant must not ask for API keys unnecessarily or require reinstallation to change configuration.

The main path will be a Go executable managed by systemd. The installer will ask whether it should start automatically on reboot, recommending that it be enabled. Linux AMD64 and ARM64 artifacts will be published. Docker will be added as an alternative after validating native installation and its operational differences.

## Web access and PWA

The console will be a Progressive Web App (PWA) installable on mobile and desktop devices. The installer will offer understandable access modes:

- only on the local machine;
- private network or LAN;
- private tailnet through Tailscale Serve and HTTPS;
- public exposure through a user-managed HTTPS reverse proxy.

Tailscale integration is optional. ModelCairn can detect its availability and generate instructions, but it does not depend on Tailscale and must not make a private instance public. Tailscale Funnel will not be the recommended mechanism for an administrative console.

## Known constraints

- Initial target deployment on Linux on a VM with 1 GB of RAM.
- Ability to operate at least two owned nodes and grow to more nodes.
- Web console served within a reduced resource budget.
- Configuration understandable for a personal installation, but extensible to more complex scenarios.
- The new public repository must start with a clean history and reviewed documentation.
- Phases can be developed quickly, but none may omit their tests, documentation, or acceptance criteria.

## Hypotheses we must validate

- An OpenAI-compatible API covers the initial integration for most target clients.
- A lightweight process with embedded persistence can meet the budget of a 1 GB VM together with the operating system and basic services.
- Configuration managed from web and files can share a schema without producing two sources of truth, while keeping referenced secrets and bootstrap parameters out of inappropriate surfaces.
- Capacity and recovery can be estimated usefully without interpreting a few observations as permanent rules.
- A main node can custody secrets and coordinate lightweight exit nodes without those nodes persisting credentials or request content.

## Outcomes that will define success

Numerical metrics will be agreed during non-functional requirements. As a general outcome, ModelCairn will be successful when:

- an operator can install it and configure a route without modifying code;
- an operator can change the route behind a compatible logical identifier without reconfiguring the client;
- an allowed failure produces a traceable fallback within the time and attempt budget;
- the estimator communicates capacity, recovery, and uncertainty without presenting guesses as confirmed limits;
- a reference installation runs within the real VM budget;
- a contributor can understand architecture, contracts, and delivery process from the repository;
- every important automatic decision can be explained from the console and permitted logs.

## Local metrics and retention

Operational metrics are a core function: they feed the adaptive estimator, explain routing, and allow analysis of historical behavior. They remain in the operator's installation and are not sent to the project.

The proposed initial policy distinguishes original data from derived views:

- detailed events with a default retention of 30 days;
- historical statistics calculated from those events to show, among other data, latency, tokens, tokens per second, errors, availability, and behavior by model, provider, and route.

Aggregated statistics neither replace events nor require deleting them. The operator may keep detailed events indefinitely. The interface must estimate disk growth, warn before space is exhausted, and allow controlled export, compaction, or deletion of data. The proposed default retention is 30 days, with an explicit option to never delete. Exact periods and granularity will be finalized as non-functional requirements.

## Topology, health, and affinity

The initial topology will use **centralized control**:

- the **main node** runs the gateway, console, persistence, estimator, scheduler, and custody of all API keys;
- the main node can also act as a direct Internet egress;
- **exit nodes** are lightweight relays: they transport authorized connections from the main node to the provider through their own egress;
- exit nodes are not a second complete installation, do not make routing decisions, and do not store API keys persistently.

The design will prefer an egress tunnel in which TLS encryption terminates at the provider, not at the relay. That way the exit node transports traffic, but does not receive the API key, prompt, or response in readable text. If a future integration needed to terminate TLS or reconstruct requests in the relay, it would be considered a different mode and must pass an explicit security review.

This decision concentrates the administration surface and secrets, but makes the main node a critical component. The architecture must protect the channel between nodes, authenticate both ends, prevent a relay from accepting arbitrary traffic, and define what happens if the main node or a relay becomes unavailable.

The console will include a view of registered nodes. For each node it must show, at minimum:

- connectivity status and last check;
- ModelCairn version;
- declared and observed resources when available;
- active requests and recent load;
- configured providers;
- number of assigned credentials, without revealing their secrets;
- routes and models it can serve;
- maintenance status and unavailability reason.

A credential will belong to the main node's secret store and will have stable affinity with an authorized egress. The scheduler will not change that assignment automatically. Reassignment will require an explicit operator action, dependency check, audit record, and a warning that operational conditions and possibly the network identity observed by the provider will change.

Affinity does not imply that node, proxy, and egress are the same object. That relationship will be defined in the domain model and must support installations without a proxy, shared proxies, and private networks such as Tailscale.

## Proposed project boundaries

The following boundaries are a proposal for discussion, not a decision already made:

- ModelCairn manages routing of AI requests; it does not aim to become a general agent platform.
- It does not host models or offer its own inference.
- It does not try to replace general observability, secret managers, or external private networks; it integrates with them through simple contracts.
- It does not guarantee availability when no authorized and healthy route exists.
- It does not present statistical inferences as official provider quotas.
- The console administers ModelCairn; it does not aim to be a universal panel for every function of each provider.

## Planned hypothesis validation

This table proposes how to prevent assumptions from becoming decisions without evidence. Specific owners and thresholds will be set during planning.

| Hypothesis | Planned evidence | Stage |
|---|---|---|
| Sufficient initial compatibility | Client matrix and contract tests | Requirements and architecture |
| Operation within 1 GB | Memory and load profile on a representative VM | Technical validation |
| Common schema for file and web | Schema prototype, round-trip, and secrets/bootstrap cases | Architecture |
| Useful adaptive estimation | Simulations, synthetic data, and test providers | Statistical design |
| Centralized secrets and relays without persistence | Threat model, relay inspection, and multinode test | Security and architecture |

## Resolved and deliberately deferred decisions

The decisions required to begin Phase 1 are resolved below. Items explicitly
deferred belong to later milestones and do not reopen this charter.

### Priority users

Decision made: the initial experience prioritizes a person with basic or intermediate technical knowledge and their own VM. The interface will remain intuitive and apply progressive disclosure. Expert operators and small teams are part of the expected evolution.

### Initial compatibility surface

Decision made: OpenAI Chat Completions will be the first interface. The architecture will allow later adapters; Anthropic compatibility for clients such as Claude Code remains an explicit case for the future protocol contract.

### Deployment model

Decision made: Go executable and systemd as the main path, with automatic startup selectable during bootstrap. Linux AMD64 and ARM64 will be supported; Docker will be a later alternative, not a dependency of the first installation.

### Network exposure

Decision made: the initial configuration will favor localhost or a private network. Tailscale Serve for private HTTPS and public exposure behind an HTTPS reverse proxy will be supported, without enabling public access automatically.

### Documentation language

Decision made: English is the canonical public version and Spanish is the official translation, working first on drafts in Spanish. Both versions are reviewed together for the initial baseline; an automated divergence check is deferred.

### License and governance

Decision made: [ADR-0002 adopts Apache-2.0](decisiones/0002-licencia-y-atribucion.md) with `NOTICE`, attribution in the official interface, and a future trademark policy. The initial contribution workflow and decision authority are defined in [project management](governance/project-management.md); mature community governance is deferred until the contributor base requires it.

### Telemetry and content

Decision made: there will be no external telemetry or central collection service. By default, prompts and responses will not be stored: only metrics and operational metadata that do not need to reproduce the content will be kept. A future feature may allow explicit local storage by policy, but before that it must define purpose, privacy warnings, encryption, redaction, retention, access, and deletion. The adaptive estimator will be designed to operate without reading or retaining message content.

### Configuration, secrets, and recovery

Decision made: SQLite will be the source of truth; YAML will allow applying and exporting configuration through the same schema used by web and API. API keys will be encrypted with a local master key. There will be export without secrets and a full encrypted backup with an independent password, whose restore must recover a functional route. The administrative password will be reset from the local CLI.

### Nodes, proxies, and egress

Decision made: the main node hosts the complete application, stores all credentials, and can also provide an egress. Auxiliary nodes will be lightweight exit relays, not complete installations or persistent secret stores. Each credential is bound to an egress and only the operator can change that affinity. The relay protocol, authentication, encryption, failure behavior, and resource budget are deliberately deferred until the relay milestone and must be contracted before implementation.

### Compatibility and provider authority

The Phase 1 surface is governed by the Chat Completions compatibility matrix. Compatibility for later protocols and the representation of provider-published limits and terms are deferred to their milestones. Official information and explicit provider instructions take precedence over observed estimates; a detailed conflict policy must be contracted before adaptive routing is implemented.

## Approval criterion

This charter was accepted for Phase 1 after maintainer review of the vision and boundaries, definition of validation methods for its hypotheses, and independent review without remaining blocking findings. A later ADR or phase contract may evolve it without rewriting this historical approval.
