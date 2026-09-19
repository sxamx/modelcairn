# Hito 7 — Evidencia de validación de sistema y seguridad

[English](hito-07-system-validation.md)

- Fecha: 19 de septiembre de 2026
- Revisión probada: `0cdcf02` (rama del PR #26)
- Resultado: bloques automatizados y VM representativa aprobados; el cierre de la
  Fase 1 continúa pendiente del QA independiente agrupado.

No se publican hostname, IP, usuario SSH, credenciales, rutas personales ni IDs de
ejecución. Las claves, contraseñas, sesiones y AgentTokens usados fueron temporales.

## Puertas automatizadas

CI aprobó en la revisión probada:

- Linux AMD64 y ARM64;
- `go vet`, race detector, cobertura y suite Go completa;
- escaneo de vulnerabilidades alcanzables;
- 14 pruebas de consola, build y activos incrustados reproducibles;
- contratos documentales, schemas SQLite y scripts de instalación;
- compuertas integradas de Hitos 2, 3 y 4 y smokes de recursos/concurrencia.

Localmente también aprobaron `go test ./...`, las 14 pruebas web y el manifiesto de
20 requisitos de Fase 1. El race detector no se ejecutó en Windows porque el Go
local carecía de CGO; la ejecución Linux de CI es la evidencia autoritativa.

## Fallos inducidos cubiertos

| Riesgo | Evidencia ejecutable |
|---|---|
| 429/5xx, clasificación y cooldown | `TestExecuteReturnsSafeNonSuccessMetadataWithoutBody`, `TestOperationalRecorderNormalizesRateLimitAndCreatesCooldown` |
| timeout/cancelación/desconexión | `TestScriptedDelayHonorsCancellation`, `TestScriptedDisconnectsAndExhaustionIsExplicit`, `TestEngineStopsImmediatelyWhenCallerIsCancelled` |
| stream antes/después del compromiso | `TestStreamEngineFallsBackBeforeCommitment`, `TestStreamEngineNeverFallsBackAfterCommitment`, suite `streaming` |
| SSRF, red privada, redirect y destino inválido | `TestForbiddenAddressPolicy`, `TestExecuteRejectsRedirectInvalidResponseAndPublicLoopback`, pruebas de parsing privado |
| límite total de intentos | `TestEngineHonorsAttemptCapAndRejectsStreaming` |
| configuración concurrente/obsoleta | `TestExecutePlanConcurrentConsumption`, `TestManagerRejectsStaleAndOptionChangedPlans` |
| migración interrumpida/deriva | `TestInterruptedInitialMigrationLeavesNoCommittedSubset` y suite `lifecycle` |
| rotación interrumpida | `TestRotationSurvivesProcessKillAtEveryBoundary` |
| capacidad de disco | `TestRequireFreeSpaceAcceptsSmallWriteAndRejectsImpossibleWrite` |
| backup corrupto/passphrase/restore | suite `backupmcb1` y evidencia funcional del Hito 6 |

Todos pasaron en CI. La cobertura canario se reparte deliberadamente por superficie:

- el gate Hito 2 escanea su directorio temporal completo después de parser, errores,
  plan, apply, export, metadatos, SQLite y logs;
- el gate Hito 3 prueba API administrativa/export y que contraseña/secreto no
  aparezcan en logs;
- el gate Hito 4 rechaza prompt, respuesta o secreto en SQLite y service log;
- las pruebas web confirman que el secreto solo viaja en su escritura, se limpia
  del campo y no llega a almacenamiento del navegador;
- la suite/evidencia Hito 6 verifica backup cifrado y ausencia del canario en texto
  claro.

Ninguna superficie encontró el canario. Las respuestas HTTP que necesariamente
devuelven el contenido solicitado no se consideran una filtración.

## Retención validada

La retención de estadísticas de login cubre valor inicial de 24 horas, duración
arbitraria hasta 100 años e ilimitada (`0`). La suite comprobó poda sin un nuevo
fallo, frontera inclusiva, saturación y reintento seguro de transición. La consola
probó el flujo plan/apply para retención ilimitada.

La retención general de solicitudes, intentos, observaciones y auditoría sigue
siendo RF-202. La validación no la presenta como implementada ni bloquea por ella
el cierre de Fase 1.

## VM representativa de 1 GB

El mismo harness integrado del Hito 4 se extendió con carga sostenida. Usa el
binario real, bootstrap, sesión administrativa, configuración publicada,
AgentToken, adaptador directo, persistencia y un upstream HTTP local determinista.
Durante 600 segundos mantuvo 10 streams concurrentes y consultó el overview una
vez por segundo.

| Medición | Resultado |
|---|---:|
| Memoria total de la VM | 975.064 KiB |
| Streams sostenidos correctos | 15.284 |
| Consultas administrativas correctas | 358 |
| RSS medio/pico | 25.540 / 57.336 KiB |
| Presupuesto RSS | 131.072 KiB |
| Swap pico del proceso | 0 KiB |
| CPU del proceso | 173,580 s; 28,786% de un CPU lógico en promedio |
| Latencia sostenida media/máxima | 0,198811 / 0,987245 s |
| Directorio de datos final | 18.931.464 bytes |
| SQLite/WAL final | 14.733.312 / 4.165.352 bytes |
| Binario Linux AMD64 | 14.094.496 bytes |

La prueba terminó con código 0. El proceso permaneció bajo el presupuesto, no usó
swap y el reporte solo conserva metadatos. Los datos de pruebas anteriores en
`/var/lib` conservaron propietario y modo; esta ejecución trabajó en un directorio
temporal aislado.

## Seguridad y privacidad

El [modelo de amenazas](../seguridad/modelo-amenazas-fase-1.es.md) enlaza activos,
límites, controles y riesgos residuales. Las suites probaron cifrado autenticado,
llavero fail-closed, redacción, sesiones/CSRF, límites de login, AgentTokens,
permisos, SSRF, planes de un uso y recuperación generacional. Prompts/respuestas
no se almacenaron y no existe telemetría externa incorporada.

## Límites de esta evidencia

- El upstream es local y determinista: mide ModelCairn, no latencia o disponibilidad
  de un proveedor público.
- RSS no incluye page cache del kernel; CPU es solo la del proceso ModelCairn.
- El ensayo no valida relays ni estimador adaptativo, ambos fuera de Fase 1.
- La consulta concurrente de consola fue overview; las demás vistas están cubiertas
  por pruebas web/HTTP, no por esta carga de 10 minutos.
- La aceptación definitiva depende del QA agrupado y de resolver sus hallazgos
  bloqueantes, si existen.
