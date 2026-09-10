# Almacenamiento de identidad administrativa — borrador Hito 3

[English](admin-identity-v1.md)

El [SQL propuesto](admin-identity-v1.draft.sql) es un contrato de diseño ejecutable,
no una migración del programa. Las migraciones publicadas 0001 y 0002 no cambian.
Tras aceptar el diseño, incorporar la siguiente migración secuencial al ejecutor
transaccional existente y sus verificaciones de checksum y compatibilidad.

## Configuración

`admin_settings` contiene como máximo una fila, con clave 1. Guarda solo `spec`
completamente resuelto, no el envelope: `resource_version` es la única versión
persistida. Reconstruir apiVersion/kind al exportar. SQL comprueba objeto JSON y
límite de 64 KiB; el validador compartido exige campos, tipos, defaults y relaciones
semánticas antes de guardar y al arrancar. SQL no sustituye ese validador. Bootstrap
confirma administrador, settings versión 1 y auditoría tipada juntos. Administrador
sin settings significa instalación incompleta: cerrar acceso y exigir reparación
offline, no crear otro administrador ni escuchar con defaults inseguros.

Actualizaciones comparan versión observada y confirman spec/versión, auditoría y
consumo del plan en una transacción. No-op conserva versión pero consume plan.
Reutilizar consumed_plan_tokens y rotación de claves con propósito/digest autenticado
específico de settings, sin otro mecanismo de consumo. Auditoría registra versiones
y nombres de campos modificados, nunca valores. Settings efectivos son el snapshot
validado al arrancar, en memoria, no otra fila mutable.

## Sesiones

El borrador revoca todas las sesiones activas anteriores antes de añadir idle_seconds:
se desconoce su política original. Filas históricas revocadas pueden mantener NULL.
Autenticación rechaza NULL independientemente del resto. Sesiones nuevas capturan
duración no nula y vencimiento absoluto desde settings efectivos; CHECK impide filas
sin revocar y sin duración. Cambiar settings no cambia la política capturada. No se
migran contraseñas ni valores bearer.

Un fallo revierte revocación y DDL. Tras aplicar correctamente, las sesiones antiguas
deben autenticarse de nuevo. Este borrador no prueba el registro de migraciones Go ni
las comprobaciones HTTP: requieren pruebas de implementación.

## Auditoría y aceptación

Acciones exitosas de identidad reutilizan audit_events con detalles tipados por
acción; no se introduce una API de auditoría genérica con texto libre. Agregados de
login fallido quedan pendientes de decidir retención. No se crea esa tabla ni se
aprueba implícitamente una ventana de 24 horas.

Ejecutar `node docs/contratos/storage/admin-identity-v1.contract.test.cjs` para probar
actualización desde datos previos al Hito 3, revocación, límites de settings y rollback.
Antes de aceptar runtime, probar también bootstrap concurrente, política capturada,
sesiones vencidas que no reviven, fallos transaccionales de plan/auditoría,
compatibilidad de checksum y reinicio real mediante el ejecutor de producción.
