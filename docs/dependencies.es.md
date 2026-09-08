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
| `golang.org/x/sys` v0.47.0 | build y ejecución desde Hito 2 | Bloqueos de archivo nativos y no bloqueantes en Windows; ya era transitiva, pasa a directa y usa licencia BSD-3-Clause |
| `golang.org/x/crypto` v0.56.0 | Cifrado de secretos desde Hito 2 | Implementación oficial de Go de XChaCha20-Poly1305; BSD-3-Clause; evita implementar el cifrador por nuestra cuenta |
| `gopkg.in/yaml.v3` v3.0.1 | Parsing de configuración desde Hito 2 | Parser YAML con árbol inspeccionable; permite rechazar aliases, tags, claves duplicadas y claves no string antes de canonizar; MIT/Apache-2.0 |
| `golang.org/x/vuln` v1.7.0 | Solo CI y desarrollo | `govulncheck` fijado para analizar vulnerabilidades alcanzables; BSD-3-Clause; no se enlaza al servidor |

Los módulos transitivos quedan fijados en `go.sum`; CI ejecuta `go mod tidy` y
rechaza cambios no confirmados en los archivos del módulo. El Hito 2 enlaza la
persistencia al arranque de la aplicación. `x/sys` podrá retirarse si la biblioteca
estándar de Go expone en el futuro la misma semántica portable de bloqueo no
bloqueante. El coste enlazado al producto se vuelve a medir en la puerta de
recursos del hito.

El almacén cifrado y su rotación están aceptados con medición representativa.
`x/crypto` podrá retirarse si la biblioteca estándar ofrece el mismo formato
XChaCha20-Poly1305 sin invalidar secretos persistidos. `yaml.v3` se mantiene
encapsulado en `internal/config` y puede sustituirse sin cambiar el modelo interno;
la suite hostil y de round-trip define el comportamiento que debe conservarse.

En cada hito que cambie dependencias, CI verifica `go mod tidy` limpio y ejecuta
un `govulncheck ./...` fijado; la licencia y el propósito de cada módulo directo se
registran aquí antes de aceptarlo.
