# Hito 3 — Identidad y plano administrativo

[English](hito-03-plan-tecnico.md)

Estado: implementación iniciada; pull request en borrador.
La [especificación administrativa](../contratos/admin-runtime-v1.es.md) concreta
transporte, autenticación y correspondencia HTTP; sus defaults esperan validación.
Prerrequisito: Hito 2 aceptado. Este documento organiza el alcance aprobado;
las propuestas indicadas requieren concretarse en los contratos antes de implementarlas.
La [propuesta de settings y auditoría](../contratos/admin-settings-v1.es.md)
detalla campos y límites; la retención del histórico de login espera decisión.

## Resultado esperado

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

El hito completo sigue pendiente: servicios de identidad/sesión,
handlers HTTP, paridad CLI, tokens de acceso, mediciones VM y aceptación integrada.
Retención de agregados de login sigue requiriendo decisión explícita. El trabajo
está publicado como PR #20 en borrador, no release fusionado. El diseño permite este
relevo concreto; no implica que todo el diseño de seguridad esté aceptado.
