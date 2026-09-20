# ModelCairn

> Status: Phase 1 accepted; Milestones 0 through 8 complete; v0.1.0 published
> as a prerelease.
> The recovered prototype is kept outside this repository as a reference.

[Español](README.es.md)

ModelCairn is a lightweight, self-hosted, configurable AI provider gateway. It
offers a stable endpoint, explainable routing and fallback, and local operational
metrics. Adaptive capacity and recovery estimation is planned, not implemented
in this prerelease. The operator retains control over
their data, credentials, providers, and egress paths; ModelCairn sends no
telemetry to a central service.

The name refers to a *cairn*: each observation acts as a stone that improves the
route signal without pretending that an estimate is an official rule.

## Project status

The [v0.1.0 prerelease](https://github.com/sxamx/modelcairn/releases/tag/v0.1.0)
is published and verified. Phase 1 is implemented, validated, and accepted; this
first version must not be mistaken for a stable release. See:

- [Project status and lifecycle](docs/project-lifecycle.md)
- [Project Charter](docs/project-charter.md)
- [Documentation index](docs/README.md)
- [Phase 1 — Operable foundation](docs/fases/fase-01-fundacion.md)
- [Milestone 8 — First release preparation](docs/fases/hito-08-plan-tecnico.md)

## Approved technical direction

- Go backend as a modular monolith.
- TypeScript/React/Vite console embedded as static assets.
- SQLite as the source of truth and YAML for validate/plan/apply/export.
- Linux executable managed by systemd; AMD64 and ARM64.
- Initial `POST /v1/chat/completions` API, including streaming and tool calls
  within an explicit matrix.
- API keys centralized and encrypted on the primary node.
- Prompts and responses are not stored by default.
- Lightweight egress relays in a later phase, without persisting API keys.

## License

ModelCairn is distributed under the [Apache License 2.0](LICENSE) and includes a
[NOTICE](NOTICE) with its original attribution. Modified distributions must also
follow the [name and marks guidance](TRADEMARKS.md) and must not imply official
status.

Notable changes are tracked in [CHANGELOG.md](CHANGELOG.md).

## Development

Milestones 0 through 8 are complete. With Go installed:

```sh
go test ./...
go run ./cmd/modelcairn serve
```

The offline configuration workflow is also available during development:

```sh
go run ./cmd/modelcairn config validate config.yaml
go run ./cmd/modelcairn config plan --data-dir ./data --out plan.json config.yaml
go run ./cmd/modelcairn config apply --data-dir ./data --plan plan.json config.yaml
```

See the [offline CLI contract](docs/contratos/cli-v1.md) for configuration,
secret-management, safety, and exit-code behavior.

The development server listens on `127.0.0.1:8080` by default. `/healthz`
reports process liveness; `/readyz` remains unavailable until persistence,
configuration, and the secret store are initialized in their milestones.
