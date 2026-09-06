# Fase 1 — Fundación operable

[English](fase-01-fundacion.md)

- Estado de esta línea base: alcance y planificación aprobados para el Hito 1.
  Para el estado vigente del proyecto, véase
  [Estado y ciclo](../estado-y-ciclo-del-proyecto.es.md).
- Objetivo: demostrar una ruta completa, segura, instalable y medible antes de
  añadir aprendizaje adaptativo, relays o el editor visual

## Resultado de la fase

Una persona instala ModelCairn en una VM Linux, completa el bootstrap, abre la
consola, registra un proveedor OpenAI-compatible, una credencial y un modelo,
publica una ruta lógica y la utiliza mediante `POST /v1/chat/completions` con
streaming. Puede diagnosticarla, reiniciar el servicio y restaurar un backup sin
perder la configuración.

## Incluido

- Empaquetado inicial para una plataforma Linux de referencia.
- Bootstrap de cuenta administrativa y modo de acceso.
- Elección durante el bootstrap de inicio automático del servicio; habilitado como
  opción recomendada, pero no obligatoria.
- Login y sesión administrativa.
- Consola adaptable y estructura PWA instalable desde la primera fase.
- Persistencia embebida y migraciones versionadas.
- Almacén central de API keys cifradas y redacción de salidas.
- CRUD web y administrativo de un proveedor OpenAI-compatible, su conexión,
  credenciales, modelos, destinos, alias y una estrategia secuencial básica.
- Token revocable para un agente o integración.
- Chat Completions no streaming y streaming.
- Matriz versionada de compatibilidad de Chat Completions, incluidas herramientas
  cuando el destino declare y demuestre esa capacidad.
- Fallback secuencial previo a entregar contenido al cliente.
- Clasificación mínima de error, timeouts y presupuesto de intentos.
- Eventos y métricas locales sin prompts ni respuestas.
- Health, readiness, logs estructurados y diagnóstico de recursos.
- Exportación de configuración sin secretos.
- Backup y restauración local documentados y probados.
- Validación de endpoints de proveedor y autorización explícita de redes privadas.
- Medición reproducible de memoria en una VM objetivo.

## Excluido y diferido

- Relays de salida y topología multinodo.
- Estimador adaptativo completo; solo se capturan desde ahora los eventos que
  necesitará posteriormente.
- Editor visual de grafos; se utilizará una estrategia secuencial sencilla.
- Compatibilidad Anthropic, Responses API y traducciones complejas.
- Varios administradores, RBAC y 2FA.
- Almacenamiento de prompts o respuestas.
- Actualización automática desde la consola.
- Alta disponibilidad del nodo principal.

## Contrato mínimo de routing

- La estrategia contiene una lista ordenada de destinos compatibles.
- Cada solicitud tiene timeout total, timeout por intento y máximo de intentos.
- Un error de autenticación del cliente nunca produce fallback.
- Una solicitud inválida nunca produce fallback.
- Los errores del proveedor se clasifican mediante reglas explícitas; la tabla
  exacta de códigos se cerrará antes de implementar el router.
- Un `429` puede marcar cooldown y permitir el siguiente destino si todavía existe
  presupuesto.
- Un timeout puede permitir fallback solo si no se entregó contenido.
- Después de enviar parte del stream, ModelCairn no mezcla la continuación de otro
  modelo en la misma respuesta.
- La especificación de streaming definirá el punto de compromiso incluyendo
  headers, apertura de SSE y primer evento; después de ese punto cualquier fallo
  termina el stream de manera explícita y no activa fallback invisible.
- El resultado registra los intentos y la razón de cada transición.

## Criterios de aceptación

1. Una instalación limpia puede completarse siguiendo únicamente la documentación.
2. El operador crea y prueba una ruta desde la web sin editar código.
3. La misma ruta responde en modo normal y streaming mediante pruebas de contrato.
   La matriz declara y prueba herramientas y demás capacidades incluidas.
4. Una credencial inválida, un `429` y un timeout producen resultados trazables y
   respetan la política configurada.
5. Ninguna prueba de logs, API administrativa, exportación o error encuentra una
   API key completa.
6. Reiniciar conserva la configuración publicada y no corrompe eventos.
7. Backup y restauración se prueban en una instalación vacía.
8. La carga de referencia permanece dentro del presupuesto de memoria que se
   acordará antes de cerrar la fase.
