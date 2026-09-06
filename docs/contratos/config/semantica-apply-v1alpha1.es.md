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

## Seguridad de parsing y salidas

- v1alpha1 acepta exactamente un documento YAML o JSON de hasta 8 MiB, con un
  máximo de 64 contenedores anidados. Los aliases de YAML y tags personalizados se
  rechazan antes del schema; los nodos y escalares decodificados quedan acotados
  por el input y los límites del schema.
- Las claves duplicadas y las claves YAML que no sean strings se rechazan antes de
  aplicar defaults, canonizar, calcular el digest o validar el schema.
- `ProviderConnection.baseUrl` rechaza información de usuario, fragments y query
  strings. IDs de modelo y campos descriptivos son datos, nunca instrucciones de
  formato para logs o auditoría.
- Un redactor común reemplaza valores exactos de secretos registrados en planes,
  exports, errores, logs y diagnósticos de tests. Los errores nunca repiten el valor
  escalar rechazado.
- `details_json` de auditoría se construye con una allowlist por acción de IDs,
  versiones, códigos de resultado y conteos tipados. Nunca acepta mapas arbitrarios,
  URLs, descripciones, headers ni cuerpos upstream.

## Secretos y tokens

`secretRef` referencia un secreto ya creado por consola o por
`modelcairn secret set <name>`, que lee el valor desde stdin/TTY. El YAML nunca
contiene valores secretos. Borrar una Credential nunca borra su Secret. El secreto
se elimina mediante una operación separada que falla mientras alguna Credential lo
referencie. Los tokens nuevos se crean mediante una operación separada que devuelve
el valor una sola vez; YAML administra sus metadatos y puede revocarlos, pero no
recrea el mismo valor.
Los secretos contienen entre 8 y 16.384 bytes UTF-8.
El fingerprint visible de un secreto es `mc_fp_` más base64url de los primeros 12
bytes de HMAC-SHA-256 sobre el secreto, usando una clave separada por propósito y
derivada de su versión de clave maestra. Sirve solo para comparar dentro de una
instalación, cambia al rotar la clave maestra y nunca autentica.

## Publicación

Aplicar una Strategy actualiza su borrador. Una operación `publish` valida el grafo,
crea una versión inmutable y cambia atómicamente la referencia activa. Route
referencia la Strategy lógica y utiliza exclusivamente su versión publicada.

El Hito 2 persiste y valida borradores de Provider, Destination, Strategy, Route y
los demás recursos de routing para completar el round trip de configuración. No
publica estrategias, activa rutas, contacta proveedores ni ejecuta fallback; esos
comportamientos comienzan en el Hito 4.

## Concurrencia

La API usa `ETag`/`If-Match` sobre `resourceVersion`. Un token de plan usa
`base64url(payload).base64url(mac)`, donde `mac` es HMAC-SHA-256 sobre el payload
codificado. Su clave deriva de la clave maestra activa con HKDF-SHA-256 y propósito
`modelcairn/plan-token/v1`. El payload canónico contiene versión y propósito,
installation ID, versión de clave activa, revisión de configuración, SHA-256 de la
configuración deseada, identidades y versiones observadas ordenadas (incluida la
ausencia), nonce aleatorio de 256 bits, emisión y vencimiento. Los tokens v1 vencen
a los diez minutos y se rechazan si fueron emitidos más de 30 segundos en el futuro.
Rotar la clave invalida los tokens pendientes.

Apply verifica el MAC antes de confiar en los claims, comprueba cada vínculo y el
reloj actual, y consume SHA-256 del nonce en la misma transacción que el cambio. El
token es de un solo uso, incluso para noop. Apply falla con `version_conflict`,
`plan_expired`, `plan_already_used` o `invalid_plan` sin cambiar configuración.
ModelCairn no sobrescribe silenciosamente cambios web y los tokens nunca contienen
secretos.

En el Hito 2, `config plan` escribe un archivo de plan redactado al usar `--out`.
`config apply --plan <archivo-plan> <archivo-config>` verifica su token autenticado,
vencimiento, digest de la configuración deseada y versiones observadas mientras
mantiene el bloqueo de la instalación. `config apply <archivo-config>` interactivo
crea un plan nuevo, lo muestra y exige confirmación dentro del mismo proceso
bloqueado; la automatización debe proporcionar un archivo de plan. La representación
HTTP administrativa del Hito 3 transportará el mismo token en
`X-ModelCairn-Plan-Token`.
