# Evidencia del Hito 6 — instalación y recuperación

[English](hito-06-installation-and-recovery.md)

Estado al 15 de septiembre de 2026: implementación, recorrido operativo y limpieza
conservadora completos. El cierre queda sujeto únicamente al CI final y la fusión.

## Compuertas automatizadas

- Suite Go completa, `go vet`, análisis de vulnerabilidades y contratos: aprobados.
- Consola React: 14 pruebas, generación tipada, build y paquete embebido: aprobados.
- Compilación Linux AMD64 y ARM64: aprobada.
- Compuertas de integración de los Hitos 2, 3 y 4, concurrencia de streaming y
  presupuesto básico: aprobadas en CI.
- Casos MCB1 añadidos: destino concurrente no reemplazable, límite de 4096 secretos
  y activación segura cuando falla el `fsync` posterior al enlace.

## VM objetivo de 1 GB

La VM de prueba tiene 952 MiB de RAM utilizable. Se instaló el artefacto AMD64 con
usuario dedicado, inicio automático y escucha loopback. Repetir el instalador
conservó el estado. Los permisos observados coinciden con el contrato:

| Elemento | Propietario | Modo |
|---|---|---:|
| Binario | `root:root` | `0755` |
| Directorio de servicio | `root:modelcairn` | `0750` |
| Variables de servicio | `root:modelcairn` | `0640` |
| Directorio de datos | `modelcairn:modelcairn` | `0700` |
| Clave maestra y SQLite | `modelcairn:modelcairn` | `0600` |
| Unidad systemd | `root:root` | `0644` |

El proceso consumió aproximadamente 5,8 MiB inmediatamente después de instalarse.
Tras reiniciar la VM, `systemd` lo inició automáticamente y health y readiness
respondieron. Una ventana posterior de 120 segundos reunió 59 muestras mientras se
consultaban ambos endpoints: promedio 15,8 MiB, pico 28,7 MiB y 0 KiB de swap. El
binario midió 13,4 MiB y el estado configurado, aproximadamente 312 KiB. El pico
queda ampliamente dentro del límite de 128 MiB.

## Hallazgo durante la instalación real

Otro servicio ya escuchaba en el puerto predeterminado. La primera versión del
instalador podía informar éxito aunque ModelCairn terminara después por la colisión.
Se corrigió para comprobar que el proceso permanezca activo durante el arranque,
detener el bucle de reinicios y devolver un diagnóstico. La repetición confirmó que
el puerto ocupado se rechaza y que ModelCairn queda detenido. La instalación limpia
continuó en un puerto loopback libre sin alterar el servicio preexistente.

## Recuperación funcional

Sobre la instalación configurada se comprobó el recorrido completo:

- creación y verificación del backup MCB1, sin hallar la canaria secreta en el
  archivo cifrado;
- contraseña incorrecta y archivo truncado rechazados sin cambiar la generación;
- ruta Chat Completions funcional contra un upstream local antes del restore;
- restore hacia una generación nueva e invalidación de la sesión administrativa
  capturada antes de restaurar;
- la misma ruta volvió a responder sin reintroducir el secreto del proveedor;
- rollback seleccionó exactamente la generación predecesora y recuperó readiness.

Las contraseñas administrativas y de backup fueron aleatorias, existieron solo en
memoria durante la prueba y no se publicaron. El upstream, la API key y los prompts
eran canarias ficticias.

## Limpieza conservadora

El desinstalador retiró la unidad y el binario y se eliminaron únicamente los
temporales creados por la prueba. Los dos directorios de datos de desarrollo se
conservaron con propietario `modelcairn:modelcairn` y modo `0700`; no se borraron ni
se publicaron. CI final y fusión se comprueban fuera de esta evidencia de VM.
