# ModelCairn

> Status: Phase 1 incremental implementation; Milestone 2 complete.
> The recovered prototype is kept outside this repository as a reference.

[Español](README.es.md)

ModelCairn is a lightweight, self-hosted, configurable AI provider gateway. It
offers a stable endpoint, explainable routing and fallback, local metrics, and
adaptive capacity and recovery estimation. The operator retains control over
their data, credentials, providers, and egress paths; ModelCairn sends no
telemetry to a central service.

The name refers to a *cairn*: each observation acts as a stone that improves the
route signal without pretending that an estimate is an official rule.

## Project status

There is no release yet. The documentation baseline and executable contracts for
Phase 1 are complete; incremental development in Go follows. See:

- [Project status and lifecycle](docs/project-lifecycle.md)
- [Project Charter](docs/project-charter.md)
- [Documentation index](docs/README.md)
- [Phase 1 — Operable foundation](docs/fases/fase-01-fundacion.md)

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
[NOTICE](NOTICE) with its original attribution.

## Development

Milestones 1 and 2 are complete; Milestone 3 is next. With Go installed:

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
