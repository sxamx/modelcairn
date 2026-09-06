# Política e inventario de dependencias

[English](dependencies.md)

ModelCairn minimiza dependencias de ejecución para proteger el objetivo de una VM
de 1 GB y la compilación cruzada. Toda dependencia directa requiere una capacidad
concreta, revisión de mantenimiento y licencia, evidencia del impacto en recursos y
un plan de retirada.

## Inventario actual

| Dependencia | Alcance | Propósito |
|---|---|---|
| Biblioteca estándar de Go | build y ejecución | CLI, ciclo HTTP, logging, sincronización y pruebas |
| `modernc.org/sqlite` v1.58.0 | build y ejecución desde Hito 2 | Driver SQLite sin CGO; permite compilaciones cruzadas Linux AMD64/ARM64 y usa licencia BSD-3-Clause |

El driver SQLite es el único módulo Go directo de terceros. Sus módulos transitivos
quedan fijados en `go.sum`; CI ejecuta `go mod tidy` y rechaza cambios no confirmados
en los archivos del módulo. El spike del Hito 1 compila y prueba el driver, mientras
que el ejecutable no lo enlazará hasta que la persistencia forme parte del arranque.
El coste medido en binario y memoria debe registrarse antes de cerrar el Hito 1.

Las bibliotecas criptográficas se añadirán únicamente en el hito que pruebe sus
contratos aprobados.
