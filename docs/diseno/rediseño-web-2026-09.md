# Rediseño de la interfaz web: estructura aprobada

Estado: arquitectura de información, dirección visual y sistema de tokens aprobados por el mantenedor el 22 de septiembre de 2026. Implementación en curso por bloques. Este documento no modifica el contrato del backend ni sustituye los ADR existentes.

La [propuesta visual 01](../prototipos/direccion-visual-web-01.html) y la [galería de tokens](../prototipos/sistema-visual-web-v1.html) son las referencias aprobadas. La implementación mantiene la PWA y mejora el móvil; los datos de ejemplo de los prototipos no se trasladan a producción.

## Objetivo

Hacer que una persona pueda configurar y observar ModelCairn sin conocer sus entidades internas. La web debe mantener el contexto al navegar y editar; lo avanzado sigue disponible sin dominar el flujo principal. «Consola» en documentación anterior significa interfaz web, no CLI.

## Mapa de navegación

```text
Inicio
  Estado · solicitudes/errores/tokens · actividad reciente · pendientes contextuales
Proveedores
  Lista · añadir
  Proveedor
    Resumen · Conexiones · API keys · Modelos
Modelos
  Catálogo global · filtros · ficha de modelo
Rutas
  Lista · crear
  Ruta
    Resumen · editor visual · revisar borrador · publicar
Aplicaciones (antes «Acceso API»)
  Clientes/agentes · token de entrada a ModelCairn
Actividad
  Solicitudes · filtros · detalle de intentos
Datos
  Visión general · por proveedor · por modelo · por ruta
Configuración
  Apariencia · acceso/seguridad · historial/retención · red/instalación · avanzado
```

## Relaciones y reglas

- Una API key de proveedor autoriza **salidas hacia el proveedor**. Un token de Aplicaciones autoriza **entradas a ModelCairn**. Nunca se presentan como la misma clase de clave.
- Al crear proveedor se pide identificador técnico obligatorio y nombre visible opcional. Si falta el segundo, se muestra el primero. Se abre el detalle del proveedor, no se fuerza una ruta.
- Conexiones, claves y modelos de un proveedor se crean y editan desde su detalle. El catálogo global de Modelos reúne todos los proveedores.
- Cambiar una clave reemplaza su valor mediante una interacción contextual y confirmación; no se vuelve a mostrar el secreto anterior.
- «Volver» restaura proveedor, pestaña y, cuando corresponda, filtros. Abrir un panel para editar no debe perder la pantalla de origen.
- Datos presenta agregados y tendencias; Actividad presenta eventos individuales. Un detalle ampliado puede abrirse contextualizado sin obligar a reconstruir la búsqueda.
- Rutas distingue claramente borrador guardado de versión publicada. El backend ya soporta parte del recorrido, pero creación visual completa y operaciones atómicas requieren trabajo adicional.
- Hasta completar el editor visual, la sección principal de Rutas muestra «Próximamente». La vista técnica anterior permanece detrás de un desplegable para administrar rutas existentes; esta decisión no altera su ejecución en el gateway.
- Toda cifra se identifica como observada, estimada o no disponible. Los wireframes pueden usar ejemplos, pero la web real no mezclará ejemplos con datos del usuario.
- El resumen de un proveedor puede graficar solicitudes por modelo a partir del historial retenido. No presenta esa actividad como uptime: la tarjeta de uptime dice «Sin sondeos» hasta que se implementen comprobaciones independientes.

## Etapas de implementación

1. Referencias visuales y mapa aprobados; conservarlos como checkpoint.
2. Base compartida de la web real: tokens de claro/oscuro, tipografía, superficies, navegación móvil y PWA. Esta etapa no equivale a una pantalla completamente rediseñada.
3. Migrar pantallas y recorridos reales por bloques: Inicio, Proveedores/Modelos, Aplicaciones, Actividad/Datos, Rutas y Configuración. Conservar el backend y no fingir métricas.
4. Verificar accesibilidad, teclado, móvil/PWA, contraste, pruebas automatizadas y rendimiento en la VM de 1 GB antes del despliegue.

El wireframe independiente muestra navegación, jerarquía, acciones y estados, no respuestas de backend. Los controles aún no implementados siguen siendo propuestas hasta completar su bloque.

## Vista previa del bloque implementado

Desde `web/`, ejecutar `npm run dev -- --host 127.0.0.1 --port 4173 --strictPort` y abrir `http://127.0.0.1:4173/preview.html`. Esta página carga el mismo componente y CSS de la web real, pero sustituye solo las respuestas administrativas por datos ficticios. Todas las escrituras simuladas responden con error; no se conecta a la VM ni contiene claves. El aviso visible identifica la simulación. `preview.html` y `preview-mock.js` son exclusivos del servidor de desarrollo y no entran en el paquete de producción de Vite.

La preview permite revisar la base visual ya implementada, no equivale a que todas las pantallas estén terminadas ni reemplaza una prueba final en móvil/PWA real.

## Pendientes de implementación por bloques

- Flujo único y recuperable de proveedor → conexión → clave → modelo → prueba.
- Edición y eliminación de conexiones y vínculos sin exponer secretos.
- Filtros de modelos según datos realmente disponibles; contexto máximo y precio no se inventan.
- Alta visual de rutas nuevas, validación y publicación segura.
- Estados vacíos, errores recuperables, pantalla estrecha/PWA y navegación por teclado.

## Móvil y PWA

- En móvil, usar navegación inferior para Inicio, Proveedores, Rutas y Datos; «Más» abre Modelos, Aplicaciones, Actividad y Configuración. Los detalles conservan pestañas desplazables y botones táctiles accesibles. Evitar tablas que requieran desplazamiento horizontal para acciones básicas.
- Respetar áreas seguras de iPhone, contraste en ambos temas, teclado, tamaño de toque, orientación y preferencia de movimiento reducido.
- La revisión visual detectó texto pequeño y demasiado pesado en Actividad. En la dirección visual se elevó el cuerpo a 15 px, las filas principales a 15 px semibold y el secundario a 14 px; no se usará texto de 11–12 px como contenido principal. Verificar en pantallas de baja densidad y con zoom, además de escritorio y móvil.
- Usar color semántico con texto redundante: verde para resultado correcto, ámbar para limitación o fallback recuperado, rojo para solicitud finalmente fallida y azul para información cuando haga falta. Un `429` intermedio no pinta de rojo toda la solicitud si otra opción resolvió la petición. La codificación debe funcionar en claro, oscuro y sin distinguir colores.
- La aplicación real ya tiene manifiesto, modo `standalone` y service worker. El rediseño debe mantenerlos y verificar instalación en iOS/Android; la maqueta local `file://` no es una PWA instalable.
- El shell puede estar disponible sin red, pero los datos de administración no se presentan como actuales estando desconectado. El service worker no debe guardar respuestas de API ni secretos; se comprobarán actualizaciones, iconos instalables y color del sistema en ambos temas antes del despliegue.
