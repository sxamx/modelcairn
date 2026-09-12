# Evidencia de integración administrativa — Hito 3

[English](hito-03-integracion-admin.md)

- Fecha: 12 de septiembre de 2026
- Estado: puerta local y medición VM aprobadas; CI del incremento pendiente

`scripts/verify-hito3.sh` construye un binario limpio y usa una instalación temporal.
Recorre bootstrap, arranque/readiness, login, recuperación de sesión, alta y lectura
de secreto, plan/apply/export de configuración, estado/emisión/revocación de
AgentToken y logout. Confirma que logout elimina la sesión local, que contraseña y
API key no aparecen en log/export y que el bearer se entrega en su única salida
intencional. Todos los temporales se eliminan al salir.

La puerta pasó localmente junto con todas las pruebas Go, `go vet` y el verificador
documental. CI la ejecutará en Linux desde este incremento. Esta evidencia no
La [medición representativa](benchmark-hito-03-2026-09-12.es.md) también está
aprobada. Queda pendiente la revisión independiente final del hito.
