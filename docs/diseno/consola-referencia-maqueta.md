# Consola: dirección visual y alcance

Estado: primera implementación parcial. La maqueta HTML proporcionada por el usuario
el 21 de septiembre de 2026 es una referencia de diseño, no una especificación
funcional ni una fuente de datos reales. No se copia su JavaScript ni su enlace
a Google Fonts.

## Dirección aprobada

- Fondo gris suave, superficies claramente separadas, acento verde petróleo y
  tipografía compacta. Tema oscuro diseñado con colores propios, no una inversión
  automática del tema claro.
- Barra lateral de 236 px, colapsable a 68 px; navegación inferior desplazable en
  pantallas pequeñas. Cabecera estable de 60 px con tema y menú de usuario.
- Marca de tres piedras apiladas, iconografía de línea, radios y espaciados
  consistentes. La inicial del avatar proviene del administrador autenticado.
- Inicio centrado en estado operativo y actividad verdadera. Las ayudas de
  configuración aparecen solo cuando son pertinentes y no obligan a crear rutas.
- Navegación con URL e historial del navegador. En las pantallas de detalle,
  pestañas, filtros y selecciones también deberán conservar contexto al volver.
- Gráficos solo cuando aporten una lectura útil; estados sin datos expresos en vez
  de curvas o métricas inventadas. Movimiento reducido cuando el sistema lo pide.

## Información y funciones

Arquitectura de información objetivo: Inicio, Proveedores, Modelos, Rutas,
Acceso API, Actividad, Datos y Configuración. En la primera implementación se
mantienen agrupadas las pantallas actuales que aún no tienen una vista nueva
funcional; no se añaden botones que lleven a secciones simuladas.

La maqueta también propone precios estimados, presupuesto, playground, sondeo de
modelos y análisis de nodos. Estos elementos requieren contratos, datos y reglas
de privacidad propios antes de mostrarse como funciones disponibles. En
particular, los precios serían configurados por el usuario y nunca equivaldrían
automáticamente a facturación real del proveedor.

## Orden de implementación

1. Base visual real: navegación, cabecera, temas, marca e Inicio.
2. Componentes compartidos: tablas, formularios, badges, estados vacíos y
   diálogos en ambos temas.
3. Separar Proveedores, Modelos y Rutas con navegación y retorno exacto al
   contexto anterior; añadir pruebas de flujo.
4. Acceso API, Actividad, Datos y Configuración; solo con información real.
5. Evaluación visual en tamaños de escritorio y móvil, accesibilidad y rendimiento
   sobre la VM de 1 GB antes de publicar un release.
