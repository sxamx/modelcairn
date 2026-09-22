# Auditoría UX de la primera consola real

Estado: corrección implementada; pendiente validación del usuario en la VM.

## Qué falló

La primera integración trasladó colores y navegación, pero no la arquitectura de
información aprobada. Proveedores y Rutas expusieron directamente entidades
internas del motor (cuenta, conexión, credencial, egreso, destino y estrategia)
y formularios JSON. Eso convirtió acciones habituales en una tarea de
configuración avanzada y duplicó conceptos visibles.

Las capturas de uso real también mostraron:

- Fondo claro bajo paneles oscuros. La causa era un `grid-template` antiguo que
  conservaba dos filas; la nueva consola ocupaba solo la primera.
- Cabecera de tabla sin filas encima del estado vacío de Actividad.
- Títulos de `fieldset` cortados en Configuración.
- Configuración saturada de controles técnicos sin divulgación progresiva.
- Popover de JSON superpuesto al resto de los controles en la lista avanzada.

## Corrección de estructura

- Proveedores: lista de proveedores; cada detalle agrupa Resumen, Modelos,
  API keys y Configuración. Las relaciones se obtienen de los recursos reales
  sin mostrar el valor de las claves.
- Modelos: catálogo global y detalle con proveedor, capacidades y destinos
  configurados. Métricas inexistentes se indican como «Sin datos».
- Rutas: aliases y recorrido ordenado a partir de la estrategia y sus destinos.
- Las entidades internas siguen editables en «Opciones avanzadas»; no son
  pestañas principales. Crear un proveedor básico ya no obliga a crear una ruta.
- Configuración: acceso e historial al frente; red, límites de acceso y
  criptografía en un desplegable avanzado.
- Actividad y modo oscuro: corregidos en su estructura, no solo con colores.

## Límites honestos

La iteración posterior añadió alta guiada de modelos, tarifas opcionales,
métricas agregadas y una vista de demostración explícita en Datos. El recorrido
de rutas ahora se ve y permite crear, reutilizar o reordenar opciones a partir
de modelos y claves ya vinculados. Continúan pendientes los
formularios guiados de conexión y clave vinculada, el descubrimiento automático
de modelos, relays, estimación adaptativa, uptime por sondeos y Playground. Los
datos simulados solo aparecen al activar manualmente la vista de ejemplo y no
se persisten ni se mezclan con el historial real.
