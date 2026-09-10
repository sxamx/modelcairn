# Configuración administrativa y auditoría de login v1

[English](admin-settings-v1.md)

Estado: propuesta de implementación; revisión agrupada registrada en el plan del hito. Retención pendiente de decisión.

## Fuente y ciclo de configuración

Un registro SQLite versionado, separado de recursos de proveedores, es la fuente
de verdad. `modelcairn admin bootstrap` importa inicialmente un documento acotado;
actualizaciones posteriores pasan por plan/apply explícito, offline con propiedad
exclusiva u online mediante API administrativa. Editar un archivo exportado no
cambia el proceso. Archivo, API y futura consola comparten modelo.

Documento: `apiVersion: modelcairn.io/v1alpha1`, `kind: AdminSettings`,
`resourceVersion` positivo para actualizar (omitido al crear), y `spec` con los
campos siguientes. Rechazar desconocidos, duplicados, aliases YAML, nulls,
documentos múltiples y archivos mayores de 64 KiB. Omitir un campo conserva su
valor al actualizar y usa default al crear. Actualización atómica con versión
observada. Es un documento separado; no amplía silenciosamente los tipos del
envelope Configuration existente.

El [esquema estructural](config/admin-settings-v1alpha1.schema.json) separa documentos
iniciales, actualizaciones y documentos resueltos. Defaults son anotaciones: el
decodificador compartido debe resolverlos explícitamente, sin reemplazar omisiones
de actualizaciones por defaults. Después validar semántica: idle <= absolute,
tasa/ráfaga de cliente <= global, origen/CIDR canónicos, IP literal de escucha y
puerto 1..65535, y combinaciones de transporte. loopback-http exige HTTP y origen/
listener loopback; ambos modos TLS exigen origen HTTPS. proxy-tls exige CIDR de
confianza no vacíos; otros modos exigen lista vacía. direct-tls exige ambas rutas
TLS no vacías; otros modos exigen rutas vacías. Cambiar modo exige limpiar
explícitamente campos del anterior. Rutas TLS absolutas y locales, sin NUL;
validar acceso al arrancar sin devolver contenidos ni errores crudos del sistema.

GET `/settings` devuelve documentos deseado/efectivo completamente resueltos y
restartRequired, calculado comparando especificaciones efectivas, no solo versiones.
`/settings/plan` y `/settings/apply` reciben actualizaciones, nunca creación inicial.
Apply sin cambios consume plan, pero no aumenta versión ni exige reinicio. Plan
devuelve documento deseado resuelto (con versión observada), nombres de campos
modificados, token y vencimiento. Apply devuelve snapshot confirmado y appliedAt,
sin releer settings después de liberar la transacción. GET sirve también para exportar.

| Campo de spec | Tipo / valor inicial | Validación |
|---|---|---|
| publicOrigin | string obligatorio | Origen canónico según admin-runtime-v1 |
| listen | string, 127.0.0.1:8080 | IP literal y puerto, sin resolver hostname |
| transport | enum, loopback-http | loopback-http, direct-tls, proxy-tls |
| trustedProxyCidrs | array string vacío | Hasta 32 CIDR canónicos; obligatorio solo en proxy-tls |
| tlsCertificatePath | string vacío | Obligatorio en direct-tls; certificado local legible |
| tlsPrivateKeyPath | string vacío | Obligatorio en direct-tls; clave privada legible por servicio |
| idleSeconds | entero, 1800 | 300..86400 y no mayor que absoluteSeconds |
| absoluteSeconds | entero, 43200 | 300..604800 |
| globalAttemptsPerMinute | entero, 30 | 1..120 |
| globalBurst | entero, 5 | 1..20 |
| clientAttemptsPerMinute | entero, 5 | 1..30 y no mayor que tasa global |
| clientBurst | entero, 3 | 1..10 y no mayor que ráfaga global |
| maxClientEntries | entero, 1024 | 64..4096 |
| clientIdleSeconds | entero, 900 | 60..3600 |
| argonMemoryKiB | entero, 19456 | 19456..65536 |
| argonIterations | entero, 2 | 2..6 |

Paralelismo Argon y verificaciones simultáneas fijos en uno para esta versión.
La espera progresiva sigue admin-runtime-v1. Subir parámetros afecta nuevos hashes;
los existentes dentro de límites siguen siendo verificables. PHC nunca autoriza
reservar memoria fuera de los límites duros admitidos.

Por simplicidad todos los cambios se activan mediante reinicio explícito. HTTP
devuelve versiones deseada/efectiva y restartRequired. Validar combinaciones antes
de persistir. Guardar no cambia el listener ni interrumpe la conexión de guardado.
Al arrancar, validar configuración y acceso a certificado/clave antes de escuchar;
si falla, corregir por CLI offline, nunca bajar automáticamente a HTTP inseguro.
Export y API nunca muestran el contenido de claves TLS.
Duraciones de sesión capturadas al emitir: tras reiniciar, los nuevos valores solo
afectan sesiones nuevas. Una sesión vencida no vuelve a ser válida porque el
operador aumente idleSeconds.

El [borrador de almacenamiento](storage/admin-identity-v1.es.md) define una migración
nueva con singleton `admin_settings`, versión, documento JSON
validado y timestamp. No modificar migraciones publicadas. Endpoints GET/plan/apply
de settings usan sesión/CSRF. Tokens ligados a revisión, propósito distinto y digest
del documento deseado completamente resuelto y normalizado, incluyendo valores
conservados y defaults. Reutilizar límite de vencimiento y semántica transaccional
de uso único de planes de configuración. Apply exige ese documento y revisión,
rechaza tokens vencidos o consumidos y consume el plan atómicamente con actualización
y auditoría. Una transacción fallida no consume el plan. Un plan de proveedores no
autoriza settings, ni a la inversa.

## Auditoría de login fallido

Propuesta: persistir agregados por minuto en lugar de un evento por intento.
Cada registro contiene minuto UTC, motivo de enum y contador saturable. Motivos:
invalid_credentials, throttled, malformed, unavailable. Excluir usuario, IP,
Origin, cabeceras, contraseña y texto libre. Cuatro contadores del minuto activo
en memoria, volcados por upsert como máximo una vez por minuto; al apagar, intentar
volcar lo restante. Un crash puede perder el último minuto sin volcar: son métricas
operativas, no un historial forense durable de cada intento.

Tabla acotada a 1440 minutos por motivo (5760 filas). Borrar vencidos dentro de la
transacción de volcado. Saltos del reloj hacia adelante limpian datos antiguos;
retrocesos no pueden superar el máximo de filas. Disco lleno no crea una cola
ilimitada: conservar solo contadores actuales acotados e informar indisponibilidad
agregada. Login exitoso y mutaciones administrativas conservan auditoría
transaccional. Esta ventana operativa no cambia la retención de eventos de
proveedores ni su opción de conservación indefinida.

Antes de implementar, aprobar esta ventana fija de estadísticas de login o elegir
retención configurable. Se conserva como propuesta porque afecta qué histórico
puede consultar el operador.
