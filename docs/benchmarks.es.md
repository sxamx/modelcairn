# Procedimiento de benchmark de recursos

[English](benchmarks.md)

El Hito 1 usa `scripts/benchmark-idle.sh` para medir el servidor Linux vacío. El
script construye un binario reducido, espera `/healthz`, muestrea `VmRSS`, escribe
un informe Markdown como evidencia y falla si se supera el presupuesto configurado.

CI ejecuta una medición de humo de diez segundos con un límite de 128 MiB. Este
límite usa deliberadamente la mitad del presupuesto provisional de 256 MiB en
estado estable y reserva espacio para módulos posteriores. Detecta regresiones
grandes del esqueleto, pero **no** constituye evidencia
representativa para la VM objetivo de 1 GB. El Hito 1 se cierra únicamente después
de que la ejecución predeterminada de 15 minutos pase en la VM de referencia
documentada:

```sh
MAX_RSS_KIB=131072 bash scripts/benchmark-idle.sh
```

El directorio generado `benchmark-results/` se ignora deliberadamente. Después de
su aprobación se copiará a la documentación un informe representativo revisado y
redactado. El informe público nunca debe contener hostnames, direcciones IP,
usuarios ni credenciales de proveedores.

La ejecución representativa del Hito 1 fue aprobada el 6 de septiembre de 2026.
Véase la [evidencia revisada](evidencia/benchmark-hito-1-2026-09-06.es.md).

El muestreo lee RSS y swap del proceso desde `/proc`; las mediciones tienen la
granularidad del intervalo configurado y no incluyen la page cache del kernel ni
procesos hijos. El script también registra memoria y swap antes/después del
muestreo, los procesos residentes más grandes antes del arranque, la versión exacta
del binario y la configuración del servidor vacío. También compila un artefacto de
prueba enlazado con SQLite y registra su tamaño y RSS pico durante la inicialización
del esquema. Esa cifra es conservadora porque incluye el harness de pruebas de Go;
el coste enlazado al producto se medirá otra vez cuando la persistencia forme parte
del arranque.

## Compuerta del producto del Hito 2

`scripts/verify-hito2.sh` configura la instalación real de ejemplo mediante la CLI,
comprueba el round trip canónico, un apply sin cambios, exclusión entre servicio y
CLI y ausencia de un secreto canario. Después mide tiempo hasta `/healthz`, RSS,
swap, tamaño del binario, SQLite y el directorio de datos. CI ejecuta una prueba de
humo corta; el cierre del hito conserva además un informe de la VM representativa.
Si la VM no tiene Go, `MODELCAIRN_BINARY` permite medir un binario Linux precompilado;
`MODELCAIRN_EXPECTED_COMMIT` es obligatorio y se contrasta con la versión incrustada.

```sh
DURATION_SECONDS=120 OUTPUT_FILE=benchmark-results/hito-02.md \
  bash scripts/verify-hito2.sh
```

## Compuerta administrativa del Hito 3

`scripts/verify-hito3.sh` acepta el mismo patrón de binario precompilado y muestrea
RSS/swap durante el recorrido administrativo completo. `MEASURE_SECONDS` añade un
período estable y `OUTPUT_FILE` produce el informe redactado. La ejecución
representativa está registrada en la
[evidencia del Hito 3](evidencia/benchmark-hito-03-2026-09-12.es.md).

## Microbenchmark de streaming del Hito 4

`BenchmarkStreamingRouterConcurrency` ejercita el adaptador directo, selección,
validación y relay SSE con lotes exactos de 1, 2, 5, 10 y 20 streams concurrentes.
Sirve como señal temprana de regresiones y CI ejecuta una iteración de humo:

```sh
go test -run '^$' -bench '^BenchmarkStreamingRouterConcurrency$' -benchtime=1x ./internal/router
```

Este microbenchmark usa un upstream local en memoria y no mide autenticación HTTP,
SQLite, red real, RSS ni swap. Sus cifras de throughput no son capacidad anunciable.
`scripts/verify-hito4.sh` es la compuerta integral: usa el binario real, bootstrap,
sesión administrativa, configuración publicada, AgentToken, adaptador directo,
persistencia y un upstream HTTP local determinista. Mide RSS, swap y latencia en
los cinco niveles, y rechaza contenido persistido o secretos en logs. CI la usa
como humo; el cierre aún exige su informe redactado en la VM representativa.
