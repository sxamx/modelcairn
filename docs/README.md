# ModelCairn documentation

[Español](README.es.md)

## Current sources of truth

1. [Project status and lifecycle](project-lifecycle.md)
2. [Project Charter](project-charter.md)
3. [Glossary and domain model](glossary-and-domain-model.md)
4. [Baseline requirements](baseline-requirements.md)
5. [Target architecture](target-architecture.md)
6. [Phase 1 — Foundation](fases/fase-01-fundacion.md)
7. [Technical contracts](fases/fase-01-contratos-tecnicos.md)
8. [Implementation plan](fases/fase-01-plan-de-implementacion.md)
9. [Milestone 2 technical delivery plan](fases/hito-02-plan-tecnico.md)
10. [Milestone 3 technical delivery plan](fases/hito-03-plan-tecnico.md)
11. [Local data and retention map](data-retention.md)
12. [Milestone 4 technical delivery plan](fases/hito-04-plan-tecnico.md)
13. [Milestone 5 technical delivery plan](fases/hito-05-plan-tecnico.md)
14. [Milestone 6 technical delivery plan](fases/hito-06-plan-tecnico.md)
15. [Milestone 7 validation and closeout plan](fases/hito-07-plan-tecnico.md)
16. [Milestone 8 first release preparation plan](fases/hito-08-plan-tecnico.md)
16. [Phase 1 threat model](seguridad/phase-1-threat-model.md)
17. [Phase 1 operations runbook](operacion/phase-1-runbook.md)

## Phase 1 executable contracts

- [Configuration JSON Schema](contratos/config/modelcairn-config-v1alpha1.schema.json)
- [Apply semantics](contratos/config/semantica-apply-v1alpha1.md)
- [Offline CLI contract](contratos/cli-v1.md)
- [Online CLI contract](contratos/cli-online-v1.md)
- [Milestone 2 integration and resource evidence](evidencia/hito-02-integracion-recursos.md)
- [Milestone 3 administrative integration evidence](evidencia/hito-03-integracion-admin.md)
- [Milestone 3 representative benchmark](evidencia/benchmark-hito-03-2026-09-12.md)
- [Milestone 4 integrated router evidence](evidencia/hito-04-router-integrado.md)
- [Milestone 4 representative benchmark](evidencia/benchmark-hito-04-2026-09-13.md)
- [Milestone 5 console benchmark](evidencia/benchmark-hito-05-2026-09-13.md)
- [Milestone 5 grouped QA](evidencia/qa-agrupado-hito-05.md)
- [Milestone 6 installation and recovery evidence](evidencia/hito-06-installation-and-recovery.md)
- [Milestone 7 system and security validation evidence](evidencia/hito-07-system-validation.md)
- [Milestone 7 grouped QA](evidencia/qa-agrupado-hito-07.md)
- [Milestone 8 verified CI candidate evidence](evidencia/hito-08-ci-candidate.md)
- [Milestone 8 grouped QA](evidencia/qa-grouped-milestone-08.md)
- [First release checklist](operacion/first-release-checklist.md)
- [Example configuration](contratos/config/example-v1alpha1.yaml)
- [Administrative OpenAPI](contratos/api/admin-v1.openapi.yaml)
- [Data API OpenAPI](contratos/api/data-v1.openapi.yaml)
- [Chat Completions compatibility matrix](contratos/compatibilidad-chat-completions-v1.md)
- [Router failure matrix](contratos/matriz-fallos-router-v1.md)
- [Documentary SQLite schema](contratos/storage/schema-v1.sql)
- [SQLite model rules](contratos/storage/modelo-sqlite-v1.md)
- [SQLite ownership and keyring durability](contratos/storage/propiedad-y-llavero-v1.md)
- [Administrative sessions](contratos/sesiones-admin-v1.md)
- [MCB1 backup format](contratos/backup-mcb1.md)
- [Linux installation contract](contratos/instalacion-linux-v1.md)
- [Local, private network, and Tailscale access](operacion/acceso-red-v1.md)
- [Native Linux installation and update](operacion/linux-installation-v1.md)
- [MCB1 backup and recovery](operacion/backup-recovery-v1.md)
- [Requirements traceability](fases/fase-01-trazabilidad.md)
- [Executable acceptance manifest](fases/fase-01-aceptacion.json)
- [Risks and quality gates](fases/fase-01-riesgos-y-puertas.md)

## Milestone 3 — accepted contracts

- [Technical delivery plan](fases/hito-03-plan-tecnico.md)
- [Administrative runtime](contratos/admin-runtime-v1.md)
- [Settings and login audit](contratos/admin-settings-v1.md)
- [Individual resource mutations](contratos/resource-mutations-v1.md)
- [AgentToken lifecycle](contratos/agent-token-lifecycle-v1.md)

## Decisions

- [ADR-0001: product name](decisiones/0001-nombre-del-producto.md)
- [ADR-0002: license and attribution](decisiones/0002-licencia-y-atribucion.md)
- [ADR-0003: technical foundation and deployment](decisiones/0003-base-tecnica-y-despliegue.md)
- [ADR-0004: configuration, secrets, and recovery](decisiones/0004-configuracion-secretos-y-recuperacion.md)
- [ADR-0005: cryptography and backup](decisiones/0005-criptografia-y-formato-de-backup.md)
- [ADR-0006: web console stack](decisiones/0006-stack-de-consola-web.md)

## Languages and status

English is canonical and Spanish is an official translation. Both must be updated
together when a document's meaning changes. An old prototype document does not
override this index.

## Governance

- [Project management](governance/project-management.md)
- [Dependency policy and inventory](dependencies.md)
- [Resource benchmark procedure](benchmarks.md)
- [Milestone 1 representative benchmark evidence](evidencia/benchmark-hito-1-2026-09-06.md)

## Publication

- [Clean repository preparation](publicacion/preparacion-del-repositorio-limpio.md)
