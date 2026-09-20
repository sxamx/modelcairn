# Milestone 8 — Verified CI candidate evidence

[Español](hito-08-candidata-ci.es.md)

- Date: September 20, 2026
- `main` revision: `d6565bb`
- Technical version identifier: `0.0.0-ci`
- Result: workflow and AMD64 package consumption passed

`0.0.0-ci` is deliberately a technical identifier, not a proposed public version.
No tag or GitHub Release was created.

## Executed chain

The manual workflow proved the revision belongs to `main`, rebuilt and tested the
frontend/backend, validated documentation and egress, generated and rebuilt two
Linux tarballs, produced an SPDX 2.3 SBOM, issued provenance, and uploaded a private
artifact with 14-day retention. Its permissions did not include `contents: write`.

| Artifact | Size |
|---|---:|
| Linux AMD64 | 5,871,620 bytes |
| Linux ARM64 | 5,410,941 bytes |
| SPDX JSON SBOM | 191,697 bytes |
| `SHA256SUMS` | 306 bytes |

The SBOM is covered by `SHA256SUMS` and has its own attestation. Both archives
passed checksum,
allowlist, traversal, `0755`/`0644` mode, manifest, architecture, and Go metadata
checks against the exact commit.

## Independent artifact consumption

The Actions zip was downloaded and inspected outside its producing workflow. The
scan found no VM IP, remote user, personal path, token prefix, or credential/provider
canaries used by tests. Only the AMD64 tarball was then copied to a temporary
directory on the representative VM.

Linux rechecked SHA-256, extracted the package, required mode 0755 on the binary,
and ran the integrated gate with a real server, bootstrap, configuration,
AgentToken rotation, rejection of the former bearer, a normal call, and SSE. It
exited zero with 56,604 KiB peak RSS and did not touch the persistent installation
or data. The temporary environment was removed afterwards.

A tightly gated destructive check also ran on the representative host. Before
acting, it rejected any installed binary, unit, or active service, took a local
snapshot of the inactive state, and armed mandatory trap-based restoration. It
installed the old package, created and verified an encrypted MCB1 backup with mode
0600, upgraded to the new package, and checked readiness and admin identity. It
then rolled back to the old package, repeated those checks, uninstalled the
temporary copy, and restored the snapshot. The preserved state recovered its
owners and modes, with no binary, unit, or active service left behind.

The follow-up review added an actual MCB1 restore into an isolated generation,
boot-enable checks through install/upgrade/rollback, and negative cases for a bad
checksum, truncated archive, and wrong architecture. All passed. Independent QA
left no technical blockers.

## Corrected findings

1. The first Windows-built local tarball did not retain executable mode. Packaging
   now fixes modes inside the archive and verification requires them.
2. Windows and Linux emitted different standard `SHA256SUMS` variants. Generation
   now requests binary format and verification accepts either standard form.
3. `upload-artifact` v4 and v5 warned about Node 20 being forced to Node 24. The
   official v7.0.1 SHA is now pinned, whose action declares `node24`.

## Remaining limits

- ARM64 was built and inspected but not executed on ARM64 hardware.
- The maintainer's version, channel, and public-data decisions remain pending.
- The validated upgrade did not include an irreversible schema migration; that
  policy will be defined before a release that needs one.
- This evidence does not authorize publication.
