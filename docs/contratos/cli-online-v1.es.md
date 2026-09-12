# Contrato de CLI online v1

[English](cli-online-v1.md)

Estado: base de sesión implementada; paridad de operaciones pendiente.

La CLI online usa exclusivamente la API administrativa y nunca abre el directorio
de datos. `--server` es el origen público exacto, sin ruta, query ni credenciales.
HTTPS es obligatorio salvo HTTP hacia loopback. No se siguen redirecciones.

```text
modelcairn admin login --server origen --username nombre [--session-file ruta]
modelcairn admin whoami --server origen [--session-file ruta]
modelcairn admin logout --server origen [--session-file ruta]
```

Login lee la contraseña mediante terminal sin eco o stdin, nunca argv. La sesión
se guarda bajo el directorio de configuración del usuario en un archivo `0600` en
POSIX; `--session-file` permite una ubicación explícita. El archivo contiene cookie,
CSRF, origen y vencimiento, por lo que es una credencial local y no debe versionarse
ni compartirse. `whoami` recupera/rota CSRF y actualiza el archivo. Logout primero
confirma revocación en el servidor y luego elimina la copia local.

Cada petición envía Origin exacto, cookie y CSRF, limita respuestas a 1 MiB y usa
timeout. La CLI no imprime cookie ni CSRF. Una futura conexión de config, secretos
y AgentToken reutilizará este cliente; no creará un segundo protocolo.
