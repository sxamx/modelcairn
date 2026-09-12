# Hito 3 — Identidad y plano administrativo

[English](hito-03-plan-tecnico.md)

Estado: aceptado en la rama del hito; el pull request espera autorización de fusión.
La [especificación administrativa](../contratos/admin-runtime-v1.es.md) concreta
transporte, autenticación y correspondencia HTTP.
Prerrequisito: Hito 2 aceptado. Este documento organiza el alcance aprobado;
las propuestas indicadas requieren concretarse en los contratos antes de implementarlas.
El [contrato de settings y auditoría](../contratos/admin-settings-v1.es.md) detalla
campos y límites. Su valor inicial configurable de 24 horas y la opción ilimitada
explícita están implementados y verificados.

## Resultado esperado

Relevo actual: medición representativa y aceptación integrada. La paridad de CLI
online ya reutiliza la API administrativa para sesiones, configuración, secretos
y AgentToken. El
[ciclo de vida de AgentToken](../contratos/agent-token-lifecycle-v1.es.md) y el
[CRUD transaccional](../contratos/resource-mutations-v1.es.md) ya están implementados.

Una instalación puede crear su administrador localmente, autenticar sesiones,
administrar configuración y secretos mediante HTTP y emitir o revocar tokens de
agente. La consola corresponde al Hito 5 y las llamadas a proveedores al Hito 4.

## Orden de entrega

1. **Identidad local.** Hash y verificación Argon2id usando ADR-0005; bootstrap
   exclusivo y repetible sin reemplazar un administrador; reset local con auditoría
   e invalidación atómica de sesiones. Validar UTF-8 y límites antes de derivar
   claves; acotar parámetros PHC antes de reservar memoria. La calibración se mide
   en la VM y nunca reduce los mínimos aprobados.
2. **Sesiones y frontera HTTP.** Login, logout y sesión actual; hashes de tokens,
   expiración absoluta e inactividad, auth_version, cookies, CSRF y Origin.
   Resolver primero la política de HTTPS y proxies confiables. Limitar memoria,
   concurrencia y frecuencia de login antes del trabajo Argon2id.
3. **Administración de recursos.** Paginación, ETag y precondiciones, validate,
   plan, apply y export; metadatos, escritura y borrado de secretos. Reutilizar
   las transacciones del Hito 2. Nunca aplicar un subconjunto cuando falla una
   referencia, versión o auditoría. La CLI online usa esta API cuando el servicio
   está activo, conforme a ADR-0004; se mantiene la exclusión del modo offline.
4. **Tokens de agentes.** Emisión aleatoria de 32 bytes, entrega única, verificador
   SHA-256, permisos de rutas, expiración y revocación irreversible por identidad.
   Una respuesta perdida no permite recuperar el valor: documentar cómo revocar y
   emitir una identidad nueva. Probar separación completa de sesiones y agentes.
5. **Integración y aceptación.** Agrupar pruebas HTTP de identidad, configuración,
   secretos, errores, concurrencia y reinicio. Medir login y memoria en la VM;
   efectuar una revisión independiente del bloque y corregir los hallazgos antes
   de aceptar el hito.

## Contratos que necesitan precisión

| Tema | Evidencia actual | Resolución propuesta |
|---|---|---|
| HTTPS detrás de proxy | La cookie exige Secure, pero falta política de confianza | Origin externo explícito; aceptar cabeceras reenviadas únicamente desde proxies configurados; acceso HTTP local restringido a loopback |
| Recuperación CSRF | OpenAPI exceptúa /session/me, el texto general exige CSRF | Especificar la excepción autenticada, comprobar contexto de origen y evitar caché; mantener ventana anterior de 60 segundos |
| GET y rotación | El contrato prohíbe mutaciones GET y /session/me rota CSRF | Precisar que prohíbe mutaciones de negocio; la rotación de seguridad está expresamente definida |
| Login bajo carga | Exige límites por origen y global, sin cantidades | Proponer un único Argon2id simultáneo inicialmente, admisión y estructuras acotadas; medir antes de fijar tasas y colas |
| Identidad de origen | Origin HTTP no distingue clientes del mismo panel | Separar origen web de dirección de cliente confiable; no confiar en X-Forwarded-For arbitrario |
| Plan HTTP | Manager devuelve token/changes/configuration; API pide planToken/expiresAt | Exponer datos de emisión desde almacenamiento y mapear DTO sin reinterpretar el token en el handler |
| Apply HTTP | Manager devuelve solo error; API exige changes/appliedAt | Devolver resultado de la misma transacción aplicada, sin una segunda lectura susceptible a carreras |
| Borrados planificados | apply tiene allowDelete, plan HTTP no lo declara | Añadir la misma opción a plan y vincularla al token, como ya hace la CLI |
| Export y secretos | La API carece de export y lectura de metadatos | Completar OpenAPI para paridad funcional, manteniendo secretos como escritura sin lectura del valor |
| Auditoría de identidad | Existe auditoría tipada de recursos | Definir acciones y campos mínimos para login, reset, sesiones y emisión sin tokens ni contraseñas |

