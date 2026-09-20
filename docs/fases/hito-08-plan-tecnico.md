# Milestone 8 — First open-source release preparation

[Español](hito-08-plan-tecnico.es.md)

- Status: **completed; v0.1.0 published and verified as a prerelease**
- Stage: G — open-source preparation and publication
- Objective: turn accepted Phase 1 into an installable, verifiable, recoverable
  candidate without presenting it as a stable version.

This milestone prepares and tests the release chain. It does not implement relays,
the adaptive estimator, additional protocols, or the visual editor. It also creates
no tag or GitHub Release until the maintainer approves the version and channel.

## Maintainer-approved decisions

On September 20, 2026, the maintainer approved:

1. `v0.1.0` as a prerelease, with no stability promise.
2. Linux AMD64/ARM64 archives, `SHA256SUMS`, SPDX SBOM, attestations, and
   GitHub's automatic source archives; no container.
3. `sxamx` as the public author/maintainer identity.
4. GitHub Private Vulnerability Reporting as the security contact, with no
   personal email published.
5. Protection for `main`, `v*` tags, and a manually approved `release`
   environment. These protections were activated before producing the candidate.

Final authorization was granted after these decisions were approved. The tag
and GitHub Release were created through the protected gate and the public assets
were verified.

Other implementation decisions are reversible and fixed by this plan.

## 1. Artifact contract

Each candidate is built from a green `main` commit. Publication creates the tag
only after validating the same bytes. Planned artifacts are:

- `modelcairn_<version>_linux_amd64.tar.gz`;
- `modelcairn_<version>_linux_arm64.tar.gz`;
- `SHA256SUMS`;
- an SPDX JSON SBOM per artifact or one SBOM identifying both binaries;
- GitHub Actions provenance attestation when available;
- release notes covering changes, compatibility, installation, upgrade, rollback,
  limitations, and evidence.

Each archive contains the binary, `LICENSE`, `NOTICE`, README, and a version
manifest. Binaries embed version, commit, and UTC date through `buildinfo`;
`modelcairn version` must match the tag and manifest.

## 2. Candidate workflow

A manual, reusable workflow will accept a version without publishing it:

1. validate SemVer and prove the commit belongs to `main`;
2. run the same CI gates, not only a build;
3. generate the frontend from its lockfile and require a clean tree;
4. compile with `CGO_ENABLED=0`, `-trimpath`, and version metadata;
5. package deterministic AMD64 and ARM64 archives;
6. generate checksums, SBOM, and provenance;
7. test every archive in a temporary Linux installation;
8. upload Actions artifacts, never a public release automatically.

A separate manual workflow publishes exactly the private artifacts from an
approved candidate. It requires the protected `release` environment, textual
confirmation, a successful run for the same `main` commit, valid checksums, and
valid attestations. Publication neither rebuilds nor accepts operator-supplied
files.

## 3. Package installation, upgrade, and rollback

Representative validation covers:

- clean installation from the architecture-correct tarball;
- bootstrap and normal/SSE route with temporary credentials;
- verified MCB1 backup before upgrade;
- upgrade from the latest supported candidate while preserving data and modes;
- startup, readiness, and functional route after upgrade;
- binary rollback and, for compatible migrations, reopening existing data;
- safe failure for wrong architecture, checksum, truncation, or bad metadata;
- conservative uninstall without deleting `/var/lib/modelcairn`.

Until a prior candidate exists, the first run tests idempotent reinstallation of
the same package and rollback to the previous binary from the same revision.

## 4. Public documentation

- `CHANGELOG.md` adopts Keep a Changelog and SemVer, starting with `Unreleased`.
- Installation docs use explicit downloads and SHA-256 verification; no `curl | sh`.
- `SECURITY.md` distinguishes supported branches/versions without promising an SLA.
- `CONTRIBUTING.md` stops claiming Phase 1 is under validation.
- Notes list real limits: no relays, adaptive estimator, Anthropic/Google protocols,
  or visual editor.
- A trademark policy exists before calling any release stable; Apache-2.0 and
  NOTICE do not grant a right to present a fork as an official release.

## 5. Supply-chain security

- minimum permissions and actions pinned by SHA;
- protected GitHub environment publication without a persistent PAT;
- OIDC for attestation; no provider secret in builds or tests;
- checksum verification before install and no canaries in artifacts/logs;
- current dependency inventory and `govulncheck` in final review;
- history and packaged-file scan for private paths, hosts, and identities.

Additional keyless Sigstore signing is recommended if integration remains
secretless; otherwise GitHub checksums and provenance are the minimum for this
first prerelease.

## 6. Acceptance and execution order

1. Fix the contract, checklist, and pending decisions.
2. Implement reproducible packaging and local tests.
3. Add a no-publish candidate workflow.
4. Run the candidate on AMD64 and cross-build ARM64.
5. Test install/upgrade/rollback on the representative VM.
6. Perform one grouped independent QA review.
7. Correct blockers and produce the readiness report.
8. Request explicit version/channel approval.
9. Only then create the tag and GitHub Release; verify published assets from a
   clean machine.

The milestone may declare **candidate ready** before step 8, but it completes only
when the authorized publication is verified or the maintainer closes it without
publishing. A release is never created merely to test the workflow.
