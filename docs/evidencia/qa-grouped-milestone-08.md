# Milestone 8 grouped QA

[Español](qa-agrupado-hito-08.es.md)

- Date: September 20, 2026
- Scope: candidate chain, packages, recovery, documentation, and publication gate
- Technical verdict: **approved for maintainer decisions**
- Publication: **not authorized**

The independent review found no P0 issue. It identified that initial evidence did
not prove an actual restore, boot-enable preservation, or corruption/architecture
rejection. It also found a private IP in a test, incomplete CI/candidate parity,
incomplete SBOM coverage, and no promotion barrier.

All technical findings were corrected and revalidated. The IP was replaced with a
fictional address; the Linux harness restored MCB1 into an isolated generation and
preserved enabled state; bad-checksum, truncation, and wrong-architecture cases
failed; candidate and CI share static gates; checksum and attestation cover the
SBOM; and publication only promotes bytes from a successful private run for the
same `main` commit.

The ARM64 residual risk remains explicit: the package was built and inspected but
not executed on ARM64 hardware. Policy for irreversible migrations is likewise
deferred until such a migration exists.

The remaining items are maintainer choices, not open technical defects: version,
channel, public assets, identity/contact, and GitHub protections. No tag or GitHub
Release may exist before that approval.
