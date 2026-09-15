# Evidencia del Hito 6 — instalación y recuperación

[English](hito-06-installation-and-recovery.md)

Estado al 15 de septiembre de 2026: implementación completa; cierre operativo en
curso. Esta evidencia no declara terminado el hito mientras falte la ruta recuperada
en la VM y el cierre de la instalación de prueba.

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
Tras reiniciar la VM, `systemd` lo inició automáticamente, health y readiness
respondieron y el consumo observado fue aproximadamente 37,2 MiB, dentro del límite
de 128 MiB. Son observaciones puntuales; el benchmark final debe registrar duración,
muestras y picos.

## Hallazgo durante la instalación real

Otro servicio ya escuchaba en el puerto predeterminado. La primera versión del
instalador podía informar éxito aunque ModelCairn terminara después por la colisión.
Se corrigió para comprobar que el proceso permanezca activo durante el arranque,
detener el bucle de reinicios y devolver un diagnóstico. La repetición confirmó que
el puerto ocupado se rechaza y que ModelCairn queda detenido. La instalación limpia
continuó en un puerto loopback libre sin alterar el servicio preexistente.

## Pendiente para cerrar

- completar ruta → backup → restore → ruta sin reintroducir el secreto;
- comprobar contraseña incorrecta, archivo truncado, invalidación de sesión y rollback;
- medir durante una ventana definida y documentar promedio, pico y swap;
- decidir y ejecutar la limpieza conservadora de los artefactos de prueba.
