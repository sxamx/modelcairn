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

Una interrupción previa al commit deja una clave nueva sin referencias; una
interrupción posterior deja claves requeridas y obsoletas. El arranque verifica
todas las versiones referenciadas antes de readiness y solo puede limpiar versiones
sin referencias. Si falta una versión referenciada, falla cerrado. Las pruebas de
corte cubren cada límite numerado, incluidos sync de directorio y limpieza, y
verifican que ciphertext y fingerprint correspondan siempre a la versión confirmada.
