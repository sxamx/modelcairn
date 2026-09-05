# Contributing to ModelCairn

ModelCairn is documentation-first and currently entering Phase 1 implementation.
Before opening a large change, read the [project status](docs/project-lifecycle.md),
[charter](docs/project-charter.md), relevant ADRs, and the phase contract.

## Workflow

1. Discuss changes that alter scope, architecture, public contracts, security, or
   compatibility in an issue before implementing them.
2. Keep each pull request focused and link its requirement or decision.
3. Include tests and documentation with behavior changes.
4. Never commit provider keys, tokens, prompts, responses, local paths, databases,
   backups, or credentials.
5. Run the checks documented for the affected milestone.
6. Explain compatibility, migrations, resource impact, and rollback when relevant.

## Decisions

Use an ADR when a decision is expensive to reverse, affects several modules, or
changes a public contract. Small reversible implementation details belong in code
and tests, not an ADR.

## Commits and pull requests

Commits should represent coherent, reviewable units. Artificial commit splitting
is discouraged. Pull requests must not silently weaken an approved MUST
requirement; propose the contract change explicitly instead.

## License of contributions

Unless explicitly stated otherwise, contributions intentionally submitted for
inclusion are provided under Apache License 2.0, as described in section 5 of the
license. Do not submit material you do not have the right to contribute.
