# Propiedad de SQLite y durabilidad del llavero v1

[English](propiedad-y-llavero-v1.md)

## Propiedad del proceso

El directorio de datos configurado es privado para la identidad del servicio.
Tanto `serve` como cada comando CLI offline con estado abren `modelcairn.lock` en
ese directorio sin seguir enlaces simbólicos y adquieren el mismo bloqueo exclusivo
no bloqueante del sistema operativo **antes** de abrir SQLite o archivos de claves.
El servicio conserva el descriptor durante toda su vida; la CLI lo conserva hasta
terminar commit, sync y cierre.

Si no adquiere el bloqueo, devuelve `installation_in_use` y no toca SQLite ni el
llavero. El archivo puede permanecer después de salir, pero el kernel libera el
bloqueo al cerrar el descriptor o morir el proceso; su existencia nunca representa
el estado del bloqueo. Las pruebas inician contendientes servicio/CLI en ambos
órdenes y demuestran que exactamente un propietario abre la base.

## Llavero privado

### Codificación del secreto

Los datos asociados del cifrado v1 comienzan con los bytes ASCII de
`modelcairn/secret/v1` y un byte cero. Siguen el ID de instalación y el del secreto:
cada uno lleva su longitud en bytes como entero sin signo de cuatro bytes
big-endian, seguido de los bytes del ID. Las versiones del recurso y de la clave
se codifican después como enteros sin signo de ocho bytes big-endian. Los IDs no
están vacíos y tienen hasta 128 bytes; las versiones son positivas. Cambiar esta
codificación requiere una migración explícita del formato.

La clave del fingerprint usa HKDF-SHA-256 con la clave maestra de 32 bytes como
entrada, los bytes del ID de instalación como salt, la cadena
`modelcairn/secret-fingerprint/v1` como info y 32 bytes de salida. HMAC-SHA-256
autentica los bytes del secreto; sus primeros 12 bytes se codifican en base64url
sin relleno, con el prefijo `mc_fp_`. Solo es un identificador visual, no un
verificador de contraseña ni un token de autenticación.

`installation_state.key_check` es HMAC-SHA-256 con la clave maestra activa sobre
el dominio ASCII `modelcairn/master-key-check/v1`, un byte cero y los bytes del ID
de instalación. Permite que una instalación vacía rechace una clave sustituida;
no es material de clave ni una credencial exportable.

### Publicación

El directorio del llavero usa modo `0700`. Cada archivo inmutable
`v<version>.key` usa modo `0600`, contiene exactamente 32 bytes aleatorios y se
crea sin seguir enlaces. Una clave nueva se publica mediante estos límites durables:

1. crear exclusivamente un temporal privado en el mismo directorio;
2. escribir exactamente 32 bytes, sincronizar el archivo y cerrarlo;
3. renombrarlo atómicamente al nombre versionado todavía libre;
4. sincronizar el directorio del llavero;
5. en una transacción SQLite, recifrar cada secreto, actualizar `key_version`,
   recalcular su fingerprint visible, actualizar la versión activa y confirmar;
6. reabrir y descifrar cada fila para verificar;
7. eliminar una clave antigua solo después de que una consulta confirmada demuestre
   que ninguna fila la referencia y sincronizar después el directorio.

En Linux, los límites 3–4 usan un renombrado que no reemplaza seguido de `fsync`
del directorio. Windows usa `MoveFileEx` sin reemplazo y con
`MOVEFILE_WRITE_THROUGH`, porque su API de archivos generalmente no expone `fsync`
para directorios; en ambos casos el archivo temporal de la clave se sincroniza
antes de publicarlo.

En Unix, el directorio de datos, el llavero y los archivos de clave deben pertenecer
al UID efectivo del servicio además de usar los modos exigidos. En Windows,
ModelCairn aplica una DACL protegida que concede control total únicamente a la
identidad del servicio, Local System y Administradores integrados, verifica que el
propietario de la clave sea la identidad del servicio y rechaza cualquier permiso
adicional antes de leer sus bytes.

Una interrupción previa al commit deja una clave nueva sin referencias; una
interrupción posterior deja claves requeridas y obsoletas. El arranque verifica
todas las versiones referenciadas antes de readiness y solo puede limpiar versiones
sin referencias. Si falta una versión referenciada, falla cerrado. Las pruebas de
corte cubren cada límite numerado, incluidos sync de directorio y limpieza, y
verifican que ciphertext y fingerprint correspondan siempre a la versión confirmada.

### Implementación de rotación y recuperación

`RotateMasterKey` serializa las mutaciones, procesa secretos fila por fila y
confirma la prueba de clave y auditoría de éxito junto con el cifrado y los
fingerprints. Mantiene las versiones lógicas y fechas de cada secreto. Las claves
nuevas superan la versión activa y todas las versiones huérfanas publicadas.
SQLite usa `synchronous=FULL`; también se sincroniza el padre del llavero.

Un commit de resultado incierto o una verificación posterior fallida deshabilita
el almacén hasta reabrirlo. Una versión devuelta distinta de cero indica que la
base confirmó el cambio; un error posterior puede corresponder a verificación o
limpieza. `CollectUnusedKeys` vuelve a validar la clave activa en disco y todas
las filas, conserva cada versión referenciada (incluso no activa) y elimina solo
versiones sin referencias y archivos regulares `.new-<32 hex minúsculas>.tmp`
bajo propiedad exclusiva de la instalación. No toca nombres desconocidos.

Una instalación existente con prueba de clave NULL falla cerrada. La columna
nullable permite migrar el esquema, pero el arranque no fabrica una prueba
ausente para una instalación existente. Una instalación nueva confirma identidad
y prueba juntas.

Doce puntos de terminación de procesos cubren publicación, recifrado parcial,
commit, verificación, borrado de claves, limpieza de temporales y sincronización
final. Demuestran recuperación de procesos, no simulan pérdida eléctrica ni el
controlador de almacenamiento. Linux usa fsync de directorios; Windows publica
con renombrado write-through y la durabilidad de limpieza queda limitada por su
API de archivos.
