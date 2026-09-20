# Security Policy

ModelCairn has not published a release yet. Until the first prerelease is verified,
security fixes are applied only to `main`.

| Version | Supported |
|---|---|
| `main` | yes, development branch |
| published versions | none yet |

After publication, this table—not an assumed SemVer range—defines support. The
project may replace an unsafe prerelease instead of backporting fixes.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting or a private security advisory
for this repository. Do not publish API keys, exploit details, prompts, responses,
backups, database files, internal URLs, or personal information in a public issue.

Include the affected revision, impact, minimal reproduction, and any suggested
mitigation. Reports will be acknowledged when the maintainer is available; this
open-source project does not currently promise a fixed response SLA.

## Scope priorities

High-priority reports include API-key disclosure, authentication or authorization
bypass, cross-provider credential routing, unsafe provider URL access, backup
decryption or restore flaws, and relay use as an unauthorized open proxy.

For ordinary bugs without security impact, use a public issue after checking that
logs and screenshots are redacted.
