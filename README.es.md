# ModelCairn

> Estado: Fase 1 aceptada; Hitos 0 a 7 completados.
> El prototipo recuperado se conserva fuera de este repositorio como referencia.

[English](README.md)

ModelCairn es un gateway de proveedores de IA ligero, autohospedado y configurable.
Ofrece un endpoint estable, routing y fallback explicables y métricas operativas
locales. La estimación adaptativa de capacidad y recuperación está planificada,
pero no implementada en esta prerelease. El operador mantiene control de
sus datos, credenciales, proveedores y egresos; ModelCairn no envía telemetría a un
servicio central.

El nombre alude a un *cairn*: cada observación funciona como una piedra que mejora
la señal de la ruta sin fingir que una estimación es una regla oficial.

## Estado del proyecto

No existe todavía una release. La Fase 1 está implementada, validada y aceptada.
La Etapa G prepara una primera candidata verificable.
Consulta:

- [Estado y ciclo del proyecto](docs/estado-y-ciclo-del-proyecto.es.md)
- [Project Charter](docs/project-charter.es.md)
- [Índice de documentación](docs/README.es.md)
- [Fase 1 — Fundación operable](docs/fases/fase-01-fundacion.es.md)
- [Hito 8 — Preparación de la primera release](docs/fases/hito-08-plan-tecnico.es.md)

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
[NOTICE](NOTICE) con su atribución de origen. Las distribuciones modificadas deben
seguir además la [guía del nombre y las marcas](TRADEMARKS.md) y no pueden insinuar
que son oficiales.

Los cambios destacables se registran en [CHANGELOG.md](CHANGELOG.md).

## Desarrollo

Los Hitos 0 a 7 están completos. Con Go instalado:

```sh
go test ./...
go run ./cmd/modelcairn serve
```

El flujo offline de configuración también está disponible durante el desarrollo:

```sh
go run ./cmd/modelcairn config validate config.yaml
go run ./cmd/modelcairn config plan --data-dir ./data --out plan.json config.yaml
go run ./cmd/modelcairn config apply --data-dir ./data --plan plan.json config.yaml
```

Consulte el [contrato de CLI offline](docs/contratos/cli-v1.es.md) para conocer
los flujos de configuración y secretos, sus protecciones y códigos de salida.

El servidor de desarrollo escucha en `127.0.0.1:8080` por defecto. `/healthz`
informa que el proceso está vivo; `/readyz` permanece no disponible hasta que la
persistencia, configuración y el almacén de secretos se inicialicen en sus hitos.
