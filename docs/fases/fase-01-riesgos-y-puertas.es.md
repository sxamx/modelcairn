# Fase 1 — Riesgos y puertas de calidad

[English](fase-01-riesgos-y-puertas.md)

- Estado de esta línea base: aceptada como conjunto de puertas del Hito 1. Para
  el avance vigente, véase
  [Estado y ciclo](../estado-y-ciclo-del-proyecto.es.md).

## Riesgos activos

| Riesgo | Señal | Mitigación | Decisión si ocurre |
|---|---|---|---|
| Go/stack supera memoria | RSS estable o pico fuera del presupuesto | benchmark desde Hito 1, perfiles y límites | optimizar o superseder ADR antes de continuar |
| SQLite acumula contención | colas de escritura/latencia bajo eventos | transacciones cortas, WAL y benchmark | separar escrituras o revisar persistencia |
| Compatibilidad “OpenAI” pierde campos | pruebas de contrato o cliente real falla | matriz explícita y rechazo de capacidades | ampliar adaptador sin passthrough silencioso |
| Fallback duplica trabajo | timeout tras enviar request | resultado indeterminado, sin retry por defecto | habilitar solo con idempotencia/política |
| Filtración de API key | secreto aparece en salida o endpoint equivocado | redactor común, afinidad y pruebas 6/6 | bloqueo inmediato del hito |
| Config web/YAML diverge | conflicto de resourceVersion | fuente única, plan/apply y ETag | rechazar y exigir nuevo plan |
| Backup no recupera | restore o prueba de ruta falla | MCB1/age y generaciones atómicas | no cerrar Hito 6 |
| UI aumenta dependencias | bundle o auditoría excede presupuesto | límite, carga diferida y revisión | retirar/sustituir dependencia |

## Puerta de entrada de un hito

- Hito anterior aceptado y sin defectos críticos/altos.
- Contratos y ADR necesarios disponibles.
- Criterios de aceptación y pruebas identificados.
- No se incorporan secretos, datos privados ni historial descartado.

## Definición de terminado

Un hito está terminado únicamente cuando:

1. el código incluido cumple su contrato y tiene pruebas proporcionales;
2. linters, build y pruebas pasan de forma reproducible;
3. documentación, migraciones y ejemplos coinciden con el comportamiento;
4. los presupuestos aplicables se miden, no se suponen;
5. errores y secretos se revisan en logs/exportaciones;
6. una revisión independiente no mantiene hallazgos críticos o altos;
7. el estado del proyecto enlaza evidencia y limitaciones conocidas.

## Política de cambios

Una implementación puede descubrir que un contrato no es viable. En ese caso se
detiene únicamente la parte afectada, se registra evidencia, se modifica el
documento o ADR y se actualizan pruebas antes de continuar. El código no se usa
para cambiar silenciosamente una decisión aprobada.
