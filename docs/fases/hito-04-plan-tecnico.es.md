# Hito 4 — Router vertical

[English](hito-04-plan-tecnico.md)

Estado: plan de implementación aceptado; Bloque 1 iniciado.
Prerrequisito: Hito 3 fusionado y aceptado.

Orden aprobado por el operador: demostrar primero el router con salida directa y
después añadir transportes proxy/relay dentro del mismo hito.

## Resultado

Entregar un recorrido real y determinista desde `POST /v1/chat/completions` hasta
un upstream compatible con OpenAI y de regreso. El recorrido autentica AgentToken,
resuelve el alias de ruta, selecciona solo destinos compatibles desde un snapshot
inmutable de estrategia, ejecuta fallback acotado, conserva tool calls y SSE, y
registra eventos operativos sin contenido.

Este hito demuestra el núcleo del router. No entrega el editor visual, el estimador
adaptativo, el protocolo de relay remoto de ModelCairn ni todos los dialectos de
proveedor. Esas funciones deberán ampliar estos contratos sin evitarlos.

## Fronteras fijas

- API de datos: `POST /v1/chat/completions`; administración permanece bajo
  `/api/v1/admin/*`.
- Autenticación mediante `Authorization: Bearer` y el verificador AgentToken actual.
- `model` es un alias lógico de Route, nunca el identificador físico upstream.
- Fase 1 acepta solo campos conocidos estrictos y jamás descarta silenciosamente
  parámetros no soportados.
- Prompt y respuesta se transmiten, pero no se persisten ni registran en logs.
- Cada credencial permanece fijada a su Egress configurado. El fallback nunca
  cambia esa afinidad implícitamente.
- Una solicitud usa un solo snapshot inmutable de configuración/estrategia aunque
  un administrador publique cambios simultáneamente.
- Los presupuestos son finitos: timeout total, timeout por intento, máximo de
  intentos y tamaño de body se controlan antes y durante la ejecución.
- No existe fallback después del punto de compromiso de la respuesta.

## Primer adaptador y salida soportados

El primer adaptador será `openai-chat-completions`. Une la URL base canónica con
`/chat/completions`, inyecta el secreto API key referenciado en Authorization,
reemplaza el alias lógico por el modelo físico seleccionado y valida media type y
forma de la respuesta.

La primera salida ejecutable será `direct`. HTTP CONNECT/SOCKS y el relay remoto
autenticado de ModelCairn necesitan contratos de transporte separados antes de
activarse. El grafo ya mantiene afinidad estable Credential-to-Egress para añadirlos
sin cambiar la semántica de selección del router.

## Recorrido de una solicitud

1. Aplicar parseo HTTP acotado y crear un request ID de ModelCairn.
2. Autenticar AgentToken y autorizar la Route solicitada.
3. Parsear y normalizar estrictamente Chat Completions.
4. Derivar capacidades requeridas: tools, tools paralelas o formato de respuesta.
5. Cargar Route y un snapshot inmutable de Strategy publicada.
6. Filtrar destinos desactivados, incompatibles o en cooldown antes de seleccionar.
7. Ejecutar destinos secuencialmente dentro de presupuestos de intentos y tiempo.
8. Clasificar cada resultado antes de devolver, enfriar o continuar por fallback.
9. Confirmar una respuesta normal válida o un upstream SSE validado.
10. Persistir Request, Attempts y observaciones normalizadas sin contenido.

## Invariantes de error y compromiso

La tabla de clasificación del contrato técnico de Fase 1 es normativa. Errores de
autenticación, configuración o capacidad nunca llaman al upstream; `429` confirmado,
fallos de conexión y `5xx` elegibles pueden hacer fallback; errores del request y
timeouts indeterminados después del envío no lo hacen inicialmente.

En streaming, ModelCairn espera estado upstream exitoso y media type SSE válido
antes de enviar cabeceras exitosas. Antes se permite fallback. Después de enviar
cabeceras o el primer evento downstream, lo que ocurra primero, un fallo termina
el stream y registra `partial`; jamás mezcla otra respuesta en el mismo stream.

## Bloques de entrega

1. **Contratos y simulador:** OpenAPI de datos exacta, parser estricto, upstream
   determinista y matriz de fallos.
2. **Snapshot y elegibilidad:** carga SQL, autorización de ruta, capacidades,
   orden secuencial estable y motivos de exclusión.
3. **Adaptador no streaming:** transporte seguro de secretos, normalización,
   cancelación y validación de conexión.
4. **Presupuestos y fallback:** clasificador, máquina de estados de intentos,
   cooldowns, Retry-After y deadlines finitos.
5. **Streaming:** validación/relay SSE, compromiso, deltas de tool calls, `[DONE]`,
   desconexión y resultados parciales.
6. **Persistencia operativa:** ciclo Request/Attempt atómico y observaciones de
   rate limit normalizadas, sin cuerpos de prompt/respuesta.
7. **Aceptación:** QA agrupado, suite de carreras, matriz de fallos simulada y
   benchmark VM con 1, 2, 5, 10 y 20 streams concurrentes.

Cada bloque recibe pruebas focales. El QA independiente se agrupa en la frontera
de la máquina de estados y en la aceptación final, no en cada edición pequeña.

## Criterios de aceptación

- Un AgentToken configurado completa llamadas normales y streaming mediante un
  simulador local determinista compatible con OpenAI.
- Tokens revocados, malformados o sin permiso devuelven el estado contratado sin
  contactar upstream.
- Tool calls atraviesan únicamente destinos con todas las capacidades necesarias.
- La matriz completa demuestra exactamente cuándo ocurre fallback y que ningún
  presupuesto puede formar un bucle infinito.
- Cancelar el cliente cancela rápidamente el upstream.
- Ninguna prueba, respuesta, log, export o fila operativa expone API keys, valores
  de sesión/bearer, prompts o respuestas del modelo.
- El benchmark registra RSS, swap, latencia y concurrencia exitosa; no se anuncia
  capacidad que la VM no soporte.
- Documentación, pruebas Go/race, vulnerabilidades, cross-builds y puerta vertical
  pasan antes de aceptar el hito.
