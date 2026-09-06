# Benchmark representativo de recursos del Hito 1

[English](benchmark-hito-1-2026-09-06.md)

- Resultado: **aprobado**
- Fecha: 6 de septiembre de 2026 (UTC)
- Revisión del producto: `6c69032`
- Entorno: VM Linux de Oracle, AMD64, 2 CPU lógicas y 975.064 KiB de RAM
- Kernel: Linux 5.15.0, compilación de Oracle
- Toolchain: Go 1.27.0, Linux AMD64
- Escenario: estado vacío en memoria, sin rutas de proveedor y sonda de salud local
- Duración e intervalo: 900 segundos, muestra cada 5 segundos
- Muestras: 180
- RSS promedio: 6.328 KiB
- RSS pico: 6.328 KiB
- Swap pico del proceso: 0 KiB
- Presupuesto pico exigido: 131.072 KiB
- RSS pico del probe de inicialización SQLite: 10.288 KiB
- Tamaño del binario del servidor vacío: 6.795.527 bytes
- Tamaño del binario de prueba SQLite: 11.168.761 bytes

El proceso utilizó el 4,8 % del presupuesto exigido de 128 MiB para el servidor
vacío y el 0,65 % de la memoria física de la VM. La cifra de SQLite incluye el
harness de pruebas de Go; por eso es una medición conservadora del spike de
integración y no una medición del arranque del producto.

La memoria disponible del sistema cambió de 599.164 KiB a 580.856 KiB durante la
ejecución. El swap libre del sistema cambió de 1.910.128 KiB a 1.910.640 KiB,
mientras el proceso ModelCairn informó cero swap. Esos valores globales incluyen
servicios residentes ajenos y se registran como contexto, no se atribuyen a
ModelCairn.

El informe crudo fue revisado antes de publicarse. Se omiten hostname, dirección
IP, usuario, identificador completo del commit y otros identificadores únicos del
entorno. El script ejecutable y los archivos de módulos fijados conservan el
método reproducible de medición.

## Decisión

La puerta representativa de recursos del servidor vacío del Hito 1 queda
aprobada. Este resultado valida solamente el esqueleto actual; persistencia,
routing, consola, carga sostenida, retención y relays de egreso necesitarán sus
propios presupuestos y puertas posteriores.
