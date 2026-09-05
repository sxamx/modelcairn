# Matriz de disposición del prototipo

[English](matriz-de-disposicion-del-prototipo.md)

- Estado: decisión de saneamiento previa al repositorio limpio
- Regla: no copiar código o documentación anterior por defecto

| Material anterior | Disposición | Motivo |
|---|---|---|
| `src/`, `public/`, scripts y tests | referencia local; reevaluar por hito | pertenecen a otra arquitectura/nombre y no prueban contratos nuevos |
| `docs/architecture.es.md` | reemplazado | contradice custodia central y relays actuales |
| `docs/control-plane.es.md` | referencia parcial | puede aportar casos, no contrato vigente |
| `docs/deployment.es.md`, `docs/install.es.md` | reemplazar durante Hito 6 | describen despliegue del prototipo |
| `docs/profiles.es.md` | referencia parcial | algunas ideas de versionado pueden migrarse con pruebas |
| `docs/strategy-builder.es.md` | diferido | útil para la fase del editor visual, no Fase 1 |
| `docs/web-console.es.md`, capturas y QA UI | referencia visual no vinculante | la consola se rediseñará contra ADR-0006 |
| `docs/roadmap.es.md`, `docs/roadmap-visual.es.md` | reemplazados | contienen fases y afirmaciones antiguas de Aegis |
| bases SQLite, previews y conversación recuperada | privado; excluir | pueden contener datos locales y no son fuente pública |

Antes de reutilizar un módulo, el hito responsable debe demostrar compatibilidad,
licencia, pruebas y coste de adaptación. Reescribir no es obligatorio; reutilizar
tampoco es automático.
