# Benchmark representativo del Hito 4

[English](benchmark-hito-04-2026-09-13.md)

- Estado: aprobado en VM objetivo.
- Fecha: 13 de septiembre de 2026.
- Entorno: Linux x86_64, 2 CPU lógicas, 975.064 KiB de RAM total.
- Binario reducido: 13.316.256 bytes.
- Carga: 20 lotes en cada nivel de concurrencia; 760 streams exitosos.
- Muestras: 211, aproximadamente cada 50 ms.
- RSS promedio del servicio: 41.544 KiB.
- RSS máximo del servicio: 56.584 KiB.
- Presupuesto máximo exigido: 131.072 KiB.
- Swap máximo del servicio: 0 KiB.

| Streams concurrentes | Streams exitosos | Latencia promedio | Latencia máxima |
|---:|---:|---:|---:|
| 1 | 20 | 0,057 s | 0,068 s |
| 2 | 40 | 0,067 s | 0,090 s |
| 5 | 100 | 0,106 s | 0,303 s |
| 10 | 200 | 0,128 s | 0,236 s |
| 20 | 400 | 0,191 s | 0,395 s |

## Alcance

`scripts/verify-hito4.sh` ejecutó el binario real contra un upstream HTTP
determinista en loopback. El recorrido incluyó bootstrap, login administrativo,
secreto cifrado, publicación de configuración, emisión de AgentToken, una llamada
normal y streaming en los cinco niveles de concurrencia. Cada respuesta terminó
con `[DONE]`, conservó el alias lógico y no devolvió errores de ModelCairn.

La compuerta también comprobó que los canarios de prompt no aparecieran en el
directorio de datos y que el secreto del proveedor no apareciera en logs. El
binario verificó internamente que correspondía a la revisión evaluada antes de
comenzar, pero el informe público omite ese identificador.

## Interpretación

El pico fue aproximadamente 55,3 MiB, muy por debajo del presupuesto de 128 MiB y
con swap nulo. Los 20 streams concurrentes completaron correctamente, por lo que
esa concurrencia queda demostrada para este fixture en la VM objetivo. Las cifras
no predicen la latencia de proveedores externos: la red fue local y el upstream
simuló una entrega breve de dos partes.

La evidencia pública omite hostname, IP, usuario, rutas temporales, credenciales e
identificadores de revisión.
