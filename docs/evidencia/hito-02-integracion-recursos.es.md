# Hito 2 — Integración y recursos

[English](hito-02-integracion-recursos.md)

Estado: punto 6 y Hito 2 aceptados. Registro: 2026-09-09.

## Cobertura del cierre

La suite automatizada y `scripts/verify-hito2.sh` cubren conjuntamente:

- migración limpia, repetida, interrumpida, con checksum alterado, historial
  discontinuo y esquema futuro o incompatible;
- rollback de recursos y auditoría, conflictos optimistas y consumo atómico de planes;
- propiedad exclusiva entre el servicio y la CLI en ambos órdenes;
- interrupción de rotación en cada límite durable y fallo cerrado ante claves o
  ciphertext ausentes, incorrectos o malformados;
- configuración real de diez recursos, exportación canónica, re-apply noop y
  rechazo de planes obsoletos, modificados o reutilizados;
- canarios secretos en parser, errores, logs, planes, exportaciones, metadatos,
  auditoría, archivos temporales y directorio persistente;
- formato, race detector, cobertura, vet, dependencias, vulnerabilidades
  alcanzables, contratos documentales y compilaciones Linux AMD64/ARM64 en CI.

La compuerta de producto crea una instalación mediante la CLI publicada, no por
fixtures internos. Mide el servidor después de persistir el grafo y el secreto;
también intenta un comando offline mientras el servicio posee la instalación y
comprueba que la configuración no cambió.

## VM representativa de 1 GB

Ejecución de 120 segundos sobre Linux x86_64 con 2 CPU lógicas y 975.064 KiB de
RAM, usando un binario precompilado porque la VM de producción no instala Go:

| Medida | Resultado | Límite exigido |
|---|---:|---:|
| Tiempo hasta `/healthz` | 86 ms | informativo |
| RSS promedio | 11.768 KiB | 131.072 KiB |
| RSS pico | 11.768 KiB | 131.072 KiB |
| Swap del proceso | 0 KiB | 0 KiB esperado |
| Binario reducido | 11.624.608 bytes | informativo |
| Base SQLite configurada | 270.336 bytes | parte del límite de datos |
| Directorio antes/después | 278.560 / 278.560 bytes | 16.777.216 bytes |

Se recolectaron 119 muestras. El round trip, el noop, la propiedad exclusiva y el
escaneo del canario aprobaron. El directorio no creció durante el servicio vacío
configurado. Esta es una medición representativa de arranque y reposo con diez
recursos; no pretende medir routing ni carga concurrente, que pertenecen al Hito 4.

## Límites y reproducibilidad

CI ejecuta la misma compuerta durante cinco segundos para detectar regresiones en
cada cambio. La VM usa 120 segundos. Los límites son parámetros explícitos y la
ejecución falla si los supera. El informe no conserva valores secretos, IP,
hostname, credenciales, rutas personales ni identificadores de la VM.

El binario medido declara el commit público
`db87b3e474b6087b37fe84e89869b8dc2e3f325e` y su SHA-256 es
`3e2eb2746fffbf7f7af765279e6a3cf9547089c451ea9f932308c5071919d41e`.
Estas huellas permiten reproducir y verificar el artefacto; no son credenciales ni
identificadores privados.

El QA agrupado detectó una brecha alta: `stderr` podía escapar del escaneo del
canario. También encontró trazabilidad incompleta del binario, estado documental
antiguo y dos posibles falsos positivos. Las cinco observaciones se corrigieron en
un solo bloque; la revisión focalizada confirmó el cierre sin defectos críticos o
altos. CI volvió a aprobar la suite completa y ambas compilaciones.
