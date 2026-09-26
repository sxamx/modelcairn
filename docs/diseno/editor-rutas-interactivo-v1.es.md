# Editor interactivo de rutas v1

[English](editor-rutas-interactivo-v1.md)

Este diseño describe la consola web real, no otra maqueta. El lienzo es el
control principal; el flujo normal no depende de un formulario fijo de recursos.

## Interacción y significado en el backend

- El nodo de entrada contiene el alias de modelo usado por las aplicaciones.
  Al crear una ruta se edita en un diálogo. En rutas existentes se muestra, pero
  su cambio queda en configuración avanzada porque afecta a los clientes.
- Puedes elegir un modelo de la paleta para añadirlo al final. El diálogo del
  destino permite elegir una API key del mismo proveedor.
  Si hay una sola clave compatible, puede preseleccionarse, pero sigue visible.
- Arrastrar un destino cambia solamente la disposición visual del lienzo.
  Tocar su tarjeta abre sus detalles; los botones Subir/Bajar cambian la
  prioridad real y reordenan las conexiones.
- Las flechas corresponden exactamente a la lista secuencial de destinos,
  independientemente de dónde se coloquen sus tarjetas.
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
- Las posiciones libres se guardan en el navegador de ese dispositivo y no
  alteran el backend ni se sincronizan entre navegadores. La prioridad sí se
  guarda en la estrategia tras confirmar y publicar la ruta.
- El editor no representa ramas arbitrarias: el backend actual implementa
  fallback ordenado, no un motor general de workflows. No se muestran nodos de
  condiciones, porcentajes ni estimaciones ficticias de la maqueta de referencia.
  Los campos avanzados de estrategia siguen en configuración avanzada.

## Móvil y accesibilidad

El lienzo tiene desplazamiento horizontal y zoom en pantallas estrechas; el
botón Ajustar al ancho ayuda a orientarse. Hay arrastre táctil de tarjetas;
tocar un modelo y usar los botones Subir/Bajar son alternativas sin arrastre
para las acciones funcionales. Los botones tienen nombres accesibles.

Criterios de aceptación: las flechas y etiquetas de prioridad equivalen al
orden guardado; mover tarjetas no lo altera ni escribe en el servidor; crear y
guardar usan plan/apply; publicar una ruta existente es explícito.
