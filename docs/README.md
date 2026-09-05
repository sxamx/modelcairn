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

## Phase 1 executable contracts

- [Configuration JSON Schema](contratos/config/modelcairn-config-v1alpha1.schema.json)
- [Apply semantics](contratos/config/semantica-apply-v1alpha1.md)
- [Example configuration](contratos/config/example-v1alpha1.yaml)
- [Administrative OpenAPI](contratos/api/admin-v1.openapi.yaml)
- [Chat Completions compatibility matrix](contratos/compatibilidad-chat-completions-v1.md)
- [Documentary SQLite schema](contratos/storage/schema-v1.sql)
- [SQLite model rules](contratos/storage/modelo-sqlite-v1.md)
- [Administrative sessions](contratos/sesiones-admin-v1.md)
- [MCB1 backup format](contratos/backup-mcb1.md)
- [Requirements traceability](fases/fase-01-trazabilidad.md)
- [Risks and quality gates](fases/fase-01-riesgos-y-puertas.md)

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

## Publication

- [Clean repository preparation](publicacion/preparacion-del-repositorio-limpio.md)
