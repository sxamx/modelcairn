# Fase 1 — Plan de implementación

[English](fase-01-plan-de-implementacion.md)

- Estado: aceptado; Hitos 0, 1 y 2 completados, Hito 3 es el siguiente
- Regla de entrega: cada hito incluye código, pruebas y documentación; no se marca
  completo por una demostración visual

## Hito 0 — Contratos ejecutables, sin producto

Estado: **completado y aprobado por QA independiente el 5 de septiembre de 2026**.

- JSON Schema del YAML con campos, tipos, referencias, ausencias y validaciones.
- OpenAPI administrativa con errores, concurrencia y ciclo de cada recurso.
- Modelo SQLite inicial con claves, constraints, índices y transacciones.
- Matriz exacta de Chat Completions de la Fase 1.
- Contrato de sesión, cookies, CSRF, revocación y reset.
- ADR criptográfico y formato/atomicidad de backup y restauración.
- Selección del stack de consola y presupuesto de artefacto/recursos.

Salida: contratos revisados que pueden generar fixtures y pruebas. Los spikes se
desechan; no se escribe todavía código persistente del producto.

## Hito 1 — Esqueleto y decisiones verificadas

Estado: **completado el 6 de septiembre de 2026**. La ejecución representativa en
la VM de 1 GB aprobó su puerta de recursos de 15 minutos; véase la
[evidencia revisada del benchmark](../evidencia/benchmark-hito-1-2026-09-06.es.md).

- Estructura modular Go, CLI y servidor HTTP mínimo.
- Build reproducible para Linux AMD64/ARM64.
- Benchmark vacío y presupuestos automatizados.
- Spikes de integración de SQLite y streaming contra los contratos aprobados.
- Registro y comprobación de dependencias.

Salida: decisiones técnicas demostradas en la VM o corregidas antes de acumular
código encima.

Prerrequisito: Hito 0 aceptado. Cada hito posterior requiere que el anterior haya
cumplido su salida y no mantenga bloqueos críticos o altos.

Las reglas comunes se definen en [Riesgos y puertas de calidad](fase-01-riesgos-y-puertas.es.md).

## Hito 2 — Persistencia, configuración y secretos

Estado: **completado y aceptado el 9 de septiembre de 2026**. Véase el
[plan técnico de entrega](hito-02-plan-tecnico.es.md).

- Migraciones y repositorios SQLite.
- Esquema YAML, validate/plan/apply/export.
- Clave maestra, credenciales cifradas y redacción común.
- Auditoría y tests de round-trip/fallo seguro.

Salida: configuración persistida completa sin ejecución de routing ni interfaz,
operable por CLI y pruebas.

## Hito 3 — Identidad y plano administrativo

- Bootstrap, contraseña administrativa y reset local.
- Sesiones, CSRF cuando aplique, expiración y rate limit de login.
- Tokens de agente revocables y separación de permisos.
- API administrativa versionada.

Salida: plano de control seguro cubierto por integración HTTP.

## Hito 4 — Router vertical

- Adaptador OpenAI-compatible y validación de conexiones.
- Modelo, destino, alias, ruta y estrategia secuencial.
- Chat Completions normal, tool calls y streaming.
- Clasificación de errores, cooldown, presupuestos y fallback.
- Eventos operativos sin contenido.

Salida: primera llamada end-to-end y suite de contrato determinista.

## Hito 5 — Consola y PWA

- Onboarding desde proveedor hasta ruta publicada.
- Gestión de recursos y secretos sin posibilidad de revelarlos.
- Diagnóstico de salud, solicitudes e intentos.
- Diseño responsive, manifest, instalación PWA y accesibilidad básica.

Salida: el caso principal se completa desde web sin editar código.

## Hito 6 — Instalación y recuperación

- Instalador systemd con elección de inicio automático.
- Modos localhost, red privada y guía Tailscale Serve.
- Exportación, backup cifrado y restauración funcional.
- Health/readiness, logs y diagnóstico.

Salida: instalación vacía y recuperación verificadas en una VM limpia.

## Hito 7 — Validación y cierre

- Matriz completa de aceptación RF/RNF/pruebas.
- Benchmark con carga y retención.
- Fallos inducidos, migraciones interrumpidas y revisión de secretos.
- Documentación del operador y contribuidor.
- QA independiente y corrección de hallazgos.

Salida: Fase 1 aceptada o lista explícita de bloqueos; nunca cierre parcial.

## Política de commits

Se harán commits cuando una unidad sea coherente y verificable, no para aumentar el
contador. Cada commit tendrá propósito único, pruebas asociadas y ningún secreto.
Los hitos podrán contener varios commits pequeños; el cierre del hito se marcará en
la documentación de estado.
