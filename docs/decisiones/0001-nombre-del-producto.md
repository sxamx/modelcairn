# ADR-0001: Product name

[Español](0001-nombre-del-producto.es.md)

- Status: accepted for design and development
- Date: 2026-09-04
- Decision: adopt **ModelCairn** as the product name

## Context

The previous prototype was named Aegis Gateway before the product vision was
defined. That name overlaps with other AI-related gateways and security projects,
making the project harder to distinguish in searches, documentation, and technical
conversations.

The product needs a name that can represent the complete gateway, not only fallback
or quota learning. It must support a clear identity for the dashboard, node mesh,
and adaptive estimator.

## Decision

The product will be called **ModelCairn** during design and development.

A *cairn* is a pile of stones placed to mark a route. The metaphor describes the
system's core behavior: each observation adds evidence about capacity, failures,
and recovery; that evidence helps choose a viable route according to the
operator's policies.

Descriptive name:

> ModelCairn — Adaptive AI Gateway

Initial message:

> Adaptive routing for the AI providers you control.

Short description:

> ModelCairn is a lightweight, self-hosted AI gateway that observes provider
> capacity and recovery patterns, then routes requests through paths defined by
> the operator.

These texts are a working basis. The vision document will define the exact promise
before they are used in public communications.

## Provisional functional names

- **Gateway:** data plane, API compatibility, and routing.
- **Studio:** web console and visual strategy builder.
- **Mesh:** communication and operation among owned nodes.
- **Capacity Estimator:** adaptive capacity and recovery estimator.

These terms describe conceptual boundaries and are not sub-brands. They also do
not require separate processes, packages, or repositories.

## Preliminary check

An exact-name search was performed on September 4, 2026:

- GitHub: no repository with an exact name match.
- npm: no exact published package.
- PyPI: no exact published project.
- crates.io: no exact published crate.
- General web search: no relevant technology product with the exact name was
  found.

This check reduces the risk of technical confusion, but is not a legal trademark
search and does not guarantee domains or social-media handles.

An independent review found that the `Cairn` root has many uses in software and
AI. It also noted that `Model` may narrow perception of the product to models,
although the system manages providers, nodes, routes, and capacity.

The objection is accepted as a known risk but does not block the decision. The
exact compound had no collisions in the technical registries reviewed, the
metaphor represents the core behavior, and the name works for an international
technical audience. Giving partial occupation of the root more weight than the
exact compound would push the brand toward artificial terms with less clarity.
The public description must explain that ModelCairn routes traffic across providers
and infrastructure, rather than managing models alone.

## Consequences

- New documentation will use ModelCairn.
- The previous prototype may retain historical references to Aegis Gateway; these
  will not be migrated automatically.
- Before the first public commit, included documents must be reviewed for
  consistent use of the agreed name.
- Before the first release, trademarks, domains, package registries, and relevant
  handles must be reviewed for the selected countries and channels.
- The remote repository must change its temporary slug to `modelcairn` before
  the first public commit.

## Alternatives considered

- **Aegis Gateway:** rejected because of direct collisions with similar projects.
- **Tidepath:** rejected because it is actively used by an automation and AI
  company.
- **SteadyRelay:** too generic and too close to electronics terminology.
- **CapacityWeave:** narrows perception to capacity and is conceptually close to
  other infrastructure brands.
- **AvailRoute:** precise, but difficult to turn into a memorable identity.
- **Navifold:** a good distribution metaphor, rejected after finding a recent exact
  use in a chemical-mapping tool.
- **RouteBraid:** a clear metaphor for braided routes, but more descriptive and
  with the `Braid` root widely used in software.
- **Trenzavia:** distinctive and expressive in Spanish; retained as a fallback,
  with less immediate clarity for an international audience.

## Review

This decision must be reopened if a material collision appears before the first
release or if the approved vision shows that the metaphor no longer represents
the product.
