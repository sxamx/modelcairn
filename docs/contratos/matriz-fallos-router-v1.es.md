# Matriz de fallos del router v1

[English](matriz-fallos-router-v1.md)

Estado: contrato inicial del Hito 4. Esta matriz decide si el router puede intentar
el siguiente destino. Todo fallback respeta el máximo de intentos y el deadline
total; nunca altera la afinidad Credential-to-Egress.

| Resultado | ¿Contactó upstream? | Fallback | Efecto operativo inicial |
|---|---:|---:|---|
| JSON, autenticación o autorización local inválida | no | no | error seguro al cliente |
| ruta inexistente o sin destino compatible | no | no | `404` o `422` seguro |
| fallo DNS, conexión rechazada o TLS antes del envío | sí | sí | intento `transport_error` |
| timeout antes de enviar el body | sí | sí | intento `timeout_pre_send` |
| timeout o desconexión con envío indeterminado | sí | no | intento `indeterminate` |
| upstream `400`, `401`, `403`, `404` | sí | no | error de request/configuración |
| upstream `408` | sí | no inicialmente | observación, sin reintento ambiguo |
| upstream `429` confirmado | sí | sí | cooldown; respeta `Retry-After` válido |
| upstream `500`, `502`, `503`, `504` | sí | sí | fallo transitorio del destino |
| otro `4xx` o `5xx` | sí | no | fallo terminal hasta clasificación explícita |
| `2xx` con media type o cuerpo inválido | sí | sí antes del compromiso | respuesta upstream inválida |
| SSE válido antes de cabeceras downstream | sí | no aplica | compromete el stream |
| error tras comprometer respuesta o SSE | sí | no | resultado `partial`; cierra conexión |
| cliente desconectado o contexto cancelado | quizá | no | cancela upstream rápidamente |
| deadline total o máximo de intentos agotado | quizá | no | error terminal acotado |

## Reglas de seguridad

- Solo los resultados enumerados como elegibles pueden activar fallback.
- `Retry-After` es una señal acotada, no una orden para exceder el deadline.
- ModelCairn no registra cuerpos, Authorization, API keys ni URLs con credenciales.
- El simulador `internal/testupstream` conserva únicamente metadatos seguros y
  permite programar estados, cabeceras, demoras, SSE y cortes de conexión.

La distinción exacta entre “antes del envío” e “indeterminado” se implementará en
el adaptador de transporte. Hasta entonces ninguna prueba debe asumir que repetir
una solicitud ambigua es seguro.
