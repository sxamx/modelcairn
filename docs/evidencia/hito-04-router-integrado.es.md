# Evidencia del router integrado del Hito 4

[English](hito-04-router-integrado.md)

- Estado: implementación de los Bloques 1–6 verificada; aceptación final pendiente.
- Fecha: 12 de septiembre de 2026.
- Superficie: `POST /v1/chat/completions`, normal y streaming SSE.

## Evidencia ejecutable

La suite automatizada demuestra:

- autenticación AgentToken y autorización de la ruta antes de revelar o contactar
  destinos;
- resolución del alias lógico mediante un snapshot publicado e inmutable;
- filtrado por capacidad, estado y cooldown;
- afinidad estable entre Credential y Egress;
- sustitución del modelo físico upstream y restauración del alias en respuestas;
- fallback secuencial acotado para `429`, fallos transitorios y respuestas
  inválidas elegibles;
- ausencia de fallback ante errores terminales o envíos ambiguos;
- validación SSE antes del compromiso y prohibición de mezclar destinos después;
- propagación de cancelación y resultado `partial` ante un corte comprometido;
- persistencia transaccional de Request, Attempt, observaciones de rate limit y
  cooldowns sin prompt, respuesta ni secreto.

La compuerta `scripts/verify-hito4.sh` levanta el binario real y un upstream
determinista en loopback. Realiza bootstrap, login, publicación de configuración,
emisión de AgentToken, una llamada normal y lotes de 1, 2, 5, 10 y 20 streams.
También mide RSS, swap y latencia, y busca canarios de contenido en el
almacenamiento local y secretos en los logs.

## Resultados actuales

- La suite Go completa, `go vet`, validadores documentales, escaneo de
  vulnerabilidades alcanzables y pruebas con detector de carreras pasan en CI.
- Las compilaciones Linux AMD64 y ARM64 pasan.
- La compuerta integral Linux pasa dentro de CI.
- Una ejecución local adicional completó los cinco niveles de concurrencia con un
  pico cercano a 59 MiB de RSS; esta cifra no sustituye la medición de la VM
  representativa.

## Pendientes para aceptar el hito

- incorporar y resolver el QA independiente agrupado;
- ejecutar la compuerta con duración representativa en la VM objetivo;
- publicar un informe redactado que omita hostname, IP, usuario, rutas,
  credenciales e identificadores de revisión;
- cerrar cualquier hallazgo y volver a ejecutar todas las puertas.

Esta evidencia no afirma todavía soporte de relay remoto, HTTP CONNECT, SOCKS,
editor visual, estimador adaptativo ni dialectos adicionales.