## Verificación proporcional

Las pruebas deben demostrar bootstrap concurrente con un solo ganador, reset con
revocación atómica, contraseña incorrecta y usuario inexistente sin diferencias de
respuesta, límites PHC, expiración tras reinicio, rechazo de cookie o CSRF alterados,
ventana CSRF entre pestañas y cabeceras de proxy falsificadas. La admisión de login
debe demostrar memoria acotada bajo solicitudes concurrentes y canceladas.
Comprobar que aumentar idleSeconds y reiniciar no revive una sesión vencida, y que
un plan de settings para documento A no permite aplicar documento B ni reutilizarse.

Para recursos, demostrar ETag obsoleto, referencias inválidas, plan vencido o
reutilizado, conflictos entre HTTP y CLI online, y resultados fieles a la
transacción. Para tokens, comprobar emisión única concurrente, autorización por
ruta y revocación inmediata para solicitudes nuevas. Capturar salidas, errores y
auditoría con canarios, permitiendo únicamente la entrega intencional inicial de
tokens al cliente autorizado.

Cada bloque incluye pruebas dirigidas y documentación. El QA independiente se
agrupa por entrega sustancial; cambios editoriales no disparan otra revisión.

## Pendiente antes de escribir autenticación

Concretar los límites y la configuración de despliegue en un contrato bilingüe,
alinear OpenAPI y definir migraciones nuevas si fueran necesarias. No modificar
migraciones ya publicadas. La configuración técnica de seguridad deberá tener
representación compartida para archivos y futura interfaz web; las opciones de
arranque necesarias para establecer esa interfaz deben distinguirse explícitamente.

## Revisión agrupada del diseño — 2026-09-09

La revisión independiente detectó dos vacíos concretos: cambios de inactividad podían
revivir sesiones vencidas y los planes de settings no ligaban explícitamente el
documento deseado. Los contratos ahora capturan duraciones al emitir la sesión y
ligan planes al digest del documento normalizado con consumo atómico de uso único.
Son correcciones documentales, no comportamiento implementado ni probado en runtime.
Ya están redactados el esquema estructural JSON Schema y GET/plan/apply de settings
en OpenAPI. La validación local pasó 95 casos positivos/negativos del esquema,
resolvió 103 referencias OpenAPI y comprobó coincidencia entre listas de campos.
No demuestra validación semántica, comportamiento HTTP ni de base de datos. El
verificador documental habitual comprueba la lista completa de campos sin añadir
dependencias de runtime ni afirmar validación completa de JSON Schema.
El [contrato de almacenamiento](../contratos/storage/admin-identity-v1.es.md) define
la migración de settings/sesiones sin modificar migraciones anteriores. Su contrato
SQL pasó 19 comprobaciones de actualización, límites y rollback transaccional.
La migración 0003 se instala con el ejecutor existente; pruebas Go y del contrato
cubren actualización secuencial, compatibilidad y rollback. Pruebas HTTP quedan
para implementación.
Pendientes: integración final de contratos y decisión del operador sobre retención
del histórico de login.

## Relevo a implementación — 2026-09-10

La primera entrega acotada ya está implementada en `internal/adminsettings`:
decodificador compartido, resolución consciente de omisiones, normalización del
origen canónico y validación semántica pura según contratos y JSON Schema. Es
independiente de retención de login: no expone login, activa la migración propuesta
ni elige esa política implícitamente. Es orden de dependencias, no eliminación de
requisitos del Hito 3.

