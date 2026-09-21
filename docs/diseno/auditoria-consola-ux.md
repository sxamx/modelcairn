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

Esta iteración ordena y visualiza la configuración existente. Aún faltan
formularios guiados para conexión, modelo, clave vinculada y ruta avanzada.
El descubrimiento automático de modelos, los relays, la estimación adaptativa,
costos y Playground tampoco se dan por implementados. No se muestran datos
simulados en la consola real.
