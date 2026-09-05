# Fase 1 — Trazabilidad de requisitos

[English](fase-01-trazabilidad.md)

| Requisito | Contrato | Hito | Evidencia de cierre |
|---|---|---:|---|
| RF-001, RF-008 | compatibilidad y streaming | 4 | suite HTTP/SSE normal, tools, corte y cancelación |
| RF-002 | sesiones y AgentToken | 3 | login, separación, revocación y expiración |
| RF-003 | JSON Schema, apply y OpenAPI | 2–5 | CRUD/CLI/web end-to-end |
| RF-004 | ADR-0005 y schema SQLite | 2 | cifrado, redacción y fallo de clave |
| RF-005 | Credential.egressRef | 2, 4 | persistencia y ausencia de reasignación automática |
| RF-006, RF-007 | tabla de errores y estrategia | 4 | matriz de fallos y agotamiento de presupuestos |
| RF-009 | requests/attempts/observations | 4 | inspección que demuestra ausencia de contenido |
| RF-010 | apply/OpenAPI/ADR-0006 | 2, 5 | equivalencia funcional CLI y web |
| RF-011 | health/readiness | 3, 6 | dependencias sanas y degradadas |
| RF-012 | MCB1 y migraciones | 2, 6 | fallo interrumpido y restauración funcional |
| RF-013 | ProviderConnection | 4 | suite SSRF, DNS/redirect y excepción privada |
| RNF-001–003 | contrato de benchmark/router | 1, 4, 7 | informe reproducible, límites y ausencia de loops |
| RNF-004 | audit_events | 2–7 | cobertura de mutaciones críticas |
| RNF-005 | instalación/backup | 6 | VM limpia, actualización manual y restore |
| RNF-006 | matriz de compatibilidad | 4 | capacidades declaradas/rechazadas |
| RNF-007 | ADR-0006 y onboarding | 5 | pruebas de usabilidad y accesibilidad básica |
| RNF-008 | arquitectura y red | 1–7 | prueba sin destinos externos de telemetría |

Cada hito actualiza esta tabla con enlaces a pruebas reales. Una fila sin evidencia
impide cerrar la Fase 1 aunque la interfaz parezca terminada.
