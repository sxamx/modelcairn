# ADR-0002: License, attribution, and trademark

[Español](0002-licencia-y-atribucion.es.md)

- Status: accepted for design and initial publication
- Date: 2026-09-05
- Decision: publish ModelCairn under the Apache License 2.0

## Context

ModelCairn is intended to be an open source project that permits use,
modification, redistribution, integration, and service delivery. The maintainer
wants to encourage adoption and contributions, preserve recognition of the
project's origin, and permit voluntary sponsorship.

Four concepts were distinguished and must not be confused:

- **copyright:** recognizes authorship and rights in the original work;
- **license:** grants permissions and establishes conditions for using the work;
- **trademark:** identifies the official project and prevents misleading
  presentations;
- **sponsorship:** voluntary financial support, independent of the license.

## Decision

The initial publication will use the **Apache License 2.0**, with SPDX identifier
`Apache-2.0`. Repository files will be under that license unless a file or
directory explicitly states another condition. Treatment of logos and other brand
assets will be defined separately before they are published.

The distribution will include:

- `LICENSE` with the complete, unmodified Apache-2.0 text;
- `NOTICE` with brief project attribution and the location of the official
  repository;
- SPDX notices in files when required by the contribution policy;
- an “About” view with version, license, repository, original creator, and
  contributors;
- discreet attribution in the official interface;
- a separate trademark policy before the first stable release.

Working text for `NOTICE`:

```text
ModelCairn
Copyright 2026 sxamx

Originally developed by sxamx.
Official repository: https://github.com/sxamx/modelcairn
```

The creator's initial public identity will be `sxamx`. It may be supplemented
with a personal name or legal entity only through an explicit decision.

## Scope of attribution

Apache-2.0 does not require every project to create a `NOTICE` file. ModelCairn
chooses to include one; therefore, distributors of derivative works must legibly
reproduce its applicable attributions as provided by section 4 of the license. It
does not require the creator's name to be permanently displayed in every fork's
interface.

Visible attribution in the footer and “About” view is part of ModelCairn's official
design. Forks may change the interface within the license permissions, but must
meet the license and attribution obligations in their distribution.

The future trademark policy must clarify which uses of the name and logo are
permitted. The code license alone does not grant permission to present a modified
version as an official project release.

## Sponsorship

Sponsorship is neither mandatory nor a condition of use. The project may include a
`.github/FUNDING.yml` file and voluntary links from GitHub and the “About” view.
No sponsor acquires ownership or control of the project merely by contributing
funds.

## Future license changes

A published version retains the permissions of the license under which it was
distributed. Changing the license for future versions requires control of the
necessary rights in all affected code.

Once external contributions exist, a change might require authorization from their
rights holders, an applicable contribution agreement, or replacement of
contributions that cannot be relicensed. CPAL will therefore not be adopted as a
temporary step with the expectation of switching automatically to Apache later.

The contribution policy must define the inbound licensing model before significant
external code is accepted.

## Alternatives considered

- **CPAL-1.0:** permits limited attribution requirements in an interface and covers
  network deployments. It was rejected because it is less familiar and creates
  more adoption friction than the final objective requires.
- **MIT:** simple and widely adopted, but Apache-2.0 provides more explicit patent
  and redistribution terms.
- **AGPL-3.0:** would require publishing modifications used to provide a network
  service. It was rejected because the maintainer prioritizes adoption and does not
  require every service-operated modification to be published.

## Consequences

- Third parties may create and distribute forks, including commercially, if they
  comply with Apache-2.0.
- A fork may change the interface and is not required by Apache-2.0 to retain
  permanent on-screen credit.
- Applicable notices must be retained as required by the license.
- The name and logo will need their own policy.
- The initial publication contains `LICENSE` and `NOTICE`; both were added while
  preparing the clean repository.

## Review

This decision must be reviewed before accepting external contributions if the
adoption objective, governance model, or need for copyleft changes. Professional
advice should be obtained for a specific legal application.
