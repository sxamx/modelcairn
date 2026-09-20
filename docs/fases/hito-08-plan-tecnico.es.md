# Hito 8 — Preparación de la primera publicación open source

[English](hito-08-plan-tecnico.md)

- Estado: **completado; v0.1.0 publicada y verificada como prerelease**
- Etapa: G — preparación open source y publicación
- Objetivo: convertir la Fase 1 aceptada en una candidata instalable, verificable
  y recuperable sin confundirla con una versión estable.

Este hito prepara y prueba la cadena de release. No implementa relays, estimador
adaptativo, protocolos adicionales ni editor visual. Tampoco crea tags o una
GitHub Release hasta que el mantenedor apruebe versión y canal.

## Decisiones aprobadas por el mantenedor

El 20 de septiembre de 2026 se aprobaron:

1. `v0.1.0` como *prerelease*, sin promesa de estabilidad.
2. Tarballs Linux AMD64/ARM64, `SHA256SUMS`, SBOM SPDX, atestaciones y los
   archivos fuente automáticos de GitHub; sin contenedor.
3. `sxamx` como identidad pública del autor/mantenedor.
4. GitHub Private Vulnerability Reporting como contacto de seguridad, sin correo
   personal publicado.
5. Protección de `main`, tags `v*` y environment `release` con aprobación
   manual. Estas protecciones quedaron activadas antes de generar la candidata.

La autorización final se concedió después de aprobar estas decisiones. El tag y
la GitHub Release se crearon mediante la barrera protegida y los assets públicos
se verificaron.

Las demás decisiones de implementación se consideran reversibles y quedan fijadas
por este plan.

## 1. Contrato de artefactos

Cada candidata se construye desde un commit de `main` con CI verde. La publicación
crea el tag únicamente después de validar los mismos bytes. Los artefactos previstos
son:

- `modelcairn_<version>_linux_amd64.tar.gz`;
- `modelcairn_<version>_linux_arm64.tar.gz`;
- `SHA256SUMS`;
- SBOM SPDX JSON por artefacto o una SBOM que identifique ambos binarios;
- atestación de procedencia generada por GitHub Actions cuando esté disponible;
- notas de release con cambios, compatibilidad, instalación, actualización,
  rollback, limitaciones y evidencia.

Cada archivo comprimido contiene el binario, `LICENSE`, `NOTICE`, README y un
manifiesto de versión. Los binarios incorporan versión, commit y fecha UTC mediante
`buildinfo`; `modelcairn version` debe coincidir con el tag y el manifiesto.

## 2. Workflow de candidata

Se añadirá un workflow manual y reutilizable que acepte una versión sin publicar:

1. comprueba formato SemVer y que el commit pertenece a `main`;
2. ejecuta las mismas puertas de CI, no solo un build;
3. genera frontend desde lockfile y exige árbol limpio;
4. compila con `CGO_ENABLED=0`, `-trimpath` y metadatos de versión;
5. empaqueta de forma determinista en AMD64 y ARM64;
6. genera checksums, SBOM y procedencia;
7. prueba cada archivo en una instalación temporal Linux;
8. sube artefactos de Actions, nunca una release pública automáticamente.

Un workflow separado y manual publica exactamente los artefactos privados de una
candidata aprobada. Exige el entorno protegido `release`, confirmación textual,
run exitoso del mismo commit de `main`, checksums y atestaciones válidas. La
publicación no recompila ni acepta archivos aportados por el operador.

## 3. Instalación, actualización y rollback desde paquete

La validación representativa cubrirá:

- instalación vacía desde el tarball correcto para la arquitectura;
- bootstrap y ruta normal/SSE con credenciales temporales;
- backup MCB1 verificado antes de actualizar;
- actualización desde la última candidata soportada conservando datos y permisos;
- arranque, readiness y ruta funcional después de actualizar;
- rollback del binario y, si hubo migración compatible, reapertura de los datos;
- fallo seguro por arquitectura, checksum, archivo truncado o metadatos incorrectos;
- desinstalación conservadora sin borrar `/var/lib/modelcairn`.

Hasta existir una candidata anterior, la primera ejecución prueba reinstalación
idempotente del mismo paquete y rollback al binario previo de la misma revisión.

## 4. Documentación pública

- `CHANGELOG.md` adopta Keep a Changelog y SemVer, comenzando por `Unreleased`.
- La guía de instalación usa descargas explícitas y verificación SHA-256; no ofrece
  `curl | sh`.
- `SECURITY.md` distingue ramas/versiones soportadas sin prometer SLA.
- `CONTRIBUTING.md` deja de afirmar que Fase 1 está en validación.
- Las notas enumeran límites reales: sin relays, estimador adaptativo, protocolos
  Anthropic/Google ni editor visual.
- Se añade política de marcas antes de llamar estable a una release; Apache-2.0 y
  NOTICE no conceden derecho a presentarse como publicación oficial.

## 5. Seguridad de la cadena

- permisos mínimos y acciones fijadas por SHA;
- publicación mediante entorno GitHub protegido, sin PAT persistente;
- OIDC para atestación; ningún secreto de proveedor en builds o pruebas;
- checksum verificado antes de instalar y canarios ausentes de artefactos/logs;
- inventario de dependencias y `govulncheck` actualizado en la revisión final;
- revisión de historial y archivos empaquetados para rutas, hosts e identidades
  privadas antes de publicar.

La firma criptográfica adicional con Sigstore queda recomendada si la integración
sin secretos es viable; de no serlo, checksums y atestación de GitHub son el mínimo
de esta primera prerelease.

## 6. Aceptación y orden de ejecución

1. Fijar contrato, checklist y decisiones pendientes.
2. Implementar empaquetado reproducible y pruebas locales.
3. Añadir workflow de candidata sin permisos de publicación.
4. Ejecutar candidata en AMD64 y comprobar por cross-build ARM64.
5. Probar instalación/actualización/rollback en la VM representativa.
6. Realizar un único QA independiente agrupado.
7. Corregir bloqueos y producir el informe de preparación.
8. Solicitar aprobación explícita de versión/canal.
9. Solo entonces crear tag y GitHub Release; verificar assets publicados desde
   una máquina limpia.

El hito puede declarar **candidata lista** antes del paso 8, pero solo se completa
cuando la publicación autorizada queda verificada o el mantenedor decide cerrar el
hito sin publicar. Nunca se crea una release para “probar” el workflow.
