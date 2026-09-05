# Perfil de backup MCB1 sobre age

[English](backup-mcb1.md)

- Estado: contrato de Fase 1
- Objetivo: restauración completa portable y streaming sin diseñar criptografía

## Formato exterior

MCB1 no inventa un cifrador ni un framing binario. Es un perfil de aplicación del
formato estándar **age v1** con un único destinatario por contraseña (`scrypt`). El
archivo comienza con el header normal `age-encryption.org/v1`; la extensión
recomendada es `.mcb.age`. ModelCairn usa la biblioteca Go `filippo.io/age` fijada
por versión y sus interfaces streaming.

El lector acepta exclusivamente:

- age v1 binario, sin armor;
- exactamente una stanza `scrypt`, sin plugins ni otros recipients;
- work factor dentro del rango admitido por la versión fijada y la política local;
- payload autenticado por el formato age.

No existe un checksum circular ni parámetros criptográficos definidos por un
header propio. La biblioteca age limita e interpreta su KDF; ModelCairn rechazará
el archivo antes de crear una generación si el header, recipient o tamaño físico
no cumplen esta política.

## Payload streaming

El plaintext de age es un tar POSIX **sin compresión** en la Fase 1. Evitar
compresión reduce complejidad, picos de RAM y ataques de expansión. Solo admite:

- `manifest.json` (máximo 1 MiB);
- `database.sqlite` (máximo predeterminado 64 GiB y limitado por espacio libre);
- `secrets.jsonl` (máximo 64 MiB, un registro por línea);
- `checksums.json` (máximo 1 MiB).

Máximo cuatro entradas y 65 GiB de plaintext. La lectura es secuencial, con buffers
acotados; rechaza rutas absolutas, `..`, duplicados, enlaces, dispositivos, tamaños
negativos, overflow y bytes posteriores no declarados. Se calcula SHA-256 de cada
entrada mientras se escribe y se compara con `checksums.json` después de que age
haya autenticado el stream completo.

Los secretos se descifran uno a uno durante creación y entran directamente al
writer age; nunca se escriben en un temporal plaintext. Durante restore se leen uno
a uno desde age y se recifran directamente con la nueva clave maestra.

## Creación consistente

1. Comprobar espacio, límites y contraseña mediante TTY o descriptor seguro.
2. Abrir una instantánea SQLite con su API de backup.
3. Crear el tar a través del writer age en un archivo temporal privado.
4. Sincronizar, cerrar —lo que finaliza autenticación— y renombrar en el mismo
   filesystem.
5. Realizar una lectura de verificación antes de informar éxito.

## Restauración generacional

Los datos activos viven en `data/generations/<id>/` con `database.sqlite` y
`master.key`; `data/current` apunta a una generación completa.

La restauración descifra hacia una generación privada nueva, genera otra clave
maestra, migra la base y recifra secretos. Después de autenticar hasta EOF,
verificar checksums, integridad SQLite y permisos, cambia `data/current` mediante
rename atómico de un enlace temporal en el mismo filesystem. La generación anterior
se conserva hasta superar readiness y una prueba de ruta.

Antes de activar la generación se eliminan todas las filas de `admin_sessions`.
Ninguna cookie capturada antes del backup puede autenticar la instalación
restaurada. Se conservan los verificadores, expiración y revocación de tokens de
agente, porque son credenciales operativas explícitas y no dependen de la clave
maestra; el operador puede revocarlos antes o después del restore.

Contraseña incorrecta, header no permitido, stream truncado, falta de espacio,
migración fallida o secreto inválido dejan intacta la generación activa.

## Compatibilidad

El `manifest.json` contiene `profile: modelcairn-backup`, `profileVersion: 1`,
versión de aplicación y schema, fecha UTC, tamaños y lista exacta de entradas.
Perfiles desconocidos se rechazan. Una evolución incompatible utilizará otra
`profileVersion`; no reinterpretará MCB1.

## Fuente

- [age: herramienta, formato y biblioteca Go](https://github.com/FiloSottile/age)
- [Especificación age v1](https://github.com/C2SP/C2SP/blob/main/age.md)
