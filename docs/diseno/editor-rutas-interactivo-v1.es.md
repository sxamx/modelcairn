# Editor interactivo de rutas v1

[English](editor-rutas-interactivo-v1.md)

Este diseño describe la consola web real, no otra maqueta. El lienzo es el
control principal; el flujo normal no depende de un formulario fijo de recursos.

## Interacción y significado en el backend

- El nodo de entrada contiene el alias de modelo usado por las aplicaciones.
  Al crear una ruta se edita en un diálogo. En rutas existentes se muestra, pero
  su cambio queda en configuración avanzada porque afecta a los clientes.
- Puedes arrastrar un modelo de la paleta al lienzo o tocarlo para añadirlo al
  final. El diálogo del destino permite elegir una API key del mismo proveedor.
  Si hay una sola clave compatible, puede preseleccionarse, pero sigue visible.
- Arrastrar un destino cambia su prioridad; tocarlo abre sus detalles. Los
  botones subir/bajar ofrecen la misma operación sin arrastre.
- El orden vertical corresponde exactamente a la lista secuencial de destinos.
  Una conexión de respaldo significa que el router puede probar el siguiente
  destino tras un fallo apto; no promete reintentos para cualquier error.
- Las métricas de un nodo corresponden al modelo en toda esta instalación, no
  exclusivamente a esa ruta. La ausencia de mediciones se indica.

## Límites de persistencia

- Una ruta nueva permanece local hasta que Revisar llama a configuration plan
  y la confirmación aplica destinos, estrategia y ruta atómicamente.
- Al editar una ruta existente, las propuestas son locales. Guardar planifica y
  aplica juntos los destinos nuevos y el borrador de estrategia. Solo Publicar
  cambia el tráfico. Descartar no escribe nada.
- El editor no representa ramas arbitrarias ni posiciones libres: el backend
  actual implementa fallback ordenado, no un motor general de workflows. Los
  campos avanzados de estrategia siguen en configuración avanzada.

## Móvil y accesibilidad

El lienzo se apila en pantallas estrechas. Hay arrastre táctil; tocar un modelo
y usar los botones subir/bajar son alternativas sin arrastre. Las acciones de
modelos y nodos tienen nombres accesibles.

Criterios de aceptación: el orden visual equivale al orden guardado; abrir o
mover nodos no escribe inmediatamente; crear y guardar usan plan/apply; publicar
una ruta existente es explícito.
