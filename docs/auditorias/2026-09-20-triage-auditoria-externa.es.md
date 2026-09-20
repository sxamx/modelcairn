# Triage de auditoría externa — 2026-09-20

Estado: completada antes de publicar v0.1.0.

## Alcance y procedencia

Un agente externo revisó el repositorio sin disponer de todo el contexto del
proyecto y entregó un inventario de posibles problemas. Ese inventario se
conserva localmente como evidencia de entrada, pero no se publica sin validar:
una severidad propuesta no equivale a una decisión del proyecto.

Este documento registra el criterio aplicado por ModelCairn. No contiene
credenciales, rutas locales, direcciones de infraestructura ni identificadores
de checkpoints.

## Hallazgos confirmados y corregidos antes del nuevo candidato

- MC-QA-001, 012, 013 y 014: el onboarding ahora planifica de forma explícita
  un secreto futuro, crea sin sobrescritura, envía CSRF también en lecturas
  protegidas y no puede cerrarse con Escape mientras está ocupado.
- MC-QA-015: los identificadores de solicitud del proveedor se almacenan como
  referencias SHA-256 truncadas, nunca en texto controlado por el upstream.
- MC-QA-016: un resultado incierto al confirmar una mutación de secretos pone
  el almacén en modo fail-closed hasta reiniciar y reconciliar.
- MC-QA-017 y 018: la consola no simula un logout exitoso ante fallo y usa la
  forma real de los componentes de readiness.
- MC-QA-022, 023, 024 y 030: bootstrap distingue proxy TLS, el instalador
  valida la identidad de servicio, comprueba stop/disable y conserva
  automáticamente el listen existente durante una actualización.
- MC-QA-028, 041 y 047: restore y rollback invalidan credenciales históricas
  antes de activarlas, comprueban integridad/esquema del objetivo y no inventan
  un predecesor legacy en una instalación vacía.
- MC-QA-066 y 067: los archivos de release incluyen la ruta de instalación
  nativa y Settings rechaza puertos privilegiados o claves TLS ocultas por el
  sandbox del servicio oficial.
- MC-QA-068 y 069: README diferencia alcance actual y futuro; OpenAPI dejó de
  anunciar una operación todavía no implementada.
- MC-QA-070 y 071: los límites de SecretStore, MCB1, CRUD y configuración
  declarativa son coherentes, por lo que un estado aceptado sigue siendo
  respaldable y exportable.
- MC-QA-073: permitir redes privadas ya no permite enviar un bearer por HTTP a
  un hostname o una IP pública.

## Observaciones aceptadas como deuda, no como bloqueo de esta prerelease

El informe también identifica mejoras válidas que exceden el contrato de la
primera prerelease: orquestación transaccional completa del onboarding,
retención de métricas operativas, endurecimiento adicional ante slow clients,
prune de generaciones, paginación completa de la consola, recuperación
multi-tab de CSRF, comprobaciones funcionales más amplias durante upgrade y
automatización exclusiva de la creación inicial de tags.

Estas observaciones no deben ocultarse ni describirse como implementadas. Se
convertirán en trabajo posterior con alcance, contrato y prueba propios. La
compatibilidad de backups entre una versión futura N+1 y N se verificará cuando
exista una migración N+1 real; no puede probarse honestamente inventando hoy ese
schema futuro.

## Decisiones descartadas o reformuladas

- No se publica el informe bruto como verdad normativa ni se copian sus
  severidades automáticamente.
- No se implementa todavía el endpoint de prueba de conexión: se retiró del
  contrato hasta contar con semántica, límites SSRF y UX aprobados.
- No se conserva ningún bearer de agente al activar historia. La prerelease
  elige seguridad monotónica: los recursos sobreviven, pero los tokens deben
  emitirse nuevamente.
- Un indicador del navegador sin conectividad no bloquea el login hacia una
  instalación local; solo se presenta como señal informativa.

## Gate de salida completado

Antes de publicar se comprobó:

1. suite Go completa, tipos y pruebas web;
2. validadores documentales y de contratos;
3. paquete reproducible con scripts, unidad y guías operativas;
4. instalación y actualización desde el tarball en Linux;
5. candidato nuevo asociado exactamente al estado revisado;
6. checksums, SBOM y attestations verificados antes de crear el tag y la
   prerelease.

Todos los puntos se aprobaron. El informe bruto permanece como evidencia local
excluida de Git; este triage es su registro público revisado.
