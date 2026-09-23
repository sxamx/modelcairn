# Rediseño de la interfaz web: estructura aprobada

Estado: arquitectura de información aprobada por el mantenedor el 22 de septiembre de 2026; wireframes y estilo visual pendientes de validación. Este documento no modifica el contrato del backend ni sustituye los ADR existentes.

La propuesta visual 01 recibió aprobación provisional de dirección el 22 de septiembre, con una petición expresa de mejorar la experiencia móvil y preservar la PWA. No equivale todavía a aprobación de todas las pantallas ni a autorización para sustituir la web real.

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
- Toda cifra se identifica como observada, estimada o no disponible. Los wireframes pueden usar ejemplos, pero la web real no mezclará ejemplos con datos del usuario.

## Alcance del wireframe

El wireframe independiente muestra navegación, jerarquía, acciones y estados, no la identidad visual final ni respuestas de backend. Los controles no implementados se marcarán como propuesta. Primero se valida este mapa y el recorrido; después se define el sistema visual (temas, tipografía, color, iconos, componentes y movimiento) y por último se implementa por bloques con pruebas de accesibilidad y rendimiento en la VM de 1 GB.

## Pendientes que deben diseñarse antes de implementar

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
