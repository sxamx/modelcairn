# Ejecución administrativa v1

[English](admin-runtime-v1.md)

Estado: especificación implementada; verificación final del hito pendiente.
Precisa el contrato de sesiones y ADR-0005.

## Transporte y despliegue

`publicOrigin` configura explícitamente esquema, host y puerto vistos por el
navegador. Rechazar credenciales, rutas distintas de `/`, queries, fragmentos y
comodines. Normalizar puertos predeterminados y comparar orígenes completos,
nunca sufijos. Validar Host contra ese origen; no deducirlo de cabeceras recibidas.

HTTPS puede terminar directamente o en un reverse proxy. En ese segundo modo,
solo pares incluidos en `trustedProxyCidrs` llegan a los handlers administrativos;
el proxy sobrescribe cabeceras de forwarding y envía un único
`X-Forwarded-Proto: https`. Rechazar metadata contradictoria o malformada. v1
admite un salto de proxy; cadenas adicionales requieren otro contrato explícito.
Tailscale es una opción de despliegue, no una dependencia del protocolo.

La IP del cliente es la del socket. Con un proxy confiable de un salto se admite
una IP válida en `X-Forwarded-For`, nunca listas. Una IP privada no otorga confianza
automática. HTTP de desarrollo requiere listener y origen público de loopback.
Las sesiones HTTPS siempre llevan cookie Secure.

## Frontera del navegador

Exigir Origin exacto en login y todas las mutaciones administrativas, incluido
logout. Origen ausente, `null`, duplicado o ajeno devuelve 403. La CLI online envía
el origen configurado y usa el mismo flujo de sesión/CSRF.

`GET /session/me` es la excepción explícita para recuperar CSRF: exige sesión
válida, rechaza Origin ajeno si está presente y exige Origin coincidente o
`Sec-Fetch-Site: same-origin`. Toda respuesta administrativa y de identidad usa
`Cache-Control: no-store`; CORS permanece deshabilitado. Este GET puede rotar
estado de seguridad; GET no modifica recursos de negocio.

El resto de solicitudes autenticadas exige CSRF ligado a sesión. Valores aleatorios
de 32 bytes, hashes persistidos y comparación constante. Conservar exactamente un
hash previo por hasta 60 segundos. Tres recargas rápidas pueden desplazar el token
de una pestaña antigua; ante rechazo CSRF el cliente consulta `/session/me` y
reintenta una vez. El rechazo no ejecuta mutaciones. Sesión vencida/revocada: 401.

## Autenticación acotada

La creación acepta desde 12 caracteres Unicode hasta 1024 bytes UTF-8, sin recortar
ni normalizar. Nunca poner contraseña en argv, logs, auditoría o errores. Aplicar
mínimos Argon2id de ADR-0005, sal nueva de 16 bytes y hash de 32 bytes. Antes de
reservar memoria validar PHC, versión y parámetros. Política inicial:
m=19456..65536 KiB, t=2..6, p=1; la calibración respeta estos límites. Un registro
fuera de política falla cerrado.

Defaults configurables propuestos, sujetos a medición representativa:

- Una derivación simultánea, incluida verificación ficticia; sin cola de espera.
  Autenticación ocupada devuelve 429 y Retry-After.
- Cubeta global: 30 intentos/minuto, ráfaga 5; por IP confiable del cliente:
  5 intentos/minuto, ráfaga 3. Ambos límites se evalúan antes de derivar.
- Máximo 1024 clientes; vencen tras 15 minutos inactivos. Si se llena, rechazar
  entradas nuevas hasta liberar por vencimiento; no expulsar límites activos.
- Fallos imponen espera por cliente de 1, 2, 4, 8, 16 y hasta 30 segundos. Informar
  Retry-After sin mantener solicitudes dormidas. Éxito limpia la espera, no rellena
  cubetas. Reiniciar reinicia estos límites en memoria; persistencia de throttling
  entre reinicios requiere límites en el perímetro de despliegue.

Usuarios inexistentes ejecutan derivación ficticia con la política actual y
devuelven el mismo estado/cuerpo que contraseña incorrecta; no se promete duración
idéntica. JSON de login limitado a 16 KiB; rechazar claves duplicadas/desconocidas,
UTF-8 inválido y tipos de contenido ajenos a JSON antes de procesar credenciales.

## Identidad local y persistencia

Bootstrap obtiene propiedad exclusiva, exige ausencia de administrador y nunca
sobrescribe uno existente. Reset exige permisos locales del servicio y servicio
detenido, conforme al bloqueo offline. Reemplazo de contraseña, incremento de
auth_version, invalidación de sesiones y auditoría exitosa confirman juntos.
No hay bootstrap/reset HTTP sin autenticar.

Al crear sesión después de verificar contraseña, volver a comprobar auth_version:
el hash se calcula fuera de la transacción de escritura y no debe competir con
reset. Cada solicitud autorizada comprueba revocación y vencimiento; solicitudes
rechazadas no extienden inactividad. Guardar vencimiento absoluto, last_seen y la
duración de inactividad vigente al emitir cada sesión; el vencimiento efectivo es
el menor entre absoluto y last_seen más esa duración capturada. Cambios de settings
afectan solo sesiones nuevas: no reviven sesiones vencidas ni alargan las existentes.
Añadir la duración capturada mediante migración nueva; revocar filas anteriores
sin duración conocida en vez de inferir su política. La limpieza no sustituye esa
comprobación al acceder.

Configuración de despliegue separada del YAML de proveedores: origen público,
listener/TLS/proxies, duraciones de sesión y límites de login. Todos tendrán lectura
y edición desde consola y archivos documentados. Cambios de transporte/listener
requieren reinicio, mostrando valores pendientes y efectivos. Setup local aporta
valores necesarios para llegar a la consola. Definir el esquema exacto es requisito
previo de implementación; no debe quedar como flags ocultos.

## Transacción y representación HTTP

Añadir allowDelete tanto a plan como a apply. El emisor expone vencimiento del token;
los handlers no decodifican tokens opacos. Cambios y appliedAt provienen del resultado
de la transacción, nunca de otra lectura posterior. Autenticar antes de consumir
planes; versiones detectan escrituras de otras sesiones autorizadas.

If-Match obligatorio ausente: 428; malformado: 400; obsoleto: 412. Conflictos de
referencias: 409. ETags fuertes con versión entre comillas; rechazar comodines y
listas en mutaciones. Crear no sobrescribe nombres existentes. Escrituras HTTP
reutilizan validación del grafo y persistencia atómica, no SQL individual directo.

Auditoría con acciones tipadas para bootstrap, reset, creación/logout de sesión y
emisión/revocación de agentes. Excluir contraseñas, PHC, cookies, CSRF, bearer,
cuerpos y texto arbitrario del cliente. Las estadísticas de login fallido se
agregan en buckets por minuto con motivos permitidos. La retención inicial de 24
horas acota el crecimiento; elegir retención ilimitada cambia explícitamente ese
límite por historial largo sin aumentar filas por minuto.

Pruebas de conexión y publicación de estrategias corresponden al Hito 4; antes de
implementarlas no pueden devolver éxito ficticio. Backup completo: Hito 6.
