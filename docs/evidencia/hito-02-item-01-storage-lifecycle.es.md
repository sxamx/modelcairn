# Evidencia del Hito 2 — ciclo de base de datos

[English](hito-02-item-01-storage-lifecycle.md)

- Alcance: punto de entrega 1 e issue #8
- Estado: implementación y QA independiente completos; CI y la ejecución Linux
  representativa son las puertas de publicación

## Comportamiento implementado

- El servicio adquiere el bloqueo no bloqueante del sistema operativo antes de
  abrir SQLite y lo conserva hasta cerrar SQLite.
- El archivo de bloqueo se abre sin seguir enlaces simbólicos en Unix ni puntos
  de reanálisis en Windows; se fuerzan permisos privados del directorio y archivo.
- Las migraciones monotónicas incrustadas usan un ledger SHA-256 inmutable y una
  transacción por migración.
- El arranque rechaza versiones desconocidas, checksums cambiados, historiales con
  huecos, corrupción física y deriva lógica del schema.
- La compatibilidad lógica se comprueba antes de migraciones pendientes y otra vez
  al terminar, contra un schema transitorio construido desde las migraciones.

## Evidencia ejecutable

La suite automatizada cubre arranque limpio y repetido, WAL, paridad con el schema
documentado, checksums cambiados, versiones futuras, huecos del ledger, rollback
tras bootstrap interrumpido, eliminación de objetos, contención servicio/offline
en ambos órdenes, contención entre procesos y recuperación después de matar al
propietario. El subconjunto sensible a concurrencia también pasa repetidamente.

Las puertas locales superadas son: suite Go completa, `go vet`, validación de
documentación, contrato del schema, higiene del diff y compilación cruzada del
binario de pruebas para Linux AMD64, Linux ARM64 y Windows AMD64. La ejecución con
race detector queda como puerta de CI Linux porque el host Windows de desarrollo
tiene CGO desactivado.

La primera QA independiente encontró un hallazgo alto y tres medios. Los cuatro
fueron corregidos; la segunda revisión no encontró hallazgos críticos o altos. Una
protección sugerida para futuras versiones también se implementó antes de publicar.
