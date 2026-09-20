# Milestone 8 — Initial CI candidate evidence

[Español](hito-08-candidata-ci.es.md)

- Date: September 19, 2026
- `main` revision: `1e3c0ca`
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
| Linux AMD64 | 5,871,658 bytes |
| Linux ARM64 | 5,410,495 bytes |
| SPDX JSON SBOM | 191,969 bytes |
| `SHA256SUMS` | 210 bytes |

The SBOM declared 93 packages and eight files. Both archives passed checksum,
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

## Corrected findings

1. The first Windows-built local tarball did not retain executable mode. Packaging
   now fixes modes inside the archive and verification requires them.
2. Windows and Linux emitted different standard `SHA256SUMS` variants. Generation
   now requests binary format and verification accepts either standard form.
3. `upload-artifact` v4 warned about Node 20 being forced to Node 24; it was updated
   to the official v5 SHA and awaits confirmation in the next run.

## Remaining limits

- ARM64 was built and inspected but not executed on ARM64 hardware.
- Package-level upgrade and rollback still need an isolated installation test.
- Final grouped QA and the maintainer's version/channel decision remain pending.
- This evidence does not authorize publication.
