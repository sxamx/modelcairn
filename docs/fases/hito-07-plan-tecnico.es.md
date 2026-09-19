# Hito 7 — Plan técnico de validación y cierre

[English](hito-07-plan-tecnico.md)

- Estado: **en ejecución**
- Etapa: F — validación de sistema y seguridad
- Objetivo: decidir con evidencia reproducible si la Fase 1 puede aceptarse; si
  no puede, registrar bloqueos concretos sin declarar un cierre parcial.

Este hito valida el producto ya construido. No incorpora el estimador adaptativo,
relays de salida, nuevos protocolos compatibles ni el editor visual: son funciones
posteriores y no son criterios de cierre de la Fase 1.

## Principios

1. La matriz RF/RNF separará la Fase 1 del trabajo diferido y enlazará cada
   afirmación con una prueba, contrato o evidencia concreta.
2. Las pruebas destructivas usarán una instalación y credenciales temporales.
3. Prompts y respuestas seguirán sin persistirse por defecto. La revisión usará
   secretos canario, nunca credenciales reales del operador.
4. La validación representativa se agrupará en una ejecución sobre la VM de 1 GB;
   no se repetirá un benchmark completo por cada cambio pequeño.
5. Habrá una sola revisión QA independiente agrupada al completar los bloques.

## 1. Inventario y trazabilidad ejecutable

- Revisar RF-001–RF-013 y RNF-001–RNF-008 contra el estado real.
- Añadir enlaces directos a pruebas, contratos y evidencia representativa.
- Marcar RF-101 en adelante y RF-201 en adelante como diferidos, no faltantes.
- Registrar cada desviación como corregida, aceptada explícitamente o bloqueante.

Salida: matriz completa y manifiesto de aceptación verificable por CI.

## 2. Sistema y fallos inducidos

- Cubrir 429, 5xx, timeout, DNS/upstream inaccesible y cancelación del cliente.
- Validar streaming antes y después de comprometer encabezados, fallback acotado y
  ausencia de reintentos inseguros.
- Probar reinicio, configuración concurrente, conflictos de versión, interrupción
  de publicación/migración, capacidad insuficiente y última generación válida.
- Revalidar backup, verificación, restore, rollback y revocación de sesiones.

“Multinodo” se limita aquí a los límites arquitectónicos y a comprobar que el nodo
principal no presupone relays. El protocolo y tráfico de relays (RF-101–RF-108)
pertenece a una fase posterior.

## 3. Carga, recursos y retención

- Ejecutar al menos 10 minutos de carga mixta en la VM de 1 GB: solicitudes
  normales, SSE y panel.
- Medir RSS medio/pico, swap, CPU, binario, crecimiento SQLite y latencias contra
  presupuestos RNF ya definidos.
- Sembrar eventos de edades sintéticas para probar retención arbitraria —horas,
  meses, años e ilimitada— sin esperar tiempo real.
- Comprobar poda por lotes y consultas históricas acotadas.

Salida: informe bilingüe reproducible con entorno, comandos y resultados.

## 4. Seguridad y privacidad proporcionales

- Actualizar modelo de amenazas y límites de confianza.
- Probar autenticación, logout/revocación, CSRF, autorización y límites de intentos.
- Buscar secretos canario en logs, auditoría, planes, exportaciones, errores, API,
  interfaz y backups. Un secreto en texto claro es bloqueante.
- Verificar permisos, cifrado en reposo, transporte por modo de red, ausencia de
  telemetría externa y ausencia de prompts/respuestas persistidos por defecto.

El objetivo es seguridad práctica para un servicio autohospedado, no controles
ajenos a su modelo de amenaza.

## 5. Operación y contribución

- Consolidar runbook de instalación, diagnóstico, actualización, backup, restore,
  rollback y desinstalación.
- Añadir guía de contribución y política de reporte privado de vulnerabilidades.
- Corregir estados obsoletos y mantener español e inglés equivalentes.
- Reservar la release pública y sus notas para la Etapa G.

## 6. Aceptación

1. Ejecutar CI y la validación representativa completa.
2. Realizar una revisión QA independiente agrupada con criterio propio.
3. Corregir hallazgos críticos/altos y medios que invaliden evidencia.
4. Publicar el informe final y actualizar trazabilidad, ciclo y tablero.

La Fase 1 solo se acepta si cada RF/RNF en alcance tiene evidencia válida, CI está
verde, la VM cumple sus presupuestos y no quedan bloqueos de seguridad, privacidad,
recuperación u operación. En caso contrario se documentan bloqueos verificables;
nunca se declara “casi completada”.
