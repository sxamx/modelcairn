# Contrato de sesión administrativa v1

[English](sesiones-admin-v1.md)

## Inicio y almacenamiento

- Usuario inicial creado únicamente durante bootstrap local.
- Contraseña mínima de 12 caracteres, sin máximo artificial menor de 1024 bytes;
  se procesa como UTF-8 y se almacena en formato PHC con Argon2id.
- Respuesta de login uniforme para usuario inexistente y contraseña incorrecta.
- Rate limit por origen y global, con espera creciente; nunca bloqueo permanente que
  permita denegación de servicio trivial.
- El ID de sesión contiene 32 bytes aleatorios; la base guarda solo su hash.
- Login responde un `SessionContext` con administrador, expiración y token CSRF;
  `/session/me` permite obtener un token CSRF nuevo después de recargar la SPA.

## Cookie y CSRF

- Cookie `mc_session`: `HttpOnly`, `SameSite=Strict`, `Path=/`, sin `Domain`.
- `Secure` es obligatorio cuando el acceso usa HTTPS. Fuera de localhost, el
  onboarding no habilita administración HTTP insegura.
- Después de login o `/session/me`, todas las solicitudes administrativas
  autenticadas envían el token CSRF ligado a sesión como `X-CSRF-Token`; las
  mutaciones verifican además `Origin` estrictamente. La SPA mantiene el token en
  memoria. El token
  rota al iniciar sesión, al consultar `/session/me` y al cambiar privilegios. Para
  no romper otra pestaña durante una recarga, el hash anterior se acepta durante
  una ventana máxima de 60 segundos.
  No se aceptan mutaciones GET.
- CORS administrativo deshabilitado por defecto. Una futura excepción requiere
  allowlist exacta, nunca `*` con credenciales.

## Duración y revocación

- Inactividad: 30 minutos por defecto, configurable entre 5 minutos y 24 horas.
- Duración absoluta: 12 horas por defecto, máximo 7 días.
- Rotación del ID al iniciar sesión y después de cualquier cambio de privilegio.
- Logout revoca servidor y borra cookie.
- Reset o cambio de contraseña incrementa `auth_version` e invalida inmediatamente
  todas las sesiones existentes.
- Reiniciar el servicio no revive sesiones vencidas ni revocadas.

## Separación de identidades

La cookie administrativa no autentica `/v1/*`. Los tokens de agente no autentican
`/api/v1/admin/*`. Los endpoints de health no revelan configuración, nombres de
proveedor ni errores internos.

## Recuperación

`modelcairn admin reset-password` requiere ejecución local con permisos del
servicio, solicita la nueva contraseña por TTY/stdin seguro, invalida sesiones y
genera auditoría. No imprime hashes ni secretos.
