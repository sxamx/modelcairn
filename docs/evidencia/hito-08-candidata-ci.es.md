# Hito 8 — Evidencia de candidata CI verificada

[English](hito-08-ci-candidate.md)

- Fecha: 20 de septiembre de 2026
- Revisión de `main`: `d6565bb`
- Identificador técnico de versión: `0.0.0-ci`
- Resultado: workflow y consumo del paquete AMD64 aprobados

`0.0.0-ci` es deliberadamente una versión técnica, no una propuesta de versión
pública. No se creó tag ni GitHub Release.

## Cadena ejecutada

El workflow manual comprobó que la revisión pertenece a `main`, reconstruyó y
probó frontend/backend, validó documentación y egreso, generó dos tarballs Linux,
los verificó y reconstruyó, produjo SBOM SPDX 2.3, emitió atestación de procedencia
y subió un artefacto privado con retención de 14 días. Sus permisos no incluían
`contents: write`.

| Artefacto | Tamaño |
|---|---:|
| Linux AMD64 | 5.871.620 bytes |
| Linux ARM64 | 5.410.941 bytes |
| SBOM SPDX JSON | 191.697 bytes |
| `SHA256SUMS` | 306 bytes |

La SBOM quedó incluida en `SHA256SUMS` y recibió su propia atestación. Ambos
tarballs aprobaron checksum,
allowlist, ausencia de traversal, modos `0755`/`0644`, manifiesto, arquitectura y
metadata Go contra el commit exacto.

## Consumo independiente del artefacto

Se descargó el zip de Actions y se inspeccionó fuera del workflow que lo produjo.
La búsqueda no encontró IP, usuario remoto, ruta personal, prefijo de token ni los
canarios de credencial/proveedor usados por las pruebas. Después se copió solo el
tarball AMD64 a un directorio temporal de la VM representativa.

En Linux se volvió a verificar SHA-256, se extrajo el paquete, se exigió modo 0755
al binario y se ejecutó la compuerta integrada con servidor real, bootstrap,
configuración, rotación de AgentToken, rechazo del bearer anterior, llamada normal
y SSE. Terminó con código cero, 56.604 KiB de RSS pico y sin tocar la instalación o
los datos persistentes. El entorno temporal se eliminó al terminar.

También se ejecutó una compuerta destructiva acotada sobre el host representativo.
Antes de actuar rechazó cualquier binario, unidad o servicio activo, tomó una
instantánea local del estado inactivo y configuró restauración obligatoria mediante
un trap. Instaló el paquete anterior, creó y verificó un respaldo MCB1 cifrado con
modo 0600, actualizó al paquete nuevo y confirmó disponibilidad e identidad
administrativa. Luego volvió al paquete anterior, repitió esas comprobaciones,
desinstaló la copia temporal y restauró la instantánea. El estado preservado
recuperó propietarios y modos, sin binario, unidad ni servicio activo.

La revisión posterior añadió restauración real del MCB1 hacia una generación
aislada, comprobación del autoarranque durante instalación/actualización/rollback
y casos negativos para checksum incorrecto, archivo truncado y arquitectura
equivocada. Todos aprobaron. El QA independiente no dejó bloqueos técnicos.

## Hallazgos corregidos

1. El primer tarball local creado desde Windows no conservó el modo ejecutable. El
   empaquetador ahora fija modos dentro del archivo y el verificador los exige.
2. Windows y Linux emitían variantes estándar distintas de `SHA256SUMS`. El
   generador ahora fuerza formato binario y el verificador acepta ambas variantes.
3. `upload-artifact` v4 y v5 produjeron una advertencia por Node 20 forzado a
   Node 24. Se fijó el SHA oficial de v7.0.1, cuya acción declara `node24`.

## Límites pendientes

- ARM64 fue construido e inspeccionado, pero no ejecutado en hardware ARM64.
- Falta la decisión del mantenedor sobre versión, canal y datos públicos.
- La actualización validada no incluyó una migración de esquema irreversible; esa
  política se definirá antes de una versión que la necesite.
- Esta evidencia no autoriza publicación.
