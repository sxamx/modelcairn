# Contrato de CLI offline v1

[English](cli-v1.md)

La CLI offline obtiene propiedad exclusiva del directorio de datos para cada
comando con estado. No puede ejecutarse a la vez que `serve` u otro propietario
offline. `config validate` es la excepción: procesa un archivo acotado sin abrir,
crear ni migrar una instalación.

## Flujo de configuración

```text
modelcairn config validate <archivo>
modelcairn config plan [--data-dir ruta] [--allow-delete] [--out plan.json] <archivo>
modelcairn config apply [--data-dir ruta] [--allow-delete] [--plan plan.json] <archivo>
modelcairn config export [--data-dir ruta]
```

`plan --out` crea un archivo nuevo y se niega a sobrescribir una ruta existente.
En sistemas POSIX solicita modo `0600`; en Windows, la privacidad efectiva depende
de la ACL heredada del directorio y el usuario debe elegir un directorio privado.
La emisión y la lectura rechazan planes mayores de 32 MiB, por lo que todo plan
emitido cabe en el límite de consumo. El JSON contiene un token autenticado de diez minutos, cambios ordenados
y configuración canónica redactada. Sin `--out`, escribe el mismo JSON en stdout.
El plan pertenece a una instalación, sirve una sola vez y queda vinculado a la
entrada exacta, permiso de borrado, versión de clave, revisión y recursos observados.

Apply no interactivo exige `--plan`. Procesa configuración y plan dentro de sus
límites antes de abrir la instalación; luego verifica y consume el token en la
misma transacción de los cambios. Apply interactivo crea y muestra un plan nuevo y
acepta únicamente `y` o `yes`; mantiene el bloqueo de instalación desde el plan
hasta la confirmación y aplicación. Cualquier otra respuesta cancela sin modificar
configuración. Export produce JSON determinista y nunca valores secretos.

## Flujo de secretos

```text
modelcairn secret set [--data-dir ruta] [--version n] <nombre>
modelcairn secret metadata [--data-dir ruta] [nombre]
modelcairn secret rotate [--data-dir ruta]
modelcairn secret delete --version n [--data-dir ruta] <nombre>
```

`secret set` lee el valor desde stdin. En una terminal interactiva real desactiva
el eco; la entrada por tubería puede terminar en un único CRLF/LF, que se elimina.
Los valores deben ser UTF-8 válido de 8 a 16.384 bytes. Versión cero crea; reemplazar
exige la versión positiva actual mediante `--version`. Metadata entrega solamente
nombre, fingerprint, versión de recurso, versión de clave y fechas. Delete exige
la versión exacta y falla mientras una Credential use el secreto. Rotate nunca
muestra material de clave.

## Clases estables de salida

| Código | Significado |
|---:|---|
| 0 | Operación completada o apply interactivo cancelado explícitamente |
| 1 | Fallo del sistema operativo, almacenamiento, integridad o ejecución inesperada |
| 2 | Uso, entrada acotada, entrada de secreto o diagnóstico de configuración |
| 3 | Conflicto de estado: versión obsoleta, ya existe, ausente o en uso |
| 4 | Plan autenticado inválido, vencido, reutilizado o que no coincide |

Los diagnósticos se escriben en stderr. Los de configuración contienen un código
estable y una ruta estructural. Las salidas y errores utilizan solo metadatos
tipados y nunca repiten valores secretos ingresados.
