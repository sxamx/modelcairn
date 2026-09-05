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

Actualmente no existen módulos Go de terceros. SQLite y las bibliotecas
criptográficas se añadirán únicamente en el hito que pruebe sus contratos aprobados.