Aceptación: defaults iniciales frente a omisiones conservadas al actualizar;
decodificación JSON/YAML estricta y límite de 64 KiB; límites numéricos y combinaciones
de transporte; origen/listener/CIDR canónicos; representación resuelta determinista;
errores con nombres de campos, nunca valores rechazados; pruebas tabuladas positivas
y negativas. Reutiliza dependencias y deja acceso a archivos fuera de validación
pura, por lo que prueba sin VM ni credenciales reales. El 2026-09-10 pasaron pruebas
del paquete, `go vet ./...`, todas las pruebas Go salvo la prueba de lock tras muerte
de proceso omitida intencionalmente en este entorno Windows, las 19 comprobaciones
del SQL propuesto y el
verificador documental. Es evidencia de implementación, no aceptación en VM.
Agrupar una revisión independiente con el bloque completo, sin repetirla por edición.

La revisión agrupada de implementación detectó dos defectos de representación: una
lista de proxies nil heredada podía serializarse como null, y normalizar publicOrigin
antes de comprobar su límite crudo podía aceptar una entrada demasiado grande. La
resolución ahora siempre posee una lista no nula y valida límites antes de normalizar.
Pasaron regresiones focales y la reverificación independiente; no quedó ningún
hallazgo pendiente en este bloque.

La persistencia de settings ahora lee solo la representación resuelta canónica,
la valida nuevamente y cierra acceso ante estado ausente o corrupto. La inserción
transaccional de revisión uno puede componerse con el futuro bootstrap de administrador
y su auditoría; no constituye un bootstrap independiente. Las pruebas cubren ausencia,
validación, rollback, duplicados, round trip canónico y corrupción. La actualización
versionada ya tiene aislamiento criptográfico de propósito respecto de planes de
proveedores y servicio interno plan/apply. Settings, consumo del nonce y auditoría
se confirman juntos. Las pruebas integradas cubren planes alterados y obsoletos,
reutilización, operaciones sin cambios, captura al iniciar y retorno a valores
efectivos. Pasaron toda la suite Go local y `go vet ./...`, incluida la prueba de
lock tras muerte de proceso. Falta integrar HTTP/CLI; no es aceptación en VM.

La revisión independiente del bloque plan/apply detectó que reutilizar un plan
aplicado devolvía conflicto de versión en lugar de plan_already_used. Se corrigió:
ante revisión obsoleta se autentica el token antes de consultar el nonce consumido.
La regresión comprueba el error exacto y rechaza tokens inválidos. La reverificación
independiente no encontró nuevos defectos; la suite Go y el análisis estático pasan.

La entrega siguiente incorpora Argon2id con PHC canónico y límites validados antes
de derivar, además de bootstrap y reset locales. Bootstrap confirma identidad,
settings y auditoría en una transacción y tiene un único ganador concurrente. Reset
incrementa auth_version, revoca sesiones y audita atómicamente. Los comandos solo
aceptan contraseña por TTY confirmado o stdin y validan settings antes de crear el
directorio de instalación. La frontera de login/sesión HTTP aún no está implementada.

Revisión de identidad local: se corrigió la exposición de argumentos rechazados
en errores de CLI y se adelantó la validación de username a la creación del estado.
Errores del lector de contraseña usan un código fijo. Las regresiones verifican
que un argumento con una contraseña canario no aparezca en stdout/stderr y que un
username inválido no cree la instalación. Esto no sustituye medir Argon2id en VM.

Siguiente bloque implementable: almacenamiento de sesiones, sin exponer todavía
login HTTP. Emitir 32 bytes aleatorios para sesión y CSRF, guardar solo hashes y
recomprobar auth_version dentro de la transacción tras verificar la contraseña.
Capturar idleSeconds y vencimiento absoluto al emitir; rechazar antes de actualizar
last_seen si hay revocación, expiración, versión inválida o CSRF incorrecto.
Rotar CSRF con un solo hash anterior durante 60 segundos. Probar reset concurrente,
rollback de auditoría y sesiones que no reviven al aumentar idleSeconds.
La admisión de login debe envolver toda derivación real o ficticia con un único
permiso sin cola, más los límites globales y por cliente ya especificados.
Mantener pendiente la exposición HTTP hasta decidir la retención de login fallido.

La capa de sesiones y login interno ya implementa ese relevo. Las pruebas cubren
valores bearer no persistidos, uso y logout, ventana CSRF, expiración sin avance de
actividad, reloj hacia atrás, carrera con reset, rollback de auditoría, respuesta
uniforme, backoff, cubetas, límite de clientes y una única derivación concurrente.
Falta revisión de seguridad agrupada antes de construir la frontera HTTP.

