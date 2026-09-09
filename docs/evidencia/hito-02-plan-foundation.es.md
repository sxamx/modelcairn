# Hito 2 — Base de autenticación de planes

[English](hito-02-plan-foundation.md)

Estado: base interna revisada; punto de entrega 5 todavía en curso.
Registro: 2026-09-08.

## Implementado

- La resolución de configuración conserva valores opcionales omitidos al
  actualizar, aplica defaults al crear, reemplaza arrays y comprueba las
  identidades y versiones suministradas.
- La preparación de solo lectura calcula operaciones ordenadas de creación,
  actualización, conservación y borrado, y valida el grafo resultante.
- Los tokens usan HMAC-SHA-256 con una clave HKDF separada por propósito y ligada
  a la instalación. Vinculan instalación, versión de clave, revisión de
  configuración, hash de operación, identidades/versiones/ausencias observadas,
  nonce y tiempos de emisión y vencimiento.
- El hash interno de operación es SHA-256 codificado en base64url. La integración
  debe calcularlo sobre la configuración deseada canónica y sus opciones,
  incluido el permiso de borrado; nunca sobre salida redactada ni omitiendo
  opciones relevantes para la autorización.
- La ejecución mantiene bloqueada la clave hasta el commit, verifica el plan,
  consume el nonce y ejecuta los cambios en una sola transacción SQLite. Si falla,
  también revierte el consumo del nonce. Una operación sin cambios consume su
  token. Un commit de resultado incierto deshabilita el almacén hasta reabrirlo.
- El reloj se consulta después de obtener los bloqueos y leer el estado, para que
  la espera no amplíe la duración del token.

## Verificación

- Pasan las pruebas locales de firmas alteradas, campos firmados inválidos,
  rechazo entre instalaciones y tras rotación, límites de reloj, conflictos de
  estado, rollback, reutilización, doble consumo concurrente y vencimiento durante
  la lectura del estado.
- Pasan `go test ./... -skip '^TestKernelReleasesLockAfterOwnerProcessDies$'` y
  `go vet ./...`. La prueba existente excluida encuentra acceso denegado en Windows
  al terminar su proceso auxiliar.
- La suite completa de almacenamiento compilada pasa en la VM Linux
  representativa, incluida esa prueba. Se suministró su esquema documentado en
  un directorio temporal aislado. `/usr/bin/time -v`: RSS máximo 30.852 KiB,
  swap 0 y 6,38 segundos. Es consumo de pruebas, no del programa en producción.
- QA estático independiente encontró el problema del reloj y aceptó la
  corrección y sus pruebas. No ejecutó pruebas de forma independiente porque su
  sandbox Windows impedía establecer los permisos del directorio temporal.

## Integración pendiente

Esto no completa la CLI ni acepta el punto de entrega 5. Falta conectar lecturas
reales de la base de datos y hashes canónicos al ejecutor; implementar cambios
multirrecurso, revisión y auditoría dentro de su transacción; incorporar archivos
de plan, confirmación interactiva, entrada acotada, exportación, comandos de
secretos y pruebas de integración. Las funciones internas deben usar solo la
transacción recibida, sin abrir operaciones de base de datos adicionales ni
volver a entrar en SecretStore. El ejecutor no construye el estado observado,
modifica recursos ni incrementa revisiones automáticamente. CI y QA de la CLI
completa siguen pendientes para la entrega posterior.
