# Runbook operativo de la Fase 1

[English](phase-1-runbook.md)

Este índice guía una instancia nativa Linux. Los procedimientos enlazados son la
fuente de verdad; no copie comandos desde evidencia histórica sin revisarlos.

## Preparar e instalar

1. Revise [acceso de red](acceso-red-v1.es.md) y elija loopback, TLS directo o
   proxy TLS antes de exponer puertos.
2. Siga [instalación y actualización](instalacion-linux-v1.es.md).
3. Ejecute el bootstrap administrativo con una contraseña nueva y configure la
   primera ruta mediante consola o CLI.
4. Compruebe `/healthz` y `/readyz`; readiness debe nombrar la dependencia degradada.

## Operación normal

- Use la vista Actividad para solicitudes e intentos sin contenido.
- Vigile RSS/swap, espacio del directorio de datos, crecimiento de SQLite, errores
  429/5xx y destinos en cooldown.
- No mueva una credencial de egreso automáticamente. Un cambio deliberado debe ser
  planificado, advertido y auditado.
- Conserve HTTP en loopback. Para Tailscale/u otra red privada, aplique uno de los
  modos HTTPS documentados.

## Diagnóstico rápido

1. **Proceso caído:** consulte `systemctl status modelcairn` y journal; un puerto
   ocupado debe producir fallo, no un falso arranque correcto.
2. **No ready:** consulte `/readyz`; revise almacenamiento, configuración y llavero.
3. **Ruta falla:** confirme AgentToken, alias publicado, elegibilidad, cooldown,
   clasificación y presupuesto. No copie cuerpos o credenciales a un issue.
4. **Disco creciendo:** mida SQLite, backups y journal por separado. La retención
   general sigue diferida; no elimine la base activa manualmente.
5. **Sospecha de filtración:** revoque/rote el secreto afectado, preserve evidencia
   redactada y use el canal privado descrito en `SECURITY.md`.

## Recuperación y mantenimiento

- Siga [backup y recuperación MCB1](backup-recuperacion-v1.es.md). Verifique un
  backup antes de depender de él y pruebe restauración en una instalación temporal.
- Una restauración crea y activa una generación; rollback vuelve al predecesor
  exacto. Las sesiones anteriores se invalidan.
- Para actualizar o volver atrás, use el flujo generacional documentado; no
  reemplace el binario activo a medias.
- La desinstalación predeterminada conserva datos. Elimine datos solo mediante una
  decisión separada y después de identificar la ruta absoluta correcta.

## Antes de pedir ayuda

Registre versión/commit, arquitectura, modo de red, timestamps UTC, estado de las
sondas y pasos mínimos. Redacte API keys, tokens, cookies, contraseñas, prompts,
respuestas, IP privadas, backups y rutas personales.
