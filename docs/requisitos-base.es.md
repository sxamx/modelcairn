# Requisitos base de ModelCairn

- Estado: borrador priorizado
- Convención: `MUST` obligatorio, `SHOULD` recomendado, `MAY` opcional

## Prioridad P0 — primera ruta útil y segura

- **RF-001 MUST:** exponer `POST /v1/chat/completions` con solicitud no streaming y
  streaming, dentro de una matriz explícita de compatibilidad.
- **RF-002 MUST:** autenticar cada cliente mediante una identidad de agente o
  integración revocable.
- **RF-003 MUST:** permitir crear proveedor, conexión de proveedor, modelo,
  credencial, egreso, destino, ruta lógica y estrategia sin modificar código.
- **RF-004 MUST:** guardar las API keys únicamente en el nodo principal, cifradas
  en reposo y redactadas en todas las salidas.
- **RF-005 MUST:** vincular cada credencial con un egreso y no cambiar esa afinidad
  automáticamente.
- **RF-006 MUST:** seleccionar un destino compatible y ejecutar fallback solamente
  cuando la política de error lo permita.
- **RF-007 MUST:** aplicar un presupuesto total de intentos y tiempo por solicitud.
- **RF-008 MUST:** conservar la semántica de streaming; no reiniciar silenciosamente
  en otro destino después de haber entregado contenido al cliente.
- **RF-009 MUST:** registrar decisiones, tiempos y errores sin almacenar prompts ni
  respuestas por defecto.
- **RF-010 MUST:** permitir configurar la primera ruta desde una consola web
  comprensible y desde una interfaz declarativa documentada.
- **RF-011 MUST:** ofrecer health y readiness diferenciados y diagnóstico básico.
- **RF-012 MUST:** realizar backup, restauración y migración verificables.
- **RF-013 MUST:** validar endpoints de proveedor y bloquear accesos de red no
  autorizados, permitiendo destinos privados solo mediante política explícita.

## Prioridad P1 — operación adaptativa y multinodo

- **RF-101 MUST:** registrar relays y comprobar identidad, salud, versión y egreso.
- **RF-102 MUST:** transportar conexiones mediante un relay sin persistir secretos
  ni contenido en él.
- **RF-103 MUST:** mostrar topología, carga, proveedores, destinos y cantidad de
  credenciales asociadas sin revelar secretos.
- **RF-104 MUST:** observar `429`, timeouts, latencia, tokens y recuperación por
  destino.
- **RF-105 MUST:** estimar capacidad y recuperación con un nivel de confianza y
  una explicación accesible.
- **RF-106 MUST:** dar prioridad a límites oficiales y reglas del operador frente a
  inferencias estadísticas.
- **RF-107 SHOULD:** distribuir solicitudes concurrentes entre destinos elegibles
  sin sobrecargar deliberadamente uno solo.
- **RF-108 MUST:** permitir al operador reasignar un egreso con advertencia,
  validación y auditoría.

## Prioridad P2 — experiencia y extensibilidad

- **RF-201 MUST:** proporcionar una PWA instalable y adaptable a móvil.
- **RF-202 MUST:** ofrecer métricas históricas, eventos detallados, filtros y
  retención configurable, incluida conservación indefinida, con protección y
  advertencias ante crecimiento de disco.
- **RF-203 SHOULD:** proporcionar un editor visual de estrategias con validación,
  simulación, publicación y rollback.
- **RF-204 MUST:** permitir adaptadores adicionales sin reescribir el router.
- **RF-205 SHOULD:** incorporar un endpoint compatible con Anthropic mediante un
  contrato de traducción y capacidades verificadas.
- **RF-206 SHOULD:** permitir almacenamiento local de contenido solo mediante una
  política futura explícita y separada de las métricas.

## Requisitos no funcionales por cerrar con mediciones

- **RNF-001:** la instalación de referencia debe funcionar en una VM Linux de 1 GB
  sin intercambio sostenido ni terminación por falta de memoria.
- **RNF-002:** el relay debe usar una fracción pequeña y medible de los recursos del
  nodo principal.
- **RNF-003:** ninguna pérdida de conectividad puede causar reintentos ilimitados.
- **RNF-004:** las operaciones administrativas críticas son auditables.
- **RNF-005:** la instalación, actualización y restauración deben ser reproducibles.
- **RNF-006:** la API no debe prometer compatibilidad para capacidades no probadas.
- **RNF-007:** la consola básica debe ser utilizable sin comprender la arquitectura
  interna y los controles avanzados deben permanecer accesibles.
- **RNF-008:** no existe telemetría externa incorporada.

Los umbrales numéricos de memoria, latencia añadida, concurrencia, disco y tiempo de
recuperación se fijarán después de construir un benchmark representativo. Elegirlos
sin una carga y hardware definidos produciría precisión ficticia.

## Fuera del alcance inicial

- Alojar o ejecutar modelos de IA.
- Convertirse en una plataforma general de agentes.
- Garantizar equivalencia de calidad entre modelos diferentes.
- Eludir límites, suspensiones o términos de proveedores.
- Alta disponibilidad automática del nodo principal.
- Sincronizar API keys entre instalaciones independientes.
- Almacenar prompts o respuestas de forma predeterminada.
