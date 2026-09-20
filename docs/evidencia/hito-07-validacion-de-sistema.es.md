# Hito 7 — Evidencia de validación de sistema y seguridad

[English](hito-07-system-validation.md)

- Fecha: 19 de septiembre de 2026
- Revisión probada: `4f6dd61` (rama del PR #26)
- Corrección funcional final: `bbdb472`
- Resultado: **aprobado**; bloques automatizados, VM representativa y
  [QA independiente agrupado](qa-agrupado-hito-07.es.md) superados.

No se publican hostname, IP, usuario SSH, credenciales, rutas personales ni IDs de
ejecución. Las claves, contraseñas, sesiones y AgentTokens usados fueron temporales.

## Puertas automatizadas

CI aprobó en la revisión probada:

- Linux AMD64 y ARM64;
- `go vet`, race detector, cobertura y suite Go completa;
- escaneo de vulnerabilidades alcanzables;
- 17 pruebas de consola, build y activos incrustados reproducibles;
- contratos documentales, schemas SQLite y scripts de instalación;
- compuertas integradas de Hitos 2, 3 y 4 y smokes de recursos/concurrencia.

Localmente también aprobaron `go test ./...`, las 17 pruebas web, el verificador
ejecutable de egreso y el manifiesto de 20 requisitos de Fase 1. El race detector
no se ejecutó en Windows porque el Go
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
| Streams sostenidos correctos | 15.276 (mínimo: 10.000) |
| Consultas administrativas correctas | 356 (mínimo: 300) |
| RSS medio/pico | 25.874 / 57.320 KiB (máximo: 131.072 KiB) |
| Swap pico del proceso | 0 KiB (requerido: 0) |
| CPU del proceso | 173,090 s; 28,702% promedio (máximo: 50% de un CPU lógico) |
| Latencia sostenida media/máxima | 0,196686 / 0,982333 s (máximos: 0,350 / 2,000 s) |
| Directorio de datos final | 18.685.824 bytes (máximo: 33.554.432 bytes) |
| SQLite/WAL final | 14.467.072 / 4.185.952 bytes |
| Binario Linux AMD64 | 14.098.592 bytes |

La prueba terminó con código 0. El proceso permaneció bajo el presupuesto, no usó
swap y el reporte solo conserva metadatos. Los datos de pruebas anteriores en
`/var/lib` conservaron propietario y modo; esta ejecución trabajó en un directorio
temporal aislado.

El onboarding quedó validado como una cadena única: constructor compartido,
fixture exacto, aplicación contra el servidor real, emisión de AgentToken y rutas
normal y SSE. Por defecto anuncia solo texto; streaming y tools requieren selección
explícita. Las pruebas web también cubren la reconciliación cuando se pierde la
respuesta de apply y la revocación/reemisión segura si se pierde la respuesta que
contenía el token de única visualización.

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
- La rotación añadida tras el benchmark recibió una compuerta funcional Linux
  separada; no se repitió la carga de 600 segundos porque no cambia el router ni
  sus recursos medidos.
