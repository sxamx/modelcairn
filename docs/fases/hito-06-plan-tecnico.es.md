# Hito 6 — Instalación y recuperación

[English](hito-06-plan-tecnico.md)

Estado: completado el 15 de septiembre de 2026.
Prerrequisito: Hito 5 fusionado y aceptado.

## Resultado

Entregar una instalación nativa reproducible para Linux AMD64 y ARM64 y una ruta
de recuperación completa que permita reconstruir una instalación funcional en una
VM limpia. El operador elige durante la instalación si habilita inicio automático,
puede operar por localhost, red privada o Tailscale Serve y puede crear/verificar/
restaurar un backup MCB1 sin exponer contraseñas ni secretos.

## Límites

- `systemd` es la ruta principal; Docker continúa fuera de este hito.
- El instalador no abre puertos, no modifica el firewall y no instala Tailscale.
- Tailscale Serve se documenta y comprueba, pero sigue siendo administrado por el
  operador fuera de ModelCairn.
- MCB1 contiene la base consistente y secretos recifrables; no incluye claves TLS,
  SSH, logs del sistema ni la clave maestra original.
- Una restauración nunca reemplaza el estado activo antes de autenticar todo el
  archivo, validar límites, integridad, migraciones y permisos.

## Bloques de entrega

1. **Contrato de instalación:** rutas, usuario/grupo, permisos, prerequisitos,
   actualización, desinstalación conservadora y matriz de modos de acceso.
2. **Instalador nativo:** script idempotente, artefacto local o publicado, unidad
   `systemd` endurecida y elección explícita de inicio automático.
3. **Bootstrap guiado:** bienvenida de terminal, creación segura del administrador,
   ajustes mínimos y siguiente paso web; las contraseñas nunca son argumentos.
4. **Backup MCB1:** snapshot SQLite consistente, secretos en streaming, age/scrypt,
   tar acotado, escritura privada atómica y verificación posterior.
5. **Restore generacional:** preflight completo, nueva clave maestra, recifrado,
   invalidación de sesiones, activación atómica y rollback comprobado.
6. **Operación:** diagnóstico de servicio, health/readiness, logs, actualización
   manual y guías localhost/red privada/Tailscale Serve.
7. **Cierre:** instalación y restore desde cero en VM objetivo, prueba funcional de
   ruta recuperada, fallos inducidos, presupuesto RAM/disco, CI y QA agrupado.

## Avance comprobado

- [x] Contrato de instalación y matriz de acceso.
- [x] Instalador nativo y unidad `systemd`.
- [x] Bootstrap guiado sin secretos en argumentos.
- [x] Guía de localhost, red privada y Tailscale Serve.
- [x] Backup MCB1 y verificación sin restaurar.
- [x] Restore generacional y rollback.
- [x] Operación completa, prueba en VM, benchmark y cierre agrupado.

## Decisiones iniciales

- Layout recomendado: binario en `/usr/local/bin/modelcairn`, configuración de
  servicio en `/etc/modelcairn`, datos en `/var/lib/modelcairn` y unidad en
  `/etc/systemd/system/modelcairn.service`.
- El servicio usa una identidad de sistema dedicada, sin shell ni directorio home.
- El instalador acepta modo interactivo y opciones no interactivas documentadas;
  ninguna opción acepta contraseñas o API keys directamente.
- El inicio automático se recomienda, pero se habilita únicamente según la elección
  capturada por el instalador. Iniciar ahora y habilitar al arrancar son decisiones
  distintas.
- La migración al layout generacional se implementará de forma compatible con el
  layout actual; una instalación existente no se mueve silenciosamente.
- Los backups se verifican al crearlos. La verificación sin restaurar no escribe en
  el directorio de datos.

## Criterios de salida

1. Una VM soportada vacía pasa de binario a consola lista mediante un flujo guiado.
2. Repetir el instalador no destruye configuración ni datos y explica cada cambio.
3. Los permisos impiden lectura de claves por usuarios ajenos al servicio.
4. Los tres modos de acceso tienen instrucciones comprobadas sin alterar firewall.
5. Backup y restore cumplen límites MCB1 y nunca dejan plaintext secreto en disco.
6. Contraseña incorrecta, archivo truncado, falta de espacio o migración fallida
   conservan intacta la generación activa.
7. Después de restore, sesiones admin antiguas fallan y una ruta seleccionada
   funciona sin volver a introducir su API key.
8. Suite, CI, VM limpia, benchmark y QA agrupado terminan sin hallazgos críticos o
   altos abiertos.

## Orden de verificación

Las pruebas unitarias cubren parsers, límites y permisos; las integraciones usan
directorios temporales y procesos reales. La VM se usa una vez por lote para
instalación, reinicio, restore y medición. El QA independiente se ejecuta después
de los bloques funcionales, no tras cada modificación.
