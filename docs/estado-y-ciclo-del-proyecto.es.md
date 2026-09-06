# Estado y ciclo del proyecto

Este documento registra en qué etapa se encuentra ModelCairn, qué evidencia permitió cerrarla y qué condiciones deben cumplirse antes de avanzar. No sustituye al roadmap funcional: gobierna el proceso utilizado para definir, construir y publicar el producto.

## Regla de avance

Una etapa se cierra cuando sus entregables están documentados, sus dudas críticas están resueltas y una revisión independiente no mantiene hallazgos bloqueantes. Es bloqueante cualquier hallazgo crítico o alto sin resolver, así como uno medio que invalide evidencia necesaria para el criterio de salida. El código existente no convierte automáticamente una idea en requisito aprobado ni una función parcial en una fase terminada.

## Etapa A — Recuperación de contexto (completada)

Objetivo: localizar las fuentes existentes sin cambiar el producto. La ejecución se realizó antes de esta documentación; su evidencia se registró durante la Etapa C.

Evidencia obtenida el 4 de septiembre de 2026:

- Se comparó el prototipo disponible con su backup: el contenido versionado coincide.
- Se comprobó la integridad del archivo y se inspeccionó una copia aislada.
- Los detalles de procedencia e identificadores se conservan únicamente en evidencia local excluida de Git.
- El backup contiene material no versionado: el historial recuperado de la conversación anterior, una base SQLite y previsualizaciones de interfaz.
- Ese material es referencia no confiable y potencialmente privada. No se incorpora al repositorio sin una revisión específica.

## Etapa B — Auditoría inicial (completada)

Objetivo: determinar si la base debe conservarse antes de diseñar o programar.

Resultados:

- Se conserva el prototipo como referencia local separada. El propietario eliminó y recreó el repositorio remoto; el nuevo clon limpio contiene únicamente documentación revisada y archivos legales.
- Las 19 pruebas existentes pasan, pero no acreditan todavía carga real, streaming completo, reinicios, seguridad del panel ni operación multinodo.
- El proyecto no declara dependencias externas de npm, una ventaja inicial para el objetivo de una VM con 1 GB de RAM.
- La documentación existente contiene estados contradictorios entre arquitectura y roadmap; se necesita una sola fuente de verdad.
- Autenticación, autorización, migraciones, protección contra replay, routing explicable y estimación de cuotas siguen siendo trabajo de producto, aunque existan implementaciones parciales.
- En el momento de la auditoría faltaba elegir una licencia; posteriormente se
  adoptó y añadió Apache-2.0 con NOTICE.

El alcance, evidencias, hallazgos y decisión de la revisión independiente se conservan en [Auditoría inicial](auditorias/2026-09-04-auditoria-inicial.es.md).

## Etapa C — Descubrimiento y definición (completada para la Fase 1)

Objetivo: convertir la visión en un contrato de producto coherente antes de modificar el código.

Primera decisión cerrada: [ADR-0001 adopta ModelCairn como nombre del producto](decisiones/0001-nombre-del-producto.es.md). Una revisión independiente detectó saturación de la raíz `Cairn` en software e IA. Tras comparar una segunda ronda de alternativas, se aceptó ese riesgo porque no existe una colisión exacta conocida, la metáfora representa el producto y el nombre funciona para su audiencia técnica.

Segunda decisión cerrada: [ADR-0002 elige Apache-2.0](decisiones/0002-licencia-y-atribucion.es.md). `LICENSE` y `NOTICE` están preparados para el primer commit. La atribución formará parte de la interfaz oficial; los activos de marca y el patrocinio se gestionarán por separado.

La [ADR-0003](decisiones/0003-base-tecnica-y-despliegue.es.md) selecciona Go,
monolito modular, systemd y artefactos Linux AMD64/ARM64. La
[ADR-0004](decisiones/0004-configuracion-secretos-y-recuperacion.es.md) define
SQLite como fuente de verdad, YAML aplicado explícitamente, custodia de secretos y
backup completo cifrado.

El [Project Charter de ModelCairn](project-charter.es.md) fue aprobado como base de
la Fase 1. Separa visión confirmada, propuestas, hipótesis y decisiones futuras.

El 5 de septiembre se preparó un primer paquete acelerado de definición:

- [Glosario y modelo de dominio](glosario-y-modelo-de-dominio.es.md);
- [Arquitectura objetivo inicial](arquitectura-objetivo.es.md);
- [Requisitos base priorizados](requisitos-base.es.md);
- [Fase 1 — Fundación operable](fases/fase-01-fundacion.es.md).

Los cuatro documentos fueron revisados, corregidos y aprobados como base de la
Fase 1.

Las decisiones agrupadas fueron aprobadas el 5 de septiembre. Los
[contratos técnicos de la Fase 1](fases/fase-01-contratos-tecnicos.es.md) y su
[plan de implementación](fases/fase-01-plan-de-implementacion.es.md) completan el
paquete previo al desarrollo. El Hito 0 produjo sus esquemas ejecutables, modelo
SQLite y ADR técnicos; QA independiente lo aprobó sin hallazgos críticos o altos.

