# Contrato de repositorios de recursos v1

[English](resource-repositories-v1.md)

## Límite

El repositorio es el único código del Hito 2 autorizado para mutar envelopes de
recursos, sus proyecciones tipadas y la auditoría de mutaciones. Recibe objetos JSON
canónicos desde la capa de validación de configuración. Aun así fuerza identidad,
versiones optimistas, referencias de base de datos, afinidad de proveedor y borrado
seguro; la validación JSON Schema completa pertenece al codec de configuración.

La identidad declarativa es `(kind, name)`. Una creación usa versión esperada cero
y recibe un UUID aleatorio inmutable y versión de recurso 1. Una actualización debe
entregar la versión positiva actual y la incrementa exactamente una vez. Las
lecturas devuelven el spec canónico almacenado y los listados se ordenan por kind y
name.

## Transacciones

- El envelope y su proyección tipada se crean o actualizan en una transacción.
- Una versión obsoleta, referencia ausente, cruce de proveedor, colisión única o
  trigger tipado revierte toda la mutación.
- La auditoría de éxito se inserta en esa misma transacción. Si no se puede
  insertar, el cambio de estado no se confirma.
- Tras el rollback se intenta una auditoría de fallo en otra transacción breve. Si
  SQLite tampoco puede escribirla, el llamador recibe el error original y un error
  estructurado de auditoría no disponible.

`Provider` y el borrador de `Strategy` no requieren hoy una proyección física
separada; su `spec_json` canónico es autoritativo. Los demás recursos de
configuración usan su tabla tipada de `schema-v1.sql`. Las referencias a destinos
de Strategy y a rutas de AgentToken se resuelven y protegen explícitamente porque
el schema físico almacena esos arreglos como JSON.

La revocación de un AgentToken es irreversible para una identidad de recurso.
Cambiar `enabled` de false nuevamente a true se rechaza como `invalid_resource`;
emitir otro token utilizable exige una identidad nueva mediante la operación
dedicada. Así, el envelope nunca puede afirmar que está habilitado mientras su fila
tipada continúa revocada.
Las actualizaciones deshabilitadas posteriores conservan el instante original de
revocación y los metadatos de la identidad emitida.

## Códigos de resultado estables

- `already_exists`
- `invalid_actor`
- `invalid_resource`
- `not_found`
- `reference_not_found`
- `resource_in_use`
- `version_conflict`
- `delete_not_allowed`
- `provider_mismatch`

Ningún código incluye el spec ni el valor escalar rechazado.

## Borrado y secretos

El borrado requiere autorización explícita y la versión exacta. Las claves foráneas
y comprobaciones de arreglos lógicos rechazan un orden inseguro con
`resource_in_use`. Borrar una Credential nunca borra su Secret; el borrado del
secreto pertenece a una operación separada del almacén de secretos.

## Detalles tipados de auditoría

Los llamadores no pueden entregar mapas arbitrarios de auditoría. El repositorio
construye internamente los detalles desde esta lista fija:

| Resultado | Campos de detalle permitidos |
|---|---|
| success | `version` |
| failure | `code` |

Las acciones se restringen a `resource.create`, `resource.update` y
`resource.delete`; el actor es `admin`, `cli` o `system`.
