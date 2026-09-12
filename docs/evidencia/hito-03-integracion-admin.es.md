# Evidencia de integración administrativa — Hito 3

[English](hito-03-integracion-admin.md)

- Fecha: 12 de septiembre de 2026
- Estado: aceptado; puerta local, medición VM representativa y CI aprobadas

`scripts/verify-hito3.sh` construye un binario limpio y usa una instalación temporal.
Recorre bootstrap, arranque/readiness, login, recuperación de sesión, alta y lectura
de secreto, plan/apply/export de configuración, estado/emisión/revocación de
AgentToken y logout. Confirma que logout elimina la sesión local, que contraseña y
API key no aparecen en log/export y que el bearer se entrega en su única salida
intencional. Todos los temporales se eliminan al salir.

La puerta pasó localmente junto con todas las pruebas Go, `go vet`, el verificador
documental y el límite ejecutable de 128 MiB. El CI del incremento publicado pasó
en Linux, incluida detección de carreras y cross-builds AMD64/ARM64. La
[medición representativa](benchmark-hito-03-2026-09-12.es.md) también está aprobada.

La revisión independiente final encontró y después verificó las correcciones de
retención del historial de login, reintento ante fallo de persistencia, concurrencia
durante cierre y frontera exacta de 24 horas. La migración 0004, las regresiones
dirigidas y el mapa local de retención cierran esos hallazgos. Las estadísticas no
guardan prompt, respuesta, contraseña, dirección ni bearer.
