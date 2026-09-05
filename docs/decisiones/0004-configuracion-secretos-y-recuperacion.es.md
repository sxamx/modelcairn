# ADR-0004: Configuración, secretos y recuperación

[English](0004-configuracion-secretos-y-recuperacion.md)

- Estado: aceptada
- Fecha: 2026-09-05
- Responsables: mantenedor principal y diseño técnico

## Contexto

La configuración debe administrarse tanto desde archivos como desde la consola sin
crear dos estados contradictorios. Las API keys son el activo local prioritario y
una restauración debe poder recuperar una instalación funcional.

## Decisión

### Fuente de verdad

- Una base SQLite local y versionada será la fuente de verdad del nodo principal.
- La consola y CLI modificarán el estado mediante el mismo contrato de validación.
- YAML será el formato humano para importar, aplicar, exportar y automatizar
  configuración.
- La primera versión no observará ni recargará continuamente un archivo editable.
  Un cambio se aplica explícitamente mediante CLI o API y produce auditoría.

### Secretos en operación

- Las API keys se cifrarán antes de persistirlas.
- El instalador generará una clave maestra aleatoria y la guardará en un archivo
  accesible solo por la identidad del servicio.
- La clave maestra no se mostrará en la consola ni aparecerá en exportaciones
  normales, logs o métricas.
- Una clave maestra ausente o incorrecta hará que el almacén de secretos falle de
  forma cerrada: no se iniciarán rutas que dependan de valores indescifrables.

Esta protección reduce el impacto de copiar únicamente la base de datos, pero no
promete proteger secretos frente a una persona con control administrativo total de
la VM. Aumentar ese nivel requeriría otro modelo operativo.

### Exportación y backup

- La exportación normal contendrá configuración sin secretos y podrá versionarse.
- El backup completo podrá incluir configuración, datos y API keys.
- Antes de incluir secretos, el backup completo se cifrará usando una contraseña
  elegida por el operador y separada de la contraseña administrativa.
- Una restauración con contraseña incorrecta fallará antes de aplicar datos.
- La prueba de restauración deberá demostrar que una ruta recuperada puede volver a
  realizar una llamada sin reingresar su API key.
- No se sobrescribirá una instalación activa sin preflight, backup previo y
  confirmación explícita.

### Recuperación administrativa

La contraseña administrativa podrá restablecerse únicamente desde una CLI local
ejecutada con permisos suficientes en la VM. No dependerá de correo ni de un
servicio externo.

## Consecuencias

- Copiar el YAML no constituye un backup completo.
- Perder simultáneamente la VM, la clave maestra y la contraseña del backup puede
  hacer irrecuperables los secretos, lo cual deberá explicarse durante onboarding.
- SQLite deberá probarse con el volumen de eventos, patrón de escritura y retención
  esperados. Si no cumple, podrá reemplazarse mediante otro ADR.
- Las herramientas de backup deben evitar exponer secretos en argumentos de
  procesos, historiales de shell o archivos temporales.

## Fuentes técnicas

- [Descripción oficial de SQLite](https://www.sqlite.org/about.html)
- [Usos apropiados y límites de concurrencia de SQLite](https://www.sqlite.org/whentouse.html)
