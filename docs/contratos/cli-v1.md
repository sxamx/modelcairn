# Offline CLI contract v1

[Español](cli-v1.es.md)

The offline CLI exclusively owns the installation data directory for every
stateful command. It cannot run concurrently with `serve` or another offline
owner. `config validate` is the exception: it parses a bounded file without
opening, creating, or migrating an installation.

## Configuration workflow

```text
modelcairn config validate <file>
modelcairn config plan [--data-dir path] [--allow-delete] [--out plan.json] <file>
modelcairn config apply [--data-dir path] [--allow-delete] [--plan plan.json] <file>
modelcairn config export [--data-dir path]
```

`plan --out` creates a new file and refuses to overwrite an existing path. POSIX
systems request mode `0600`; on Windows, effective privacy depends on the parent
directory's inherited ACL and users must choose a private directory. Emission and
reading reject plans larger than 32 MiB, so every emitted plan fits the consumption
limit. The JSON contains a ten-minute authenticated token, sorted changes, and a
redacted canonical configuration. Without `--out`, the same JSON is written to
stdout. A plan is installation-specific, single-use, and bound to the exact input,
deletion permission, key version, revision, and observed resources.

Non-interactive apply requires `--plan`. It parses the configuration and plan
within their byte limits before opening the installation, then verifies and
consumes the token in the same transaction as the mutations. Interactive apply
creates and displays a fresh plan and accepts only `y` or `yes`; the installation
lock remains held through confirmation and apply. Any other answer cancels with
no configuration mutation. Export writes deterministic JSON and never secret
values.

## Secret workflow

```text
modelcairn secret set [--data-dir path] [--version n] <name>
modelcairn secret metadata [--data-dir path] [name]
modelcairn secret rotate [--data-dir path]
modelcairn secret delete --version n [--data-dir path] <name>
```

`secret set` reads the value from stdin. A real interactive terminal uses no-echo
input; piped input may end in one CRLF/LF terminator, which is removed. Values are
valid UTF-8 between 8 and 16,384 bytes. Version zero creates; replacing requires
the current positive `--version`. Metadata returns only name, fingerprint,
resource version, key version, and timestamps. Delete requires an exact version
and fails while a Credential references the secret. Rotate never prints key
material.

## Stable exit classes

| Code | Meaning |
|---:|---|
| 0 | Completed or explicitly cancelled interactive apply |
| 1 | Operating-system, storage, integrity, or unexpected runtime failure |
| 2 | Usage, bounded-input, secret-input, or configuration diagnostic |
| 3 | Persisted-state conflict: stale version, already existing, missing, or in use |
| 4 | Invalid, expired, reused, or mismatched authenticated plan |

Diagnostics go to stderr. Configuration diagnostics contain a stable code and
structural path. Outputs and errors pass only typed metadata and never echo input
secret values.
