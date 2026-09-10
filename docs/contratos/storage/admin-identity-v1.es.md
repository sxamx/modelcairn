# Almacenamiento de identidad administrativa — Hito 3

[English](admin-identity-v1.md)

La migración de producción `0003_admin_settings.sql` implementa este contrato con
el ejecutor transaccional y verificaciones de checksum/compatibilidad existentes.
Las migraciones publicadas 0001 y 0002 no cambian. La prueba ejecutable lee 0003
directamente para evitar que una segunda copia SQL diverja de producción.

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

La persistencia de runtime decodifica únicamente JSON resuelto canónico byte a byte
y lo valida otra vez. Insertar revisión uno es una primitiva transaccional del
bootstrap: quien la llama debe crear administrador y auditoría tipada en esa misma
transacción. Ningún flujo público puede confirmar solo uno de esos registros.

El servicio local de identidad ya compone ese bootstrap: valida username, settings
y contraseña antes de escribir, deriva Argon2id antes de abrir la transacción y
confirma administrador, settings y `admin.bootstrap` juntos. Exactamente un intento
concurrente puede ganar; una instalación parcialmente formada se rechaza para
reparación offline. `admin reset-password` toma propiedad exclusiva mediante el
lock de instalación, lee los parámetros Argon2id persistidos, deriva fuera de la
transacción y confirma contraseña, incremento de auth_version, revocación de todas
las sesiones y `admin.password_reset` juntos. Un fallo de auditoría revierte todo.
La contraseña llega por TTY con confirmación o por stdin, nunca por argv.

`UpdateAdminSettingsTx` valida spec resuelto, comprueba versión observada, actualiza
singleton e inserta auditoría tipada `admin_settings.apply` en la transacción que
recibe. Nombres de campos modificados provienen de settings validados, sin valores.
No-op conserva versión y updated_at y registra la operación. El servicio debe
componerla con `ExecuteSettingsPlan` y revertir ante cualquier error. Las regresiones
inyectan fallo de auditoría y demuestran rollback de settings y nonce, seguido de
reintento exitoso. El servicio público plan/apply sigue pendiente.

## Sesiones

La migración 0003 revoca todas las sesiones activas anteriores antes de añadir idle_seconds:
se desconoce su política original. Filas históricas revocadas pueden mantener NULL.
Autenticación rechaza NULL independientemente del resto. Sesiones nuevas capturan
duración no nula y vencimiento absoluto desde settings efectivos; CHECK impide filas
sin revocar y sin duración. Cambiar settings no cambia la política capturada. No se
migran contraseñas ni valores bearer.

Un fallo revierte revocación y DDL. Tras aplicar correctamente, las sesiones antiguas
deben autenticarse de nuevo. Las pruebas Go cubren el registro de migraciones;
las comprobaciones HTTP requieren una entrega posterior.

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
Las pruebas Go ya cubren actualización secuencial, compatibilidad y reapertura;
enforcement HTTP queda para entregas posteriores; bootstrap y reset locales ya
tienen cobertura de concurrencia, rollback y entrada sin exposición del secreto.
