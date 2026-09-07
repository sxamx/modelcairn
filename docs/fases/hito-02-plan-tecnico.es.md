# Hito 2 — Plan técnico de entrega

[English](hito-02-plan-tecnico.md)

- Estado: preparado para implementación
- Salida: configuración persistida completa sin ejecución de routing ni interfaz
  web, operable mediante CLI y pruebas
- Prerrequisito: Hito 1 aceptado

## Progreso

- Punto de entrega 1 — ciclo de base de datos y migraciones: aceptado. La evidencia
  está en [Evidencia del ciclo de base de datos del Hito 2](../evidencia/hito-02-item-01-storage-lifecycle.es.md).
- Punto de entrega 2 — repositorios de recursos y auditoría: aceptado localmente
  bajo el [contrato de repositorios](../contratos/storage/resource-repositories-v1.es.md),
  con [evidencia ejecutable](../evidencia/hito-02-item-02-resource-repositories.es.md).
- Punto de entrega 3 — clave maestra y secretos: en curso. El núcleo de cifrado
  tiene pruebas; están implementados llavero, verificación de arranque y rotación.
  Véase [evidencia de rotación y validaciones pendientes](../evidencia/hito-02-rotation.es.md).
  Este punto todavía no está aceptado.

## Orden de entrega

1. **Ciclo de base de datos y migraciones.** Sustituir el cargador del spike por
   migraciones incrustadas y monotónicas; verificar checksums, WAL, integridad,
   propiedad exclusiva, arranque interrumpido e inicialización limpia.
2. **Repositorios de recursos y auditoría.** Implementar recursos y registros
   tipados en transacciones, versiones optimistas, referencias, invariantes de
   afinidad de proveedor, borrado ordenado y eventos de auditoría sin secretos.
3. **Clave maestra y almacén de secretos.** Crear el llavero privado versionado,
   cifrar y rotar secretos con XChaCha20-Poly1305 y datos asociados vinculados,
   exponer solo metadatos y fallar cerrado ante material ausente o inválido.
4. **Codec y validación de configuración.** Leer YAML acotado, aplicar defaults,
   canonizar documentos, validar estructura y referencias cruzadas, y producir
   exportaciones deterministas redactadas.
5. **CLI de plan y apply atómico.** Producir planes deterministas de
   create/update/noop/delete, archivos de plan autenticados con vencimiento,
   confirmación interactiva, detección de conflictos, apply todo-o-nada y auditoría
   bajo el bloqueo de instalación.
6. **Integración y puerta de recursos.** Probar round trips, rollback, corrupción,
   interrupción de rotación, conflictos concurrentes, redacción y coste real de
   SQLite en memoria/disco sobre la VM representativa.

Cada punto puede revisarse por separado, pero se acepta solo después de completar
los puntos anteriores de los que depende. Crear y restaurar el contenedor de backup
pertenece al Hito 6; el Hito 2 debe conservar los contratos que este consumirá.

## Aceptación del hito

- Una instalación limpia migra al schema actual y los arranques repetidos son noop;
  checksums cambiados y schemas incompatibles fallan de forma cerrada.
- La configuración sobrevive export y re-apply sin deriva semántica ni revelar secretos.
- Referencias inválidas, cruce de proveedor, planes obsoletos y operaciones
  parciales se rechazan con el resultado documentado y sin subconjuntos aplicados.
- Los secretos nunca aparecen en YAML, planes, exports, errores, logs, detalles de
  auditoría ni snapshots de pruebas.
- Interrumpir la rotación en cada límite durable deja un estado que arranca con el
  material anterior o nuevo y nunca pierde capacidad de descifrado.
- Las operaciones CLI con estado no pueden competir con el servicio por SQLite.
- Una mutación exitosa y su auditoría confirman juntas; una mutación revertida
  registra el fallo por separado sin afirmar que fue aplicada.
- Los límites de bytes, profundidad, documentos, aliases y schema de YAML/JSON se
  prueban en sus bordes. Secretos canario no aparecen en ninguna salida y la
  auditoría tipada rechaza campos no aprobados.
- Pasan las pruebas relevantes, validación documental, compilaciones cruzadas,
  revisión de dependencias y medición representativa de recursos.
- La revisión de dependencias exige `go mod tidy` limpio, `govulncheck ./...` sin
  vulnerabilidades alcanzables conocidas y licencia registrada para cada módulo
  directo.
- QA independiente no mantiene hallazgos críticos o altos.

## Postergaciones explícitas

Las sesiones administrativas y mutaciones HTTP pertenecen al Hito 3. Las llamadas
a proveedores, publicación de estrategias y fallback pertenecen al Hito 4. La
consola es del Hito 5 y el backup/restauración cifrados completos son del Hito 6.
