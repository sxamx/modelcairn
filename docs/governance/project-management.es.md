# Gestión del proyecto

[English](project-management.md)

ModelCairn usa un flujo incremental basado en hitos. La documentación forma parte del producto: arquitectura, contratos, decisiones, riesgos y evidencia de aceptación deben evolucionar junto con la implementación.

## Modelo de trabajo

- **Hito:** resultado coherente del producto con criterios explícitos de entrada y salida.
- **Issue:** unidad acotada de trabajo con responsable, criterios de aceptación y enlaces a la documentación que la gobierna.
- **Pull request:** unidad revisable de integración. Debe ser suficientemente pequeña para comprenderla y probarla de forma independiente.
- **ADR:** registro de decisión de arquitectura, usado cuando una decisión tiene consecuencias y compromisos duraderos.
- **Puerta de calidad:** evidencia objetiva exigida antes de aceptar un hito o cambio.

## Flujo del tablero

Las columnas recomendadas para GitHub Projects son:

1. **Inbox** — capturado, pero todavía sin refinar.
2. **Ready** — acotado, priorizado y sin bloqueos.
3. **In progress** — en implementación activa; se debe limitar el trabajo simultáneo.
4. **Review** — implementación terminada y pendiente de revisión técnica o QA.
5. **Done** — criterios de aceptación y puertas de calidad satisfechos.

Se usan etiquetas de tipo (`type:feature`, `type:bug`, `type:docs`, `type:security`), prioridad (`priority:p0` a `priority:p3`) y área (`area:gateway`, `area:web`, `area:storage`, `area:installer`, `area:relay`).

## Definición de preparado

Un issue está preparado cuando su objetivo, alcance, criterios de aceptación, dependencias, riesgos y contratos relevantes están claros. Puede haber incógnitas de implementación; no debe haber incógnitas sobre el comportamiento esperado del producto.

## Definición de terminado

Un cambio está terminado cuando:

- satisface sus criterios de aceptación;
- pasan las pruebas automatizadas relevantes;
- se revisaron sus implicaciones de seguridad y privacidad;
- la documentación técnica y para usuarios está actualizada;
- los cambios de configuración aparecen de forma coherente en archivos y en la interfaz web;
- no contiene credenciales ni detalles privados del entorno;
- cuenta con revisión independiente cuando el riesgo o alcance lo justifica.

## Responsabilidad sobre decisiones

El mantenedor decide el producto. Los contribuidores y la automatización pueden proponer alternativas y documentar compromisos, pero no deben ampliar silenciosamente el alcance. Las decisiones que afecten materialmente compatibilidad, seguridad, persistencia o despliegue requieren un ADR.

## Disciplina de versiones

Hasta la primera versión estable, la compatibilidad se rige por los contratos publicados y las notas explícitas de versionado. Cada versión debe incluir registro de cambios, instrucciones de migración cuando correspondan, evidencia de pruebas y limitaciones conocidas.
