# QA agrupado del Hito 7

[English](qa-agrupado-hito-07.md)

- Fecha: 19 de septiembre de 2026
- Alcance final: revisión independiente del cierre de Fase 1 en el PR #26
- Revisión aprobada: `bbdb472`
- Veredicto: **aprobado, sin bloqueos abiertos**

La primera revisión bloqueó el cierre. Detectó capacidades demasiado permisivas
en onboarding, ausencia de presupuestos cuantitativos, trazabilidad insuficiente
de egreso/redirect, cobertura canario incompleta y recuperación parcial ante
respuestas perdidas. Esos hallazgos se corrigieron y se volvieron a revisar.

La segunda revisión encontró un bloqueo alto adicional: la interfaz proponía
revocar y reemitir el mismo `AgentToken`, operación que el backend prohíbe. Se
incorporó rotación atómica y repetible. Las pruebas demuestran que el bearer
anterior queda inválido, solo el más reciente funciona, una segunda rotación
recupera otra respuesta perdida y una identidad sin emitir, vencida o revocada no
puede rotarse. La compuerta Linux ejercitó el endpoint real, exigió 401 al token
anterior y usó el nuevo en rutas normal y SSE.

La reevaluación final confirmó:

- CI verde en tests y builds Linux AMD64/ARM64;
- 17 pruebas web y suites Go completas aprobadas;
- benchmark de 600 segundos dentro de todos los presupuestos;
- inventario ejecutable de egreso y ausencia de telemetría externa;
- capacidades `text` conservadoras y opt-in para streaming/tools;
- recuperación de apply y entrega perdida de token coherentes con el backend;
- canarios ausentes de persistencia, logs y artefactos sensibles.

La única observación baja fue un texto antiguo de la interfaz que aún decía
“revocar”; se corrigió a “rotar”. No quedan hallazgos críticos, altos ni medios que
invaliden la evidencia o la aceptación de Fase 1.
