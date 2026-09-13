# Hito 5 — Consola y PWA

[English](hito-05-plan-tecnico.md)

Estado: en ejecución desde el 13 de septiembre de 2026.
Prerrequisito: Hito 4 fusionado y aceptado.

## Resultado

Entregar una consola web responsive e instalable que permita completar el caso
principal —iniciar sesión, registrar el secreto de un proveedor, construir y
publicar una ruta y emitir un token de agente— sin editar archivos ni código.
También debe mostrar salud y actividad operativa sin revelar secretos, prompts o
respuestas.

La consola es una SPA de TypeScript, React y Vite. Sus archivos compilados se
incrustan en el ejecutable Go; Node.js no se ejecuta en producción. La API
administrativa sigue siendo la única autoridad y la interfaz no mantiene una
segunda fuente de configuración.

## Límites

- Incluye autenticación, onboarding guiado, CRUD de recursos, metadatos de
  secretos, ciclo de tokens, diagnóstico básico, responsive, PWA y accesibilidad.
- No incluye todavía editor visual de estrategias, relays, backup/restauración,
  instalador systemd ni contenido de prompts/respuestas.
- Los valores secretos se envían únicamente al endpoint de escritura, se limpian
  del estado de formulario después de usarlos y nunca vuelven desde el servidor.
- La sesión usa cookie `HttpOnly`; el token CSRF vive solo en memoria y se recupera
  mediante `/session/me` al recargar.

## Bloques de entrega

1. **Fundación web (implementada):** workspace frontend reproducible, generación de tipos desde
   OpenAPI, shell accesible, activos incrustados, fallback de navegación seguro,
   manifest y service worker mínimo.
2. **Acceso y sesión (implementación inicial):** login, recuperación de sesión, logout, estados de carga y
   errores uniformes; ninguna ruta administrativa visible sin autenticar.
3. **Resumen operativo (implementación inicial):** salud/readiness, conteos de recursos, estado de rutas y
   destinos, solicitudes e intentos recientes mediante DTOs acotados.
4. **Onboarding guiado (implementación inicial):** flujo proveedor → cuenta/conexión → secreto → modelo →
   destino → estrategia → ruta → token, con validación antes de confirmar.
5. **Administración cotidiana (implementación avanzada):** listas y formularios de recursos, actualización
   optimista con ETag, secretos solo reemplazables y token visible una vez.
6. **Diagnóstico e histórico (implementado):** filtros y paginación de solicitudes, intentos y
   errores; latencia, TTFT, tokens y fallback sin contenido.
7. **Cierre:** pruebas de frontend y contrato, accesibilidad automatizada, build
   reproducible, presupuesto de bundle/RAM, prueba móvil/PWA y QA agrupado.

## Decisiones de implementación

- Se usa `fetch` con un cliente tipado generado; no se añade una biblioteca de
  caché o router hasta que una necesidad medida lo justifique.
- El servidor entrega `/`, rutas SPA conocidas y activos con políticas de caché
  distintas. Las rutas `/api/*`, `/v1/*`, `/healthz` y `/readyz` nunca reciben el
  fallback HTML.
- `index.html` no se cachea de forma durable; los activos con hash sí pueden ser
  inmutables. Respuestas administrativas continúan con `no-store`.
- El service worker limita su caché al shell estático. Nunca intercepta ni guarda
  API, health, prompts, respuestas o credenciales.
- La interfaz conserva una atribución discreta y visible a ModelCairn y su
  repositorio, conforme al `NOTICE` y la decisión de licencia.

## Criterios de salida

1. Una instalación configurada puede completar el caso principal desde la web.
2. Recargar, cerrar sesión, expirar sesión y perder conectividad producen estados
   recuperables y comprensibles.
3. Ningún secreto reaparece en DOM, respuestas, logs, almacenamiento web o caché.
4. La PWA es instalable bajo HTTPS o localhost y funciona como shell cuando el
   backend está temporalmente inaccesible, mostrando que está sin conexión.
5. Navegación por teclado, foco, nombres accesibles, contraste y tamaños móviles
   superan la puerta automatizada acordada.
6. El JavaScript inicial comprimido queda bajo el objetivo de 250 KiB o documenta
   una justificación sin superar 400 KiB.
7. Suite Go, frontend, contratos, build incrustado y benchmark representativo
   pasan sin swap sostenido ni hallazgos críticos/altos abiertos.

## Estrategia de verificación

Cada bloque incorpora pruebas focales. Las pruebas end-to-end usan un servidor
real temporal y datos sintéticos; no usan proveedores ni secretos reales. El QA
independiente se agrupa después de los bloques funcionales y antes del benchmark,
evitando revisiones repetitivas por cambios editoriales.
