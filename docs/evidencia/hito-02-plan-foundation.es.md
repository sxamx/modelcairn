# Hito 2 — Base de autenticación de planes

[English](hito-02-plan-foundation.md)

Estado: punto de entrega 5 aceptado.
Registro: 2026-09-09.

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
- El plan lee en un único estado SQLite la revisión, identidades y versiones,
  ausencias observadas y catálogo de nombres de secretos. Su hash cubre la
  configuración deseada canónica y el permiso de borrado.
- Apply recalcula ese estado dentro de la transacción, rechaza planes obsoletos o
  con opciones distintas, procesa escrituras por dependencias y luego borrados
  desde consumidores a dependencias, e incrementa una vez la revisión global. Un noop real consume el
  token sin cambiar la revisión.
- La exportación completa reconstruye todos los recursos persistidos y aplica el
  redactor compartido al entregar la salida.
- La CLI incorpora `config validate/plan/apply/export` y
  `secret set/metadata/rotate/delete`. Apply automatizado exige un archivo de plan;
  el modo interactivo conserva el bloqueo desde el plan hasta la confirmación. La
  entrada de secretos usa una terminal sin eco o stdin acotado.

## Verificación

- Pasan las pruebas locales de firmas alteradas, campos firmados inválidos,
  rechazo entre instalaciones y tras rotación, límites de reloj, conflictos de
  estado, rollback, reutilización, doble consumo concurrente y vencimiento durante
  la lectura del estado.
- Las pruebas de persistencia cubren creación y exportación del ejemplo documentado
  de diez recursos, planes obsoletos y con opciones cambiadas, revisión en noop,
  borrado completo por dependencias, migración de una referencia antes de borrar
  su dependencia, serialización redactada del plan, rollback de
  una relación intermedia insegura y conservación de IDs de destinos históricos.
- Las pruebas de comandos cubren validación sin crear estado, archivo de plan
  exclusivo, aplicación y reutilización, exportación, confirmación y cancelación,
  plan inválido antes de abrir estado y el ciclo completo de secretos sin exponerlos.
- La puerta local pasa `go test ./...` (salvo la prueba de finalización de procesos
  reservada para Linux), `go vet ./...`, validación de 86 documentos y `diff --check`.
- El binario Linux AMD64 completó en la VM representativa el ciclo de secreto,
  validación, plan, apply, export, consulta de metadatos y rotación. El canario no
  apareció en los artefactos ni en el directorio de datos. La medición de `plan`
  informó RSS máximo de 13.148 KiB, cero swap y 0,02 segundos; es una medición de
  operación individual, no una prueba de carga.
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
- El QA agrupado de la CLI detectó truncamiento silencioso, salida de metadatos sin
  redacción, límites incompatibles de plan, detección imprecisa de terminal y un
  código incorrecto de conflicto. Los cinco defectos quedaron corregidos y cubiertos.
- El CI del commit `ca37d12` aprobó formato, vet, vulnerabilidades alcanzables,
  suite completa, contratos, benchmarks, presupuesto de recursos, módulos y las
  compilaciones Linux AMD64/ARM64.

## Límite conservador aceptado

Las funciones internas deben usar solo la
transacción recibida, sin abrir operaciones de base de datos adicionales ni
volver a entrar en SecretStore. La integración oficial pasa por `config.Manager`;
los consumidores directos no deben construir mutaciones sin validar.

v1alpha1 conserva las restricciones de relaciones de SQLite durante todo apply.
Las proyecciones omiten las columnas relacionales que no cambian, por lo que se
pueden editar metadatos, estado y capacidades normalmente. Un cambio coordinado
de relaciones o un intercambio entre valores con
restricción UNIQUE puede ser válido como grafo final, pero imposible como estado
intermedio de SQLite; la transacción lo rechaza y revierte. Se puede expresar un
reemplazo con un valor restringido distinto, migración de referencias y borrado
explícito por dependencias, lo cual asigna
una identidad nueva donde se solicitó borrar. Mantener la identidad durante esos
intercambios requiere un diseño de transición revisado por separado; esta base no
debilita restricciones persistentes ni reescribe relaciones históricas de destinos.
