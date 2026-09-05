# ModelCairn Glossary and Domain Model

- Status: draft for joint review
- Stage: discovery and definition
- Scope: common vocabulary and conceptual relationships; does not yet define tables or classes

## Purpose

This document prevents the same word from representing different concepts during project design, implementation, and conversations. When the interface uses a simpler word, the technical documentation must relate it to the corresponding domain term.

## Actors

### Operator

Person who installs and administers a ModelCairn instance. Controls its data, providers, credentials, routes, relays, and policies.

### Administrator

Human identity with full permissions inside an installation. In the first phase it may coincide with the operator; the model allows adding more users and roles later.

### Client

Program that consumes ModelCairn's stable API, for example an agent, an application, or a development tool.

### Agent

Logical identity attributed to a client or set of requests. Allows applying policies, observing activity, and distributing concurrency without depending only on IP address.

## Infrastructure

### Installation

Self-hosted set administered by an operator. It does not depend on a central ModelCairn service and does not send telemetry to the project.

### Main node

Machine that runs the control plane and data plane: API, router, console, persistence, secrets, metrics, and estimator. It can also provide a direct egress. It is the only persistent custodian of API keys in the initial topology.

### Exit relay

Lightweight auxiliary service that provides an alternative egress. It transports authorized connections from the main node without deciding routes or persisting API keys, prompts, or responses. It is not a second complete installation.

### Egress

Path and network identity through which a connection reaches the provider. It may be the main node's direct egress or a relay's egress.

### Proxy

General term for a traffic intermediary. In ModelCairn, **exit relay** will be preferred when referring to the owned component, because "proxy" does not indicate whether it terminates TLS, makes decisions, or stores information.

### Control plane

Administration functions: web console, configuration, secrets, strategy publication, audit, and node state.

### Data plane

Path used by real requests: reception, client authentication, route selection, provider call, streaming, fallback, and response.

## Providers and access

### Provider

Authorized external service that offers models through an API. An account can have one or more credentials and different limits.

### Provider connection

Versioned configuration of a concrete endpoint: provider, base URL, adapter, network options, and declared capabilities. Two endpoints from the same provider are not automatically considered equivalent.

### Protocol adapter

Component that implements an input or output contract, for example OpenAI Chat Completions or Anthropic Messages. Translating between protocols requires an adapter; changing the model name does not perform that translation.

### Credential

Managed reference to a secret that authorizes calls to a provider. The interface may show its label, status, and permitted last characters, but never retrieve the complete value after it has been saved.

### API key

Frequent type of secret used as a credential. Not all future credentials have to be API keys; temporary tokens or other methods could exist.

### Egress affinity

Persistent assignment between a credential and an authorized egress. It does not change automatically. A manual modification must warn about its consequences and be recorded.

### Physical model

Real identifier understood by the provider, together with its declared or observed capabilities.

### Model alias

Stable name exposed to the client. It can point to one or more physical destinations according to a strategy, without claiming that different models are semantically identical.

### Capability

Function that a route can preserve, for example streaming, tools, vision, structured output, or specific parameters. An incompatible route must not be used as silent fallback if it degrades a required capability.

## Routing

### Request

Operation received from a client. It has an identity, a protocol, requirements, a total budget, and a final result.

### Destination

Eligible combination of provider connection, physical model, credential, and egress. It is the concrete unit to which the router can direct an attempt.

### Logical route

Stable entry point that links an alias with a strategy and its policies. The client uses the logical route without knowing each underlying destination.

### Strategy

Versioned graph or policy that decides which destinations may be attempted, in what order, under what conditions, and within what budgets.

### Attempt

A concrete call to a destination within a request. A request can produce several attempts when its strategy allows fallback.

### Fallback

Authorized transition from a failed or unavailable attempt to another destination. It does not mean retrying indefinitely or hiding incompatibilities.

### Execution budget

Maximum limits of time, attempts, and, where applicable, cost or tokens that a strategy can consume to resolve a request.

### Error policy

Rules that classify an outcome and decide whether to respond immediately, wait, open a cooldown, or allow fallback.

### Cooldown

Period during which a destination temporarily reduces or suspends its eligibility after a signal such as `429`. By itself it does not prove the official quota.

### Circuit breaker

Mechanism that temporarily stops sending traffic to a destination with repeated failures and performs controlled recovery probes.

## Estimation and observability

### Operational event

Structured record of something that happened, without storing prompts or responses by default. It may include timings, tokens, destination, status, decision, and redacted error.

### Metric

Numerical measurement derived from events, such as latency, TTFT, success rate, or tokens per second.

### Adaptive capacity and recovery estimator

Component that uses historical observations to estimate availability, capacity, and recovery. It expresses uncertainty and never presents an inference as an official quota.

### Confidence

Measure of how much evidence supports an estimate. It must consider quantity, recency, and consistency of observations.

### Health

Observed operational state of a node, relay, provider, or destination. Health is not equivalent to compatibility and does not guarantee future availability.

### Traceability

Ability to explain which strategy and signals produced a decision without revealing secrets or private content.

## Configuration and lifecycle

### Source of truth

Authoritative state from which ModelCairn operates. Files and the console must not maintain independent copies that can contradict each other.

### Declarative configuration

Versionable description of desired state. Its exact format and the way to reconcile web changes remain an architecture decision.

### Bootstrap

Minimum secure configuration needed to start the installation for the first time, including the administrative identity and access mode.

### Onboarding

Later flow that helps configure providers, credentials, models, and a first functional route.

### Migration

Versioned change of the data or configuration schema. It must be verifiable and have an explicit recovery or rollback strategy.

### Audit

Record of relevant administrative actions: who changed what, when, and with what result. It must not contain secret values.

## Essential relationships

1. An installation has exactly one main node in the initial topology.
2. An installation may register zero or more exit relays.
3. An egress belongs to the main node or to a relay.
4. A credential belongs to the main node's secret store and has affinity with an egress.
5. A provider has one or more versioned connections, offers physical models, and accepts certain credentials.
6. A destination combines connection, physical model, credential, and egress.
7. A logical route exposes an alias and references a versioned strategy.
8. A strategy selects compatible destinations through explicit policies.
9. A request produces one or more attempts and one final result.
10. Attempts generate events; metrics and estimates are derived from them.

## Initial invariant rules

- No relay persists secrets or request content.
- No API key appears complete in logs, metrics, audit, or exports.
- The router does not automatically change a credential's egress affinity.
- A fallback cannot exceed the request's total budget.
- A required capability is not silently removed during translation or fallback.
- Official rules configured by the operator prevail over observed estimates.
- Published decisions are versioned; editing a draft does not alter active traffic until publication.
