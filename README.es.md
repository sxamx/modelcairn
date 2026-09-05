# ModelCairn

> Estado: documentación y planificación de la Fase 1 aprobadas; Hito 1 preparado.
> El prototipo recuperado se conserva fuera de este repositorio como referencia.

[English](README.md)

ModelCairn es un gateway de proveedores de IA ligero, autohospedado y configurable.
Ofrece un endpoint estable, routing y fallback explicables, métricas locales y una
estimación adaptativa de capacidad y recuperación. El operador mantiene control de
sus datos, credenciales, proveedores y egresos; ModelCairn no envía telemetría a un
servicio central.

El nombre alude a un *cairn*: cada observación funciona como una piedra que mejora
la señal de la ruta sin fingir que una estimación es una regla oficial.

## Estado del proyecto

No existe todavía una release. La base documental y los contratos ejecutables de
la Fase 1 están completos; a continuación comienza el desarrollo incremental en Go.
Consulta:

- [Estado y ciclo del proyecto](docs/estado-y-ciclo-del-proyecto.es.md)
- [Project Charter](docs/project-charter.es.md)
- [Índice de documentación](docs/README.es.md)
- [Fase 1 — Fundación operable](docs/fases/fase-01-fundacion.es.md)

## Dirección técnica aprobada

- Backend Go como monolito modular.
- Consola TypeScript/React/Vite incrustada como activos estáticos.
- SQLite como fuente de verdad y YAML para validate/plan/apply/export.
- Ejecutable Linux administrado por systemd; AMD64 y ARM64.
- API inicial `POST /v1/chat/completions`, incluido streaming y tool calls dentro
  de una matriz explícita.
- API keys centralizadas y cifradas en el nodo principal.
- Prompts y respuestas no se almacenan por defecto.
- Relays de salida ligeros en una fase posterior, sin persistir API keys.

## Licencia

ModelCairn se distribuye bajo [Apache License 2.0](LICENSE) e incluye
[NOTICE](NOTICE) con su atribución de origen.