La revisión posterior corrigió la admisión de login: el permiso exclusivo ahora
se adquiere antes de consultar SQLite y se mantiene hasta terminar el intento.
Esto evita acumular intentos esperando la conexión antes del límite de concurrencia.
Una regresión bloquea SQLite y comprueba rechazo inmediato de un login ocupado.
Los tokens de sesión se limitan a 43 caracteres antes de decodificarlos.
La integración HTTP y su revisión siguen pendientes.

La frontera HTTP interna ya comprueba transporte, Host, origen, proxy confiable
de un salto y cookie única; aún falta conectarla a los handlers. Los encabezados
malformados se conservan para rechazarlos, nunca se tratan como ausentes.
Para mutaciones HTTP de ajustes se debe usar `AdminSettingsService.ApplySession`:
comprueba sesión vigente, auth_version y CSRF dentro de la misma transacción que
actividad, consumo del plan, ajustes y auditoría. La identidad auditada se deriva
de esa sesión. `Apply` queda reservado al CLI local con bloqueo de instalación.
Una validación previa en middleware sirve para rechazar temprano, pero no autoriza
una escritura posterior. La hora se lee después de adquirir los bloqueos.
Si reset confirma primero, la mutación se rechaza; si la mutación confirma primero,
reset invalida la sesión después. Los fallos revierten también actividad y nonce.
La regresión fuerza reset mientras apply espera, y comprueba rollback por fallo de
auditoría y reutilización del plan tras ese fallo. Plan/State todavía requieren
autenticación del caller; un plan nunca sustituye autorización al aplicarlo.
Reutilizar esta composición transaccional en las futuras mutaciones protegidas,
sin llamar a DB ni abrir transacciones adicionales desde sus callbacks.

El servidor ya expone el recorrido administrativo inicial: login, recuperación
de sesión/CSRF, logout y lectura/plan/apply de ajustes. Login acepta solo JSON de
hasta 16 KiB con campos únicos y conocidos; los ajustes aceptan JSON/YAML hasta
64 KiB. Las cookies aplican HttpOnly, SameSite Strict, Path raíz y Secure según el
transporte. Las respuestas son no-store y los logs registran la plantilla de ruta,
no cuerpos ni credenciales. Una instalación aún no inicializada expone health y
readiness sin registrar rutas administrativas, para permitir bootstrap local.
Una prueba integrada recorre login, rotación CSRF, lectura, plan, apply, auditoría
transaccional, logout y rechazo posterior. Falta la revisión agrupada del bloque,
la decisión/implementación de agregados de login y el resto de la API del hito.

Revisión de integración: el arranque ahora usa `listen` persistido y rechaza un
`--listen` distinto tras bootstrap. El flag sigue disponible antes del bootstrap
para health. direct-tls carga certificado/clave antes de abrir el puerto y envuelve
el listener con TLS (mínimo 1.2); un error detiene el arranque con código fijo.
La prueba conecta por HTTPS real con confianza explícita en el certificado local.
La cookie de sesión no fija Expires/Max-Age: SQLite impone los límites absoluto y
deslizante en cada solicitud. Esto evita cerrar una sesión activa al cumplirse su
primer plazo de inactividad. CSRF inválido devuelve 403; sesión inválida 401;
fallos internos de sesión 503. Logout incluye no-store.

### Relevo técnico para recursos y secretos

- `storage.AuthorizeAdminMutationTx` ya implementado devuelve Actor derivado de
  sesión y actualiza actividad dentro de la transacción del caller. No reutilizar
  Actor de middleware ni pasar callbacks de terceros. Ante cualquier error se
  revierte todo. Orden obligatorio: bloqueo de SecretStore, transacción SQLite,
  autorización, precondiciones/validación, escritura/auditoría, commit, liberación.
- Configuración: añadir `Manager.ApplySession` usando esa función al inicio del
  callback snapshot de ExecutePlan. Conservar `prepareTx` y ApplyConfigTx para
  validar el grafo completo. Devolver changes/appliedAt capturados en esa operación
  solo tras commit; emitir expiresAt desde el emisor, sin decodificar tokens en HTTP.
