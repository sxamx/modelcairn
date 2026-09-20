# First release checklist

[Español](checklist-primera-release.es.md)

This list is an operational gate. Checking an item requires linked evidence; one
successful command is insufficient.

## Candidate

- [ ] Version and channel approved by the maintainer.
- [x] Clean `main` commit with the complete CI green.
- [x] AMD64/ARM64 binaries identify the correct version, commit, and date.
- [x] Tarballs contain only allowlisted files with expected modes.
- [x] `SHA256SUMS`, SBOM, and provenance match candidate bytes.
- [x] Secret/private-identifier and vulnerability scans passed.

## Operations

- [x] Clean tarball installation passed.
- [x] Bootstrap, login, normal route, and SSE passed.
- [x] Pre-upgrade backup created and verified.
- [x] Upgrade preserves data, modes, boot startup, and readiness.
- [x] Functional rollback and restorable backup verified.
- [x] Wrong checksum, truncated archive, and wrong architecture fail without
      changing the active installation.

## Documentation and review

- [x] README, changelog, install, upgrade, rollback, and limitations match the
      candidate.
- [x] License, NOTICE, contribution, security, and trademark policy are present.
- [x] Notes link evidence and claim neither stability nor deferred features.
- [x] Independent grouped QA has no blockers.

## Authorized publication

- [ ] Explicit maintainer approval recorded.
- [ ] Tag targets the approved commit and activates the protected environment.
- [ ] GitHub Release contains exactly the candidate hashes.
- [ ] Clean download, checksum, installation, and `modelcairn version` verified.
- [ ] Board, lifecycle, and documentation updated after—not before—public assets
      are verified.
