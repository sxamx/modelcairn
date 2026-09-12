# Benchmark representativo del Hito 3

[English](benchmark-hito-03-2026-09-12.md)

- Estado: aprobado en VM objetivo
- Fecha: 12 de septiembre de 2026
- Entorno: Linux x86_64, 2 CPU lógicas, 975.064 KiB de RAM total
- Binario reducido: 13.103.264 bytes
- Flujo: bootstrap, login Argon2id, recuperación CSRF, secreto, config
  plan/apply/export, AgentToken issue/revoke y logout
- Estado estable adicional: 120 segundos
- Muestras: 2.136, aproximadamente cada 50 ms
- RSS promedio del servicio: 55.358 KiB
- RSS máximo del servicio: 55.368 KiB
- Swap máximo del servicio: 0 KiB
- Memoria disponible del sistema después del flujo: 512.692 KiB

El muestreo cubrió el login para capturar la asignación Argon2id y mantuvo el
servidor administrativo real en ejecución. La prueba integrada también confirmó
que contraseña y API key no aparecieron en logs/export y que la sesión local fue
eliminada después de logout. La VM no tenía Go: se usó un binario Linux precompilado
cuya revisión fue verificada por el gate antes de ejecutar.

La evidencia pública omite hostname, IP, usuario, rutas temporales, credenciales e
identificadores de revisión. La cifra mide RSS del proceso, no page cache del kernel
ni memoria de otros servicios. Con un pico cercano a 54,1 MiB, el plano
administrativo mantiene margen amplio dentro de la VM de aproximadamente 1 GiB.
