# Preparación del repositorio limpio

[English](preparacion-del-repositorio-limpio.md)

- Estado: clon limpio y allowlist completados; commit y push pendientes
- Objetivo: publicar ModelCairn sin el historial ni los archivos descartados del
  prototipo anterior

## Restricción

El espacio de trabajo local anterior conserva el historial Git del prototipo. Este
directorio `modelcairn` fue clonado desde el remoto vacío y no contiene esa historia.

## Procedimiento seguro propuesto

1. Renombrar el repositorio remoto a `modelcairn`. **Completado.**
2. Autenticar `gh` con la cuenta pública correcta. **Completado.**
3. Verificar localmente `user.name` y `user.email` sin publicarlos en documentos.
4. Conservar el directorio anterior como archivo local. **Completado.**
5. Clonar el repositorio remoto vacío en un directorio nuevo `modelcairn`. **Completado.**
6. Copiar mediante allowlist únicamente la documentación vigente y archivos
   públicos aprobados.
   **Completado.**
7. Añadir `LICENSE`, `NOTICE`, `.gitignore`, README e índice. **Completado.**
8. Buscar secretos, rutas locales, identificadores privados y referencias obsoletas del producto.
9. Validar JSON Schema, OpenAPI, SQL, enlaces y Markdown.
10. Crear un único primer commit documental con la identidad correcta.
11. Revisar el diff y el commit local antes de solicitar autorización para push.

Mover el directorio es recuperable; no se eliminará el archivo local ni el backup
existente. El push será una acción separada y explícita.

## Allowlist documental inicial

- `README.md`, `LICENSE`, `NOTICE`, `.gitignore`;
- `docs/README.es.md`;
- Charter, estado, glosario, requisitos y arquitectura objetivo;
- ADR-0001 a ADR-0006;
- auditoría inicial pública y matriz de disposición;
- todos los documentos y contratos de `docs/fases/` y `docs/contratos/` vigentes.

No se copiarán código, base de datos, capturas, archivos de conversación ni
documentos del prototipo salvo revisión y decisión específica durante cada hito.
