# Changelog

All notable changes to ModelCairn will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Lightweight self-hosted gateway with OpenAI Chat Completions compatibility.
- Bounded sequential fallback for normal and SSE requests.
- Encrypted provider credentials and revocable/rotatable agent tokens.
- Transactional configuration through file, CLI, API, and embedded web console.
- Local operational metrics without prompt/response persistence or external
  telemetry.
- Native systemd installation, encrypted backup, restore, and rollback workflows.

### Security

- Administrative session, CSRF, login backoff, SSRF, redaction, and canary gates.
- Provider-affinity constraints and explicit private-network exceptions.

### Known limitations

- No egress relays or multinode proxy protocol yet.
- No adaptive capacity/recovery estimator yet.
- No Anthropic or Google-compatible protocol translation yet.
- No visual routing/fallback editor yet.

[Unreleased]: https://github.com/sxamx/modelcairn/commits/main
