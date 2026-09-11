# Ciclo de vida de AgentToken v1

[English](agent-token-lifecycle-v1.md)

Estado: núcleo e interfaz administrativa implementados en el Hito 3; conexión con
la API de datos pendiente del router vertical.

## Propósito y separación

Un `AgentToken` autentica clientes de la API de datos y limita las rutas que pueden
usar. No autentica la API administrativa, no reemplaza una sesión de administrador
y una cookie administrativa tampoco puede sustituirlo. Su recurso declarativo
define `allowedRouteRefs`, vencimiento y habilitación; el valor bearer pertenece a
un ciclo de vida separado y nunca aparece en configuración, exportaciones o backup.

En v1, `enabled: false` es revocación irreversible para la identidad (`uid`) del
recurso, no una pausa. Eliminar y recrear el mismo nombre crea otra identidad y no
revive el token anterior.

## Formato y persistencia

La emisión genera 32 bytes con el CSPRNG y entrega
`mc_at_v1_<base64url-sin-padding>`. La base de datos conserva SHA-256 de la cadena
bearer completa, un prefijo de presentación no sensible, `issued_at`, `expires_at`
y `revoked_at`; nunca conserva el bearer. El prefijo contiene la etiqueta de
formato y ocho caracteres aleatorios, suficientes para reconocer credenciales sin
tratarlos como autenticadores.

El hash rápido no protege un secreto débil: es correcto porque el bearer tiene 256
bits aleatorios. La búsqueda usa su digest binario de longitud fija. No escribir bearer,
hash, cabeceras Authorization ni cuerpos en logs, auditoría, métricas o errores.

## Emisión administrativa

`POST /api/v1/admin/agent-tokens/{name}/issue` exige transporte/origen, sesión y
CSRF administrativos. Dentro del bloqueo de `SecretStore` y una única transacción:

1. volver a autorizar la sesión y obtener el actor;
2. cargar por nombre el recurso y su fila tipada;
3. exigir que exista, esté habilitado, no esté vencido y nunca haya sido emitido o
   revocado;
4. generar el bearer, calcular su verificador y actualizar con una condición que
   todavía exija estado sin emitir;
5. insertar auditoría `agent_token.issue` y confirmar.

Dos emisiones simultáneas tienen un solo ganador. La respuesta `201` lleva
`Cache-Control: no-store`, el bearer exactamente una vez y su estado seguro. Si la
respuesta se pierde, el valor no se puede recuperar: el operador revoca esa
identidad y crea otra. No se usa `If-Match`: la precondición es el estado de emisión
de una sola vez, separado de la versión declarativa.

## Revocación administrativa

`POST /api/v1/admin/agent-tokens/{name}/revoke` aplica la misma frontera y
transacción. Un recurso inexistente devuelve 404. Marca `revoked_at` incluso si aún
no fue emitido, de modo que una revocación preventiva también sea irreversible.
Repetir la revocación devuelve 204 sin cambiar el instante original; se audita solo
la primera transición. No requiere `If-Match` porque es monotónica e idempotente.

Deshabilitar el recurso por CRUD usa la misma propiedad irreversible. Borrar un
recurso revocado no borra auditoría ni permite que su bearer vuelva a ser válido.

## Estado seguro para administración

Las representaciones administrativas de `AgentToken` pueden añadir `tokenStatus`:

- `state`: `unissued`, `active`, `expired` o `revoked`;
- `prefix`, `issuedAt`, `expiresAt` y `revokedAt`, presentes cuando corresponda.

Nunca incluyen `verifier_sha256`. `revoked` prevalece sobre `expired`; sin emisión
ni revocación el estado es `unissued`. El vencimiento se evalúa en cada solicitud,
no depende de una limpieza periódica.

## Autenticación de datos

La API de datos acepta exclusivamente `Authorization: Bearer <token>`, una sola
cabecera y sin esquemas alternativos. Antes de buscar en SQLite valida longitud,
etiqueta y base64url canónica. Calcula SHA-256, carga por verificador y rechaza con
401 valores desconocidos, revocados, vencidos o pertenecientes a recursos
deshabilitados/eliminados. Estos casos comparten respuesta y no revelan cuál falló.

Después verifica que la ruta resuelta esté en `allowedRouteRefs`; falta de permiso
devuelve 403. La autorización y la captura de identidad/ruta para la solicitud se
hacen antes de contactar proveedores. Rotación automática, múltiples bearers por
identidad y permisos distintos de rutas quedan fuera de v1.

## Aceptación agrupada

- bearer aleatorio con formato canónico y entrega única;
- solo un ganador bajo emisión concurrente y rollback si falla auditoría;
- bearer ausente de SQLite, logs, errores, DTO y exportación;
- revocación previa o posterior a emisión irreversible e idempotente;
- vencimiento y borrado bloquean solicitudes nuevas;
- 401 uniforme para token inválido/revocado/vencido y 403 para ruta no permitida;
- separación completa entre sesión administrativa y bearer de agente.
