# Fase 1 — Contratos técnicos

[English](fase-01-contratos-tecnicos.md)

- Estado: borrador técnico verificable
- Depende de: ADR-0003, ADR-0004 y Fase 1 — Fundación operable
- Regla: este documento define comportamiento; el código no puede cambiarlo
  silenciosamente

## 1. Contrato HTTP inicial

### Superficies

- Datos: `POST /v1/chat/completions`.
- Administración: `/api/v1/admin/*`, autenticada mediante sesión.
- Operación: `/healthz` comprueba proceso; `/readyz` comprueba persistencia,
  secretos y configuración activa; `/metrics` no será público por defecto.
- La API de datos usa tokens de agente, separados de la sesión administrativa.

La compatibilidad será declarada mediante una matriz versionada. La Fase 1 admite
mensajes de texto, parámetros comunes, streaming SSE y tool calls de función cuando
el destino los declare. Parámetros desconocidos no se descartarán silenciosamente:
se enviarán solo si el adaptador los admite; en otro caso se responderá error de
compatibilidad antes del primer intento.

### Límites iniciales configurables

El esquema incluirá tamaño máximo de body, cabeceras, timeout total, timeout por
intento, máximo de intentos y máximo de conexiones. Los valores finales se fijarán
con pruebas; siempre existirán límites seguros predeterminados.

## 2. Errores, reintentos y fallback

| Situación | Acción predeterminada | Fallback |
|---|---|---|
| Token de agente ausente/inválido/revocado | responder `401` | nunca |
| Agente sin permiso para la ruta | responder `403` | nunca |
| JSON, parámetros o capacidades inválidas | responder `400` | nunca |
| Ruta o alias inexistente | responder `404` | nunca |
| Body demasiado grande | responder `413` | nunca |
| Proveedor responde `401` | bloquear la credencial hasta prueba o rotación manual | sí, hacia otra credencial autorizada |
| Proveedor responde `403` por permiso comprobado de cuenta/modelo | marcar esa combinación incompatible | solo si el adaptador clasifica la causa con seguridad |
| Proveedor responde `403` por safety/policy o causa desconocida | devolver el rechazo sin penalizar la credencial | no |
| Proveedor responde `429` | registrar alcance conocido, aplicar cooldown conservador | sí |
| Proveedor responde `408`, `5xx` o falla la conexión | clasificar como transitorio | sí, dentro del presupuesto |
| Error permanente del request en proveedor (`400/404/422`) | devolver error compatible y registrar | no por defecto |
| Timeout antes de conectar o enviar el body | cancelar intento | sí, antes del compromiso del stream |
| Timeout después de enviar el body, sin respuesta confirmada | resultado indeterminado | no por defecto; solo con idempotencia o política explícita |
| Timeout total o intentos agotados | terminar solicitud | no |
| Cancelación del cliente | cancelar upstream y registrar | no |

La tabla es predeterminada, no universal: un adaptador puede refinarla cuando el
proveedor documente otra semántica. Nunca se reintenta indefinidamente. Se
respetará `Retry-After` válido y metadata oficial de rate limit; en su ausencia se
usará cooldown conservador con jitter y máximo configurable.

Cada observación `429` tendrá un alcance: proveedor, cuenta de proveedor,
credencial, conexión, modelo o combinación concreta. Los headers oficiales y el
adaptador tienen precedencia. Si el alcance es desconocido, se enfría la credencial
en todas sus combinaciones y se limita la prueba del proveedor; no se intenta la
misma credencial inmediatamente mediante otro egreso. Las credenciales podrán
agruparse bajo una cuenta lógica para compartir límites conocidos sin almacenar
credenciales de inicio de sesión de esa cuenta.

Un resultado indeterminado significa que el proveedor pudo procesar la solicitud
aunque ModelCairn no recibiera la respuesta. La trazabilidad advertirá el riesgo de
duplicación y ningún fallback automático asumirá que “timeout” equivale a “no
procesado”.

Cada intento tendrá un identificador interno. Se conservará el request ID del
proveedor cuando exista, aplicando redacción. El cliente recibirá un ID de solicitud
de ModelCairn para diagnóstico.

## 3. Contrato de streaming

- ModelCairn no enviará `200` ni abrirá el stream hasta que un destino haya
  aceptado la solicitud y exista una respuesta upstream válida.
- El **punto de compromiso** ocurre al enviar al cliente las cabeceras exitosas o
  el primer byte/evento SSE, lo que suceda primero.
- Antes del compromiso se permite fallback conforme a estrategia y presupuesto.
- Después del compromiso no se cambia de modelo ni se mezcla otra respuesta. Un
  fallo termina el stream y se registra como parcial.
- Se preservarán orden, índices, deltas de contenido, deltas de tool calls,
  `finish_reason` y terminador `[DONE]` cuando el dialecto lo utilice.
- Cancelar la conexión cliente cancela el contexto upstream rápidamente.
- La métrica distingue TTFT del proveedor, sobrecarga del gateway, duración total
  y respuesta parcial.

Esta política evita respuestas híbridas difíciles de detectar. La referencia de
compatibilidad se contrastará contra la documentación oficial vigente de Chat
Completions antes de implementar cada campo.

## 4. Configuración declarativa

