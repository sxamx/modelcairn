# Benchmark representativo de la consola del Hito 5

[English](benchmark-hito-05-2026-09-13.md)

- Estado: aprobado en VM objetivo.
- Fecha: 13 de septiembre de 2026.
- Entorno: Linux x86_64, 2 CPU lógicas y aproximadamente 1 GB de RAM.
- Duración: 180 segundos; 90 muestras cada 2 segundos.
- Binario reducido con consola incrustada: 13.725.856 bytes.
- RSS promedio: 14.640 KiB.
- RSS máximo: 14.640 KiB.
- Swap máximo del proceso: 0 KiB.
- Presupuesto máximo exigido: 131.072 KiB.

## Alcance

`scripts/benchmark-console.sh` ejecutó el binario real del Hito 5 desde un
directorio temporal. Antes de medir comprobó `/healthz`, el documento de la SPA,
el manifiesto PWA y la política no-cache del documento principal. El proceso se
cerró al finalizar y no modificó la instalación permanente.

El benchmark mide el coste en reposo del servidor con la consola incrustada; no
repite la carga concurrente del motor, ya cubierta por el Hito 4. La evidencia
pública omite hostname, IP, usuario, rutas temporales e identificadores de
revisión.
