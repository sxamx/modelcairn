# Matriz de compatibilidad Chat Completions v1

[English](compatibilidad-chat-completions-v1.md)

- Estado: contrato de Fase 1
- Endpoint: `POST /v1/chat/completions`
- Contrato de esquema: [OpenAPI de datos v1](api/data-v1.openapi.yaml)

El parser ejecutable limita el cuerpo a 1 MiB y la profundidad JSON a 64 niveles,
rechaza claves duplicadas en cualquier nivel y exige UTF-8 válido. En esta primera
entrega, los campos desconocidos son errores: no existe un modo passthrough.

## Solicitud

| Campo | Fase 1 | Regla |
|---|---|---|
| `model` | obligatorio | alias de ruta lógica, no ID físico |
| `messages` | obligatorio | roles system, user, assistant y tool; developer requiere capacidad `developer-role` |
| contenido de texto | soportado | cadenas y partes de texto |
| imágenes, audio o archivos | rechazado inicialmente | error de capacidad antes de llamar proveedor |
| `stream` | soportado | `false` por defecto; `true` usa SSE |
| `tools` función | soportado condicional | solo destinos con capacidad `tools` |
| `tool_choice` | soportado condicional | se valida contra tools declaradas |
| `parallel_tool_calls` | soportado condicional | requiere capacidad declarada |
| `temperature`, `top_p` | passthrough validado | el adaptador declara rangos/soporte |
| `max_tokens`, `max_completion_tokens` | normalizado | conflicto entre ambos produce `400` |
| `stop`, `n`, penalties, `seed`, `logprobs` | futuro | el parser actual los rechaza |
| `response_format` | futuro | el parser actual lo rechaza |
| campos desconocidos | rechazados por defecto | modo passthrough futuro, explícito por conexión |

Una ruta solo es elegible si conserva todas las capacidades exigidas por la
solicitud. La interfaz mostrará por qué un destino fue excluido.

`stream_options` exige `stream: true`; `tool_choice` y
`parallel_tool_calls` exigen una lista `tools`. Declarar ambas variantes de límite
de tokens en una misma solicitud produce error. Un mensaje assistant puede tener
contenido, tool calls o ambos, pero no puede omitir ambos.

## Respuesta no streaming

ModelCairn preserva `id`, `object`, `created`, alias solicitado en `model`,
`choices`, mensajes, tool calls, `finish_reason` y `usage` cuando el proveedor los
entrega. Los IDs propios se distinguen de los request IDs upstream. Campos
adicionales permitidos se documentan y no alteran la forma requerida por clientes.

## Streaming SSE

- `Content-Type: text/event-stream`.
- Cada frame emitido cumple `data: <json>\n\n` y termina con `data: [DONE]\n\n` en
  finalización normal.
- Se preservan índices, `delta.role`, `delta.content`, deltas de tool calls y
  `finish_reason`.
- Un error posterior al compromiso cierra el stream; no inserta una respuesta JSON
  incompatible ni continúa desde otro destino.
- Usage en stream se entrega solo cuando la conexión/adaptador lo soporta y la
  petición lo solicita de manera compatible.

## Error de ModelCairn

Antes de comprometer el stream, el cuerpo usa:

```json
{
  "error": {
    "message": "Descripción segura",
    "type": "modelcairn_error",
    "code": "capability_not_supported",
    "param": "tools",
    "request_id": "req_..."
  }
}
```

Los códigos internos no incluyen URLs privadas, respuestas completas del proveedor
ni secretos. La documentación oficial de OpenAI es referencia de forma, no una
afirmación de equivalencia para funciones fuera de esta matriz.