### Flujo

`modelcairn config validate archivo.yaml` valida sin cambiar estado.
`modelcairn config plan archivo.yaml` muestra diferencias redactadas.
`modelcairn config apply archivo.yaml` aplica una transacción y audita el cambio.
`modelcairn config export` produce YAML sin secretos.

### Reglas

- El documento incluye `apiVersion` y `kind`.
- Los recursos usan IDs estables y etiquetas humanas modificables.
- Las referencias deben existir o crearse en la misma transacción.
- Los secretos se referencian por ID; nunca se exporta su valor.
- Un apply inválido no cambia parcialmente la configuración.
- La consola usa los mismos esquemas y validadores que CLI/API.
- La publicación de una estrategia crea una versión inmutable; editar crea un
  borrador nuevo.

Recursos de Fase 1: `Provider`, `ProviderAccount`, `ProviderConnection`, `Credential`, `Egress`,
`Model`, `Destination`, `Strategy`, `Route` y `AgentToken`.

## 5. API administrativa

- CRUD versionado para los recursos de configuración.
- Operaciones separadas para validar, probar, publicar, pausar y archivar.
- Los secretos se aceptan al crear o rotar, pero nunca se devuelven completos.
- Las modificaciones usan control optimista de versión para evitar sobrescrituras.
- Toda mutación produce evento de auditoría con actor, acción, recurso, fecha y
  resultado, sin datos secretos.
- Los endpoints privados requieren una bandera explícita y una advertencia
  confirmada; redirects y cada resolución DNS se vuelven a validar.
- Las pruebas de conexión tienen timeout, límites y no permiten convertir el
  servidor en un escáner de red.

## 6. Modelo físico y migraciones

SQLite será propiedad exclusiva del proceso principal y no se ubicará en un
filesystem de red. Grupos iniciales:

- configuración versionada y versiones publicadas;
- credenciales cifradas y afinidades;
- identidades de agente y hashes de token;
- solicitudes, intentos y decisiones;
- observaciones y agregados métricos;
- sesiones administrativas y auditoría;
- versión de esquema y trabajos de mantenimiento.

La propiedad entre servicio y CLI offline, junto con el protocolo de llavero
resistente a interrupciones, se especifica en
[Propiedad de SQLite y durabilidad del llavero](../contratos/storage/propiedad-y-llavero-v1.es.md).

Cada migración tiene ID monotónico, checksum y transacción cuando SQLite lo
permita. Antes de una migración destructiva se crea y verifica un backup. Si una
migración falla, el servicio no queda ready y muestra una instrucción de
recuperación; no intenta continuar con un esquema parcialmente compatible. La
estrategia inicial es roll-forward o restauración del backup, no migraciones
inversas improvisadas.

## 7. Criptografía y backups

No se diseñarán algoritmos propios.

- Las contraseñas administrativas usan Argon2id con los parámetros mínimos y
  calibración definidos por ADR-0005.
- Las API keys usan XChaCha20-Poly1305 con nonce único y contexto asociado al ID y
  versión del secreto.
- La clave maestra se genera con el CSPRNG del sistema, vive fuera de SQLite y
  tiene permisos exclusivos para el servicio.
- Los tokens de agente se generan aleatoriamente, se muestran una vez y se guarda
  SHA-256 del token de 256 bits.
- El backup completo usa el perfil MCB1 streaming sobre age v1 y restaura en una
  generación atómica con clave maestra nueva.
- Contraseñas y claves no se pasan por argumentos visibles de procesos ni se
  escriben en temporales sin protección.

Las decisiones completas están en ADR-0005 y en la especificación MCB1. El
benchmark podrá aumentar parámetros Argon2id, pero nunca reducirlos bajo el mínimo
documentado.

## 8. Benchmark de recursos

### Entorno registrado

- distribución y kernel Linux, arquitectura, vCPU y memoria total;
- servicios residentes y memoria disponible antes de iniciar ModelCairn;
- versión exacta del ejecutable y configuración;
- swap configurada, pero el éxito no puede depender de swap sostenida.

### Escenarios

1. Reposo durante 15 minutos.
2. Consola y consultas métricas.
3. 1, 2, 5, 10 y 20 streams concurrentes contra proveedor simulado.
4. Fallback con timeout y `429`.
5. Escritura sostenida de eventos y compactación.
6. Backup y restauración.

Se medirán RSS estable y pico, CPU, goroutines, descriptores, tamaño/latencia de
SQLite, actividad de swap, latencia añadida y pérdidas. Como presupuesto inicial
de diseño —que el benchmark puede corregir— el proceso principal no deberá superar
256 MiB RSS estable ni 384 MiB de pico en la carga de referencia, reservando el
resto para SO, caché y otros servicios. No se prometerán 20 streams si la VM no los
sostiene; se publicará la capacidad realmente medida.

## 9. Evidencia para cerrar la Fase 1

- matriz de compatibilidad y suite de contrato;
- pruebas unitarias, integración HTTP y end-to-end;
- pruebas de filtración de secretos y SSRF;
- informe reproducible del benchmark;
- restauración funcional desde backup cifrado;
- guía de instalación limpia y desinstalación no destructiva;
- QA independiente sin hallazgos críticos o altos pendientes.