9. Las migraciones se prueban desde cada versión de esquema incluida en la fase.
10. Una revisión independiente no mantiene defectos críticos o altos abiertos.
11. Revocar un token de agente impide nuevas llamadas; un token de agente no puede
    acceder a administración y una sesión administrativa no sustituye sus permisos.
12. Cookies, expiración de sesión, CSRF cuando corresponda, rate limiting de login
    y persistencia segura tras reinicio pasan pruebas de integración.
13. Endpoints locales, metadata cloud, esquemas, puertos, redirects y cambios de
    resolución no autorizados quedan bloqueados; un endpoint privado requiere una
    excepción explícita y auditable.
14. Una migración interrumpida falla de forma segura. La estrategia inicial será
    migración hacia adelante con backup previo obligatorio y restauración probada,
    no migraciones inversas improvisadas.

## Estrategia de pruebas

- Unitarias para validación, redacción, clasificación de errores y presupuestos.
- Contrato contra un proveedor simulado determinista.
- Integración HTTP real para administración, login, API y streaming.
- End-to-end desde una instalación vacía hasta una llamada funcional.
- Fallos inducidos: `401`, `429`, `500`, timeout, stream cortado, disco restringido
  y reinicio durante operaciones seguras.
- Seguridad focalizada: secretos en salidas, sesión, permisos de archivos, límites
  de cuerpo, entradas malformadas, SSRF, redirects, endpoints privados y
  verificación del hash resistente de la contraseña administrativa.
- Rendimiento: reposo, concurrencia gradual y retención de eventos.
- Recuperación: backup, restauración y migración.

## Puerta para iniciar implementación

Decisiones cerradas:

- Go y monolito modular como base técnica;
- Linux AMD64 y ARM64, validando primero la VM real de referencia;
- systemd como método principal y elección de inicio automático en el instalador;
- SQLite como fuente de verdad y YAML mediante aplicación/exportación explícita;
- clave maestra local, exportación sin secretos y backup completo cifrado;
- recuperación administrativa mediante CLI local;
- bloqueo de endpoints privados por defecto y excepciones explícitas.

Entregables técnicos desarrollados y aprobados por QA:

- tabla inicial de errores y contrato detallado de streaming;
- escenario, carga y umbral del benchmark de recursos;
- contratos de configuración y API administrativa;
- modelo físico inicial y plan de migraciones;
- diseño verificable de cifrado y formato de backup.

El Hito 0 produjo además el JSON Schema, OpenAPI, schema SQLite, contrato de
sesiones, matriz de compatibilidad y ADR de frontend necesarios para implementar
estos entregables sin inventar su forma durante el código.

Estos puntos se desarrollan en [Contratos técnicos de la Fase 1](fase-01-contratos-tecnicos.es.md)
y se ordenan en el [Plan de implementación de la Fase 1](fase-01-plan-de-implementacion.es.md).
El Hito 0 fue aprobado sin hallazgos críticos o altos pendientes. La implementación
puede abrirse con el Hito 1 después de preparar el repositorio limpio.

El benchmark deberá declarar hardware y arquitectura, memoria disponible después
del sistema operativo y servicios base, concurrencia, duración, patrón de carga,
pico de RSS, actividad de swap y margen reservado. El umbral se aprobará con esos
datos, no con una medición aislada en reposo.

## Matriz inicial de trazabilidad

| Requisito | Evidencia principal |
|---|---|
| RF-001, RF-008 | Pruebas de contrato normal, streaming, corte y herramientas |
| RF-002 | Integración de token, revocación y separación de permisos |
| RF-003, RF-010 | Flujo end-to-end desde consola y round-trip declarativo |
| RF-004, RF-005 | Pruebas de almacén, redacción y afinidad persistente |
| RF-006, RF-007 | Casos de errores, fallback y agotamiento de presupuesto |
| RF-009 | Inspección automática de eventos sin contenido |
| RF-011 | Pruebas de health/readiness con dependencias degradadas |
| RF-012 | Migración, interrupción, backup y restauración |
| RF-013 | Suite SSRF y excepción auditable para endpoint privado |

`health` indicará que el proceso está vivo. `readiness` exigirá persistencia,
almacén de secretos y configuración activa utilizables; la caída de un proveedor
individual afectará sus destinos, pero no hará que toda la instancia deje de estar
ready si existe una ruta administrativa operable.
