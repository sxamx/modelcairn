# Semántica declarativa `apply` v1alpha1

[English](semantica-apply-v1alpha1.md)

El JSON Schema vecino define estructura y tipos. Este documento define efectos.

## Identidad y referencias

- La identidad declarativa es `(kind, metadata.name)`; no cambia al modificar
  `displayName`.
- SQLite asigna un UUID inmutable. `uid` y `resourceVersion` aparecen en export,
  pero se ignoran al crear y deben coincidir al actualizar si se proporcionan.
- Las referencias usan el nombre del recurso del tipo esperado. No se permiten
  referencias ambiguas ni ciclos Strategy→Strategy en v1alpha1.
- El `ProviderAccount` de la credencial y la `ProviderConnection` del modelo de un
  Destination deben pertenecer al mismo Provider. Validate, apply, API y publish
  rechazan el cruce con `provider_mismatch`; esta regla evita enviar una API key a
  un endpoint de otro proveedor.
- En API, el `kind` del cuerpo debe corresponder al plural de `{kind}` de la ruta;
  una discrepancia devuelve `400 kind_mismatch`.

## Operaciones

- `validate`: estructura, referencias, capacidades, URLs y reglas; cero escrituras.
- `plan`: ejecuta validate y devuelve create/update/noop/delete, siempre redactado.
- `apply`: transacción única. Crea o actualiza solo recursos declarados.
- La ausencia de un recurso en el archivo significa **no gestionarlo**, no borrarlo.
- `state: absent` solicita borrado y exige `--allow-delete`; si existen referencias,
  falla o requiere antes retirarlas en la misma transacción.
- Ningún fallo produce aplicación parcial.
- Los arrays declarados reemplazan el array completo; no se fusionan por posición.
- Campos omitidos reciben defaults al crear y conservan valor al actualizar, salvo
  que el schema permita `null` explícito para borrarlo.

## Secretos y tokens

`secretRef` referencia un secreto ya creado por consola o por
`modelcairn secret set <name>`, que lee el valor desde stdin/TTY. El YAML nunca
contiene valores secretos. Declarar `Credential state: absent` elimina el
ciphertext solo si no existen destinos activos. Los tokens nuevos se crean mediante
una operación separada que devuelve el valor una sola vez; YAML administra sus
metadatos y puede revocarlos, pero no recrea el mismo valor.

## Publicación

Aplicar una Strategy actualiza su borrador. Una operación `publish` valida el grafo,
crea una versión inmutable y cambia atómicamente la referencia activa. Route
referencia la Strategy lógica y utiliza exclusivamente su versión publicada.

## Concurrencia

La API usa `ETag`/`If-Match` sobre `resourceVersion`. CLI plan guarda las versiones
observadas en un `planToken` opaco y de corta duración, vinculado a la configuración
deseada canónica y a su vencimiento. La respuesta de plan devuelve este token y
apply debe enviarlo en `X-ModelCairn-Plan-Token`. Apply falla con
`409 version_conflict` si cambió una versión observada, si la configuración enviada
difiere de la planificada o si venció el token. El usuario debe volver a ejecutar
plan; ModelCairn no sobrescribe silenciosamente cambios web. Los tokens nunca
contienen valores secretos.
