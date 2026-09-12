# Contrato de CLI online v1

[English](cli-online-v1.md)

Estado: sesión y ciclo de vida de AgentToken implementados; paridad de
configuración y secretos pendiente.

La CLI online usa exclusivamente la API administrativa y nunca abre el directorio
de datos. `--server` es el origen público exacto, sin ruta, query ni credenciales.
HTTPS es obligatorio salvo HTTP hacia loopback. No se siguen redirecciones.

```text
modelcairn admin login --server origen --username nombre [--session-file ruta]
modelcairn admin whoami --server origen [--session-file ruta]
modelcairn admin logout --server origen [--session-file ruta]
modelcairn agent-token status --server origen [--session-file ruta] <nombre>
modelcairn agent-token issue --server origen [--session-file ruta] <nombre>
modelcairn agent-token revoke --server origen [--session-file ruta] <nombre>
```

Login lee la contraseña mediante terminal sin eco o stdin, nunca argv. La sesión
se guarda bajo el directorio de configuración del usuario en un archivo `0600` en
POSIX; `--session-file` permite una ubicación explícita. El archivo contiene cookie,
CSRF, origen y vencimiento, por lo que es una credencial local y no debe versionarse
ni compartirse. `whoami` recupera/rota CSRF y actualiza el archivo. Logout primero
confirma revocación en el servidor y luego elimina la copia local.

Cada petición envía Origin exacto, cookie y CSRF, limita respuestas a 1 MiB y usa
timeout. Ante 403 recupera CSRF mediante `session/me` y reintenta exactamente una
vez. La CLI no imprime cookie ni CSRF. `agent-token issue` es la única salida que
entrega intencionalmente el bearer una vez. La futura conexión de config y secretos
reutilizará este cliente; no creará un segundo protocolo.