- CRUD de recursos: construir la operación mediante el validador de configuración
  existente y verificar nombre/tipo y ETag dentro de la misma transacción. No
  conectar Repository.Put/Delete directamente a HTTP: no validan todo el contrato
  del grafo. Crear exige ausencia; actualizar/borrar exige versión observada.
- Secretos: añadir entradas autenticadas que conserven el bloqueo existente de
  SecretStore durante autorización, escritura, auditoría, commit y registro de
  redacción. Reutilizar putTx y extraer deleteTx; no llamar Put/Delete desde un
  callback transaccional. Si falla el registro posterior al commit, cerrar acceso
  al almacén y comunicar resultado incierto; nunca anunciar rollback inexistente.
- Lecturas paginadas: consulta SQL por tipo, `id > cursor`, orden por id y límite
  1..200 más una fila para nextCursor. Cursor acotado y ligado al tipo consultado.
  No cargar List completo para luego paginar; no prometer snapshot entre páginas.
- ETags: una versión positiva entre comillas; ausencia 428, formato inválido 400,
  obsoleto 412. Los planes mantienen conflictos 409. Tokens de agentes requieren
  su servicio específico; CRUD genérico no puede emitir ni revivir verificadores.

Esta es la guía de implementación del siguiente bloque, no funcionalidades ya
entregadas. Agrupar su aceptación en pruebas de grafo, precondiciones, revocación,
rollback de auditoría y ausencia de secretos en respuestas/logs. La entrega
posterior implementa la retención configurable aprobada; este párrafo se conserva
como historial del relevo.

El primer punto de ese relevo ya está implementado: validate, plan, apply y export
de Configuration están conectados a la API administrativa. Plan devuelve expiry
del emisor y cambios redacted; apply devuelve changes/appliedAt capturados en la
operación. `Manager.ApplySession` autoriza dentro de ExecutePlan antes de preparar
el snapshot, y confirma sesión, consumo de nonce, grafo completo y auditoría en una
transacción. Una sesión revocada no consume el plan; la regresión reutiliza ese
mismo plan con una sesión nueva y confirma éxito. El flujo HTTP integrado valida,
planea, aplica y exporta un recurso sin reflejar el token. CRUD individual y
secretos, tokens de agentes y CLI online están conectados.

Las lecturas de recursos y metadatos de secretos ya exponen GET individual y lista
paginada. Las consultas filtran por tipo y `id > cursor`, solicitan límite más uno
y nunca cargan todo el catálogo; el cursor opaco está acotado y ligado a su scope.
Las páginas no prometen snapshot entre solicitudes. Los GET individuales devuelven
ETag fuerte con resourceVersion. La representación de secretos publica únicamente
nombre, fingerprint no reversible, versión y actualización; excluye valor, id
interno, versión de clave y fecha de creación. Las escrituras ya están conectadas.

Las escrituras de secretos ya implementan PUT y DELETE con Origin, sesión/CSRF y
precondiciones fuertes. Omitir If-Match crea; si el nombre ya existe exige 428.
Reemplazo/borrado requieren una versión entre comillas; formato inválido da 400 y
versión obsoleta 412. `SecretStore.PutSession/DeleteSession` mantienen bloqueo,
autorización, cifrado/borrado y auditoría hasta commit. El actor se deriva de la
sesión. Una sesión revocada no escribe. Los cuerpos JSON rechazan campos duplicados
o desconocidos y el buffer del valor se limpia al terminar. La API devuelve solo
metadatos/ETag. Un fallo excepcional de registro de redacción después del commit
cierra el SecretStore para evitar continuar con una protección incompleta.

La puerta integrada automatizada ya recorre el plano administrativo completo y
está incluida en CI. Ahora impone un máximo de 128 MiB de RSS pico en vez de solo
informar la medición. La medición VM representativa aprobó con 55.368 KiB de RSS
máximo y cero swap del proceso. La revisión independiente final terminó y bloqueó
la aceptación por el contrato de retención de login; también detectó la medición
de memoria antes no bloqueante, que ya fue corregida. Identidad, sesiones,
administración HTTP, tokens de acceso y paridad CLI ya están implementados. La
retención de agregados de login ahora es configurable, usa 24 horas inicialmente y
admite conservación ilimitada explícita. La migración 0004 y las pruebas integradas
cierran el hallazgo bloqueante de la revisión. El trabajo
está publicado como PR #20 en borrador, no release fusionado. El diseño permite este
relevo concreto; no implica que todo el diseño de seguridad esté aceptado.