La [ADR-0005](decisiones/0005-criptografia-y-formato-de-backup.es.md) fija
Argon2id, XChaCha20-Poly1305, manejo de clave maestra y restauración con recifrado.
La [ADR-0006](decisiones/0006-stack-de-consola-web.es.md) selecciona TypeScript,
React y Vite con activos incrustados en el ejecutable Go.

Entregables previstos:

1. Visión, problema, usuarios y casos de uso legítimos.
2. Alcance, exclusiones y principios de uso responsable.
3. Glosario y modelo de dominio.
4. Requisitos funcionales priorizados y trazables.
5. Requisitos no funcionales medibles: RAM, CPU, disco, latencia, concurrencia, disponibilidad y recuperación.
6. Modelo de amenazas, límites de confianza y tratamiento de secretos.
7. Contrato exacto de routing, errores, cooldown, fallback y streaming.
8. Modelo estadístico para límites y nivel de confianza de sus predicciones.
9. Contrato de configuración común para archivo, API y panel web.
10. Arquitectura objetivo y registros de decisiones arquitectónicas (ADR).
11. Estrategia de migraciones, compatibilidad y rollback.
12. Plan maestro de pruebas y criterios de aceptación.
13. Decisión y justificación de licencia, gobernanza y contribución open source.
14. Matriz que decida qué módulos actuales se conservan, corrigen o reemplazan.

Para la Fase 1 quedaron completos o suficientemente contratados los puntos que
afectan su implementación. El modelo estadístico detallado, protocolo de relays,
gobernanza comunitaria final y contratos de fases posteriores se documentarán
justo antes de desarrollarlos; no bloquean el Hito 1.

Criterio de salida: los documentos no se contradicen, cada requisito importante tiene criterio de aceptación y la arquitectura cabe razonablemente en los límites de recursos declarados. Después debe superar una revisión independiente.

## Etapa D — Planificación técnica de la Fase 1 (completada)

Objetivo: transformar los contratos aprobados en hitos pequeños, dependencias, riesgos, migraciones y pruebas. Esta etapa decide el orden de implementación; no implementa funciones.

Resultado: siete hitos de implementación más un Hito 0 documental, trazabilidad
RF/RNF, riesgos, puertas de entrada y definición de terminado. El Hito 0 fue
aprobado por QA independiente sin hallazgos críticos o altos pendientes.

## Etapa E — Implementación incremental (activa)

Cada incremento debe incluir código, pruebas, documentación operativa, medición de recursos y revisión independiente. No se aceptan grandes lotes de funciones sin una puerta de calidad intermedia.

El Hito 1 comenzó con el workspace modular de Go, CLI y ciclo HTTP mínimos,
sondas de health/readiness, Actions fijadas, compilaciones cruzadas para Linux e
inventario de dependencias. Los spikes contractuales ya verifican el comportamiento
de SQLite y el compromiso, cancelación y desconexión de SSE. El Hito 1 quedó
completado el 6 de septiembre después de aprobar CI y de que la ejecución revisada
de 15 minutos en la VM representativa de 1 GB informara 6.328 KiB de RSS pico y
cero swap del proceso. El Hito 2 es el siguiente.

## Etapa F — Validación de sistema y seguridad (pendiente)

Incluye carga en una VM equivalente a Oracle Free Tier, fallos de red, reinicios, migraciones, recuperación de backups, pruebas multinodo, privacidad, abuso del panel y revisión del modelo de amenazas.

## Etapa G — Preparación open source y publicación (pendiente)

Incluye incorporar la licencia elegida en la Etapa C, completar la guía de contribución y la política de seguridad, publicar versiones y notas de release, y verificar instalación, actualización y rollback reproducibles.

## Registro de cambios de etapa

| Fecha | Cambio | Evidencia |
|---|---|---|
| 2026-09-04 | Etapa A ejecutada y completada | Repositorio y backup comparados; evidencia técnica conservada localmente |
| 2026-09-04 | Etapa B ejecutada y completada | Auditoría local, 19 pruebas y revisión independiente |
| 2026-09-04 | Etapa C iniciada | Pausa explícita de implementación y lista de entregables |
| 2026-09-05 | Etapa C cerrada para Fase 1 | Charter, arquitectura, requisitos, contratos y ADR aprobados |
| 2026-09-05 | Etapa D completada para Fase 1 | Hitos, riesgos, trazabilidad y QA independiente |
| 2026-09-05 | Etapa E e Hito 1 iniciados | Esqueleto Go, pruebas, CI, compilaciones cruzadas y revisión independiente de código |
| 2026-09-06 | Hito 1 completado | Spikes SQLite y streaming, CI verde, compilaciones cruzadas y benchmark representativo revisado |
