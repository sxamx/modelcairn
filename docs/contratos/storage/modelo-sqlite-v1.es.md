# Modelo SQLite v1

[English](modelo-sqlite-v1.md)

El archivo [schema-v1.sql](schema-v1.sql) sigue siendo el contrato físico inicial
legible. La migración `0001_initial.sql` del Hito 2 implementa ese contrato bajo un
checksum SHA-256 inmutable y pruebas ejecutables del ciclo de vida.

## Reglas transaccionales

- Crear o actualizar un recurso y su tabla tipada ocurre en una transacción.
- Publicar una estrategia inserta una versión inmutable y cambia la ruta en una
  sola transacción.
- Registrar una solicitud y sus intentos puede usar transacciones breves separadas;
  ninguna transacción permanece abierta durante una llamada de red o stream.
- La afinidad vive en `credentials.egress_id`; el router nunca la modifica.
- El writer SQLite será coordinado dentro del proceso y se probará WAL. No habrá
  acceso directo desde otros procesos ni filesystem de red.
- Las retenciones eliminan por lotes pequeños y nunca desactivan límites de disco.
- Los cambios de configuración y su evento de auditoría exitoso confirman juntos.
  Después de revertir una mutación, su evento de fallo se intenta en otra
  transacción breve y no puede afirmar que el estado fue aplicado. Si SQLite no
  puede registrar ese evento, se emite un error operativo estructurado sin secretos.
- El nonce de un token de plan se consume en la misma transacción que apply. Su
  reutilización devuelve `plan_already_used`; las filas vencidas son datos de
  mantenimiento y pueden depurarse.

## Contenido deliberadamente ausente

No existen columnas para prompt, mensajes, tool outputs o respuesta. El contrato
`content_stored = 0` hace visible la política de Fase 1. Los errores persistidos
son clases normalizadas; cuerpos upstream y cabeceras sensibles no se almacenan.

## Evolución

Los recursos conservan `spec_json` para round-trip y tablas tipadas para invariantes
y consultas críticas. Una migración debe mantener ambas representaciones
consistentes. Si esta duplicación no supera las pruebas, se simplificará mediante
un ADR antes de publicar una versión estable.
