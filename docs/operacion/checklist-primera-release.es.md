# Checklist de la primera release

[English](first-release-checklist.md)

Esta lista es una puerta operativa. Marcar un punto exige evidencia enlazada; no
basta con que el comando haya funcionado una vez.

## Candidata

- [ ] Versión y canal aprobados por el mantenedor.
- [x] Commit de `main`, árbol limpio y CI completo verde.
- [x] Binarios AMD64/ARM64 identifican versión, commit y fecha correctos.
- [x] Tarballs contienen solo archivos allowlisted y modos esperados.
- [x] `SHA256SUMS`, SBOM y procedencia corresponden a los bytes candidatos.
- [x] Escaneo de secretos/identificadores privados y vulnerabilidades aprobado.

## Operación

- [x] Instalación limpia desde el tarball aprobada.
- [x] Bootstrap, login, ruta normal y SSE aprobados.
- [x] Backup previo a actualización creado y verificado.
- [x] Actualización conserva datos, permisos, inicio automático y readiness.
- [x] Rollback funcional y backup restaurable comprobados.
- [x] Checksum incorrecto, archivo truncado y arquitectura errónea fallan sin
      modificar la instalación activa.

## Documentación y revisión

- [x] README, changelog, instalación, actualización, rollback y limitaciones
      coinciden con la candidata.
- [x] Licencia, NOTICE, contribución, seguridad y política de marca presentes.
- [x] Notas enlazan evidencia y no afirman estabilidad ni funciones diferidas.
- [x] QA agrupado independiente sin bloqueos.

## Publicación autorizada

- [ ] Aprobación explícita del mantenedor registrada.
- [ ] Tag apunta al commit aprobado y activa el entorno protegido.
- [ ] GitHub Release contiene exactamente los hashes candidatos.
- [ ] Descarga limpia, checksum, instalación y `modelcairn version` verificados.
- [ ] Tablero, ciclo y documentación actualizados después —no antes— de comprobar
      los assets públicos.
