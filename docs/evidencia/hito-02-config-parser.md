# Milestone 2 — Configuration parsing and validation

[Español](hito-02-config-parser.es.md)

Status: independent QA approved; CI remains before acceptance.

`internal/config` turns exactly one YAML or JSON document into a single typed
model. Before canonicalization it rejects inputs larger than 8 MiB, more than 64
nested containers, additional documents, aliases, custom tags, duplicate keys,
and non-string YAML keys. Diagnostics contain only stable codes and paths and do
not copy rejected values.

Validation implements schema types and limits, defaults without losing field
omission, explicit `null` where allowed, HTTP(S) URLs, explicit permission for
literal private hosts, references, deletion against an effective catalog, and
provider affinity. JSON export orders resources by identity and applies the shared
redactor before strings are escaped.

Tests cover the example with all ten resource kinds, canonical round trips,
hostile inputs, URLs, tombstones, deletion against existing state, defaults and
presence, `null`, canaries, Unicode, and the 10,000-resource boundary. On the
998,465,536-byte representative VM, the Linux AMD64 suite passed with 51,152 KiB
maximum RSS and zero swap; the binary hash was verified after transfer.
Local `internal/config` coverage was 83.4%.

Limits: DNS resolution and protection against DNS/redirect changes belong to the
Milestone 4 egress client. Plan/apply integration belongs to the following
Milestone 2 items. The third independent review closed the earlier findings with
no blockers; CI remains before acceptance.
