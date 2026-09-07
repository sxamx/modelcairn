# Evidencia del Hito 2 — repositorios de recursos y auditoría

[English](hito-02-item-02-resource-repositories.md)

- Alcance: punto de entrega 2 e issue #9
- Estado: implementación y QA independiente completos; CI sigue como puerta de publicación

## Comportamiento implementado

- Un repositorio es dueño de los envelopes de recursos de configuración, sus
  proyecciones tipadas y la auditoría de mutaciones.
- Las creaciones asignan UUID inmutable y versión 1; las actualizaciones exigen la
  versión actual exacta y la incrementan una vez; lecturas y listados son deterministas.
- Envelope, proyección y auditoría de éxito confirman juntos. Un fallo tipado, de
  referencia, afinidad, unicidad o auditoría revierte toda la mutación.
- Una mutación cuyo rollback es conocido registra auditoría tipada de fallo en otra
  transacción. Los errores de commit se consideran indeterminados y nunca se
  etiquetan erróneamente mediante otra auditoría de fallo.
- Referencias físicas y respaldadas por JSON fuerzan un orden de borrado seguro.
  Borrar Credential deja intacto su Secret.
- Revocar AgentToken es irreversible para una identidad y conserva el instante
  original junto con los metadatos del verificador emitido.

## Evidencia ejecutable

Las pruebas automatizadas hacen round-trip de los diez tipos v1alpha1 e inspeccionan
sus referencias tipadas. Cubren rollback de creación/actualización, versiones
obsoletas y concurrentes, referencias ausentes, destinos entre proveedores, orden
de borrado físico y lógico, autorización explícita de borrado, rollback por fallo
de auditoría de éxito, auditoría separada de fallo, listas permitidas de salida y
base de auditoría no disponible.

El subconjunto sensible a transacciones pasa diez repeticiones. La revocación de
AgentToken y la preservación de su timestamp pasan veinte repeticiones avanzando el
reloj entre escrituras. Suite Go completa, `go vet`, documentación e higiene del
diff pasan localmente.

La QA independiente encontró dos errores bloqueantes de coherencia y una prueba de
regresión débil: acción de auditoría fuera del contrato, reactivación contradictoria
del token y reloj fijo que podía ocultar el reemplazo del timestamp. Los tres se
corrigieron. La revisión final no informó bloqueantes funcionales pendientes.
