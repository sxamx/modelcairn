# Project management

[Español](project-management.es.md)

ModelCairn uses an incremental, milestone-based workflow. Documentation is part of the product: architecture, contracts, decisions, risks, and acceptance evidence must evolve together with the implementation.

## Working model

- **Milestone:** a coherent product outcome with explicit entry and exit criteria.
- **Issue:** a bounded unit of work with an owner, acceptance criteria, and links to its governing documentation.
- **Pull request:** the reviewable integration unit. It should be small enough to understand and test independently.
- **ADR:** an Architecture Decision Record used when a decision has meaningful, lasting trade-offs.
- **Quality gate:** objective evidence required before a milestone or change is accepted.

## Board workflow

The recommended GitHub Projects columns are:

1. **Inbox** — captured but not yet refined.
2. **Ready** — scoped, prioritized, and unblocked.
3. **In progress** — actively being implemented; limit concurrent work.
4. **Review** — implementation complete and awaiting technical or QA review.
5. **Done** — acceptance criteria and quality gates satisfied.

Use labels for type (`type:feature`, `type:bug`, `type:docs`, `type:security`), priority (`priority:p0` through `priority:p3`), and area (`area:gateway`, `area:web`, `area:storage`, `area:installer`, `area:relay`).

## Definition of ready

An issue is ready when its goal, scope, acceptance criteria, dependencies, risks, and relevant contracts are clear. Unknown implementation details are acceptable; unknown product behavior is not.

## Definition of done

A change is done when:

- acceptance criteria are satisfied;
- relevant automated tests pass;
- security and privacy implications have been reviewed;
- user-facing and technical documentation are current;
- configuration changes are represented consistently in files and the web interface;
- no credentials or private environment details are present;
- an independent review is obtained when risk or scope justifies it.

## Decision ownership

The maintainer owns product decisions. Contributors and automation may propose alternatives and document trade-offs, but must not silently expand product scope. Decisions that materially affect compatibility, security, persistence, or deployment require an ADR.

## Release discipline

Until the first stable release, compatibility is governed by the published contracts and explicit versioning notes. Releases should include a changelog, migration guidance when applicable, test evidence, and known limitations.
