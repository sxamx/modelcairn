# ADR-0005: Criptografía y formato de backup

[English](0005-criptografia-y-formato-de-backup.md)

- Estado: aceptada técnicamente; parámetros sujetos a benchmark
- Fecha: 2026-09-05

## Decisión

- Contraseñas administrativas: Argon2id con sal única de 16 bytes. Mínimo inicial
  `m=19456 KiB`, `t=2`, `p=1`; el instalador podrá aumentar el coste si la VM cumple
  el objetivo de 100–500 ms sin superar el presupuesto de memoria.
- Cifrado de API keys: XChaCha20-Poly1305 con clave de 256 bits, nonce aleatorio de
  24 bytes y datos asociados que incluyen versión, ID de instalación, ID y versión
  de credencial.
- Clave maestra: 32 bytes del CSPRNG del sistema, archivo separado de SQLite, modo
  `0600`, propietario del servicio. Rotación versionada y transaccional.
- Tokens de agente: 32 bytes aleatorios codificados para transporte; se muestra el
  valor una vez y se almacena SHA-256 del token completo. Su entropía aleatoria de
  256 bits evita depender de la clave maestra y permite que sus verificadores
  sobrevivan a rotación y restauración. No se aceptan tokens elegidos por el usuario.
- Backup completo: perfil MCB1 sobre el formato streaming age v1 con destinatario
  por contraseña scrypt. El manifiesto y checksums internos complementan, pero no
  sustituyen, la autenticación del formato age.
- La contraseña del backup se solicita de forma interactiva o por descriptor de
  archivo; nunca por argumento de línea de comandos.

## Restauración

1. Leer y validar límites, versión y estructura sin extraer archivos.
2. Derivar la clave de backup y autenticar todo el contenido antes de mutar estado.
3. Restaurar en un directorio temporal del mismo filesystem con permisos privados.
4. Generar una clave maestra nueva para la instalación de destino.
5. Descifrar cada secreto desde el contenedor y recifrarlo con la clave nueva.
6. Ejecutar migraciones y comprobaciones de integridad sobre la copia.
7. Detener escrituras, crear backup de recuperación y realizar reemplazo atómico.
8. Arrancar, comprobar readiness y probar una ruta seleccionada por el operador.
9. Ante cualquier fallo, conservar la instalación anterior y eliminar temporales
   de forma segura cuando el sistema lo permita.

No se restaura la clave maestra original. Esto evita reutilizar indefinidamente la
misma clave entre máquinas y permite recuperar en otra VM solo con el backup y su
contraseña.

## Formato lógico MCB1

El perfil age, límites, entradas y reemplazo generacional se definen en la
[especificación MCB1](../contratos/backup-mcb1.es.md). No incluye logs del sistema,
claves TLS ni credenciales SSH. Límites de tamaño y número de entradas se validan
antes de reservar memoria o disco.

## Consecuencias

- Un atacante con root en ejecución puede acceder a la clave maestra; este ADR no
  afirma lo contrario.
- Perder la contraseña del único backup completo hace irrecuperables sus secretos.
- Los parámetros Argon2id se guardan junto al hash/ciphertext para permitir aumento
  progresivo sin invalidar datos anteriores.
- Rotar la clave maestra recifra API keys, pero no cambia sesiones ni tokens de
  agente durante operación normal. Restaurar elimina todas las sesiones
  administrativas y conserva los verificadores y metadatos de revocación de tokens
  de agente.
- Las bibliotecas se fijarán por versión y pasarán análisis de dependencias.

## Fuentes

- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [Paquete oficial Go XChaCha20-Poly1305](https://pkg.go.dev/golang.org/x/crypto/chacha20poly1305)
- [Biblioteca y formato age](https://github.com/FiloSottile/age)
