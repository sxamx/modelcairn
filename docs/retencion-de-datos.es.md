# Mapa de datos locales y retención

[English](data-retention.md)

Este inventario explica por separado cada clase de log o historial. Al ser
autohosteado, todos los datos de la aplicación permanecen en la instalación del
operador salvo que este los exporte o respalde. Configurar retención decide cuándo
se borran por antigüedad; no envía información a ningún lugar.

| Clase de datos | Para qué sirve | Qué conserva | Retención inicial | Configuración | Estado de entrega |
|---|---|---|---|---|---|
| Estadísticas de login fallido | Diagnosticar errores de acceso y posibles ataques | Minuto UTC, motivo permitido y conteo agregado | 24 horas | Cualquier duración hasta 100 años o ilimitada (`0`) | Implementado en Hito 3 |
| Auditoría administrativa | Explicar cambios privilegiados y quién los realizó | Acción tipada, identificadores de recurso, resultado, detalles permitidos y fecha | Todavía sin vencimiento automático | Política/interfaz por diseñar antes de la consola de auditoría | Almacenamiento implementado; control de retención pendiente |
| Historial de solicitudes | Examinar tráfico y resultados del gateway | Ruta/alias, tiempos, estado, tokens y latencias; sin prompt/respuesta por defecto | Sin vencimiento automático en Fase 1 | RF-202 planea retención finita o ilimitada, con 30 días como default propuesto | Registro y consulta implementados; control diferido |
| Historial de intentos/fallback | Explicar qué destino se probó y por qué continuó la ruta | Secuencia, destino, resultado, estado del proveedor, clase de error y motivo de fallback | Sigue al historial de solicitudes | RF-202 planea la misma política | Registro y consulta implementados; control diferido |
| Observaciones adaptativas de rate limit | Aprender límites y recuperación de proveedores | Alcance, reset normalizado, fuente y fecha; sin cuerpo crudo | Sin vencimiento automático en Fase 1 | RF-202 planea configuración incluida ilimitada | Observación de 429 implementada; estimador diferido |
| Cooldowns activos | Evitar enviar trabajo a un destino temporalmente no disponible | Alcance, motivo, inicio/fin y observación de respaldo | Hasta terminar o ser reemplazado | Es estado operativo, no un archivo del usuario | Creación/actualización implementada; limpieza posterior pendiente |
| Logs del proceso | Diagnosticar arranque y fallos del servicio | Eventos estructurados con plantillas de ruta; sin credenciales ni cuerpos | Lo controla la política de systemd/contenedor | Inicialmente se configura fuera de ModelCairn | Salida implementada; política del instalador pendiente |
| Backups/exports | Recuperación y portabilidad bajo control del operador | Snapshot/export explícitamente elegido | Hasta que el operador lo borre | Control total del operador | Backup completo MCB1 implementado en Hito 6 |

## Reglas comunes

- Prompts y respuestas de modelos no se almacenan por defecto.
- Contraseñas, valores de API keys, tokens de sesión/CSRF y bearer de agentes nunca
  son campos de historial.
- La futura consola debe mostrar el efecto estimado en disco antes de activar
  retención ilimitada y distinguir “ilimitado” de “desactivado”.
- Cambiar retención debe usar el mismo modelo de settings para archivo/API/web y
  limpiar mediante transacciones pequeñas sin bloquear solicitudes.
- Cada retención afecta solo su clase. Borrar estadísticas de login, por ejemplo,
  no debe borrar el historial de rendimiento de proveedores.

Este mapa será la fuente de verdad para los futuros controles de retención. Otro
hito podrá precisar defaults pendientes después de benchmarks representativos de
almacenamiento, pero deberá registrar la decisión aquí y en el contrato ejecutable.
