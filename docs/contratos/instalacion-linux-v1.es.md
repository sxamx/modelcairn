# Contrato de instalación nativa Linux v1

[English](instalacion-linux-v1.md)

La ruta soportada requiere Linux con `systemd`, arquitectura AMD64 o ARM64 y
privilegios administrativos durante instalación. El servicio posterior no usa root.

| Elemento | Ruta/identidad | Permiso |
|---|---|---:|
| Binario | `/usr/local/bin/modelcairn`, root:root | `0755` |
| Ajustes de servicio | `/etc/modelcairn`, root:modelcairn | `0750` |
| Estado | `/var/lib/modelcairn`, modelcairn:modelcairn | `0700` |
| Unidad | `/etc/systemd/system/modelcairn.service`, root:root | `0644` |
| Proceso | usuario/grupo de sistema `modelcairn` | sin shell/home |

El instalador exige un binario local ejecutable, nunca descarga código mediante
una tubería al shell. Repetirlo reemplaza atómicamente binario, unidad y dirección
de escucha, pero conserva todo el directorio de datos. No abre firewall, no instala
Tailscale y no elimina datos.

`--enable` controla arranque tras reinicio; `--start` controla la ejecución actual.
En modo interactivo ambas decisiones se preguntan por separado. El modo no
interactivo exige expresarlas y falla si quedan implícitas.
`--no-start` detiene una instancia ya activa, de modo que actualizar el binario no
deja ejecutándose silenciosamente la versión anterior.

La dirección predeterminada es `127.0.0.1:8080`. Usar `0.0.0.0` o `[::]` expone el
servicio a la red alcanzable y requiere que el operador configure firewall y HTTPS.
Para Tailscale se conserva loopback y se publica externamente con Tailscale Serve.

Desinstalar la unidad o el binario nunca autoriza borrar `/var/lib/modelcairn`.
Eliminar datos exige una acción separada, explícita y documentada después de crear
un backup verificable.
