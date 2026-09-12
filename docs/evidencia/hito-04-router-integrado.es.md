# Evidencia del router integrado del Hito 4

[English](hito-04-router-integrado.md)

- Estado: implementación y QA de los Bloques 1–6 verificados; aceptación final pendiente.
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

La matriz HTTP integrada induce `429`, `503`, respuesta JSON inválida, error
terminal, timeout y cancelación. Comprueba intentos, motivos de fallback,
cooldowns y outcomes persistidos. También demuestra fallback SSE únicamente antes
del compromiso, corte parcial sin segundo destino y round trip de tool calls.

## QA independiente agrupado

La revisión independiente detectó validación superficial de tool calls, métricas
de tokens/TTFT no conectadas y JSON upstream ambiguo. Los hallazgos se corrigieron
con validación estructural, propagación de métricas hasta SQLite y rechazo de
claves duplicadas, UTF-8 inválido y profundidad excesiva en respuestas upstream.
Las regresiones correspondientes forman parte de la suite.

## Resultados actuales

- La suite Go completa, `go vet`, validadores documentales, escaneo de
  vulnerabilidades alcanzables y pruebas con detector de carreras pasan en CI.
- Las compilaciones Linux AMD64 y ARM64 pasan.
- La compuerta integral Linux pasa dentro de CI.
- Una ejecución local adicional completó los cinco niveles de concurrencia con un
  pico cercano a 59 MiB de RSS; esta cifra no sustituye la medición de la VM
  representativa.

## Pendientes para aceptar el hito

- ejecutar la compuerta con duración representativa en la VM objetivo;
- publicar un informe redactado que omita hostname, IP, usuario, rutas,
  credenciales e identificadores de revisión;
- volver a ejecutar todas las puertas sobre la revisión final.

Esta evidencia no afirma todavía soporte de relay remoto, HTTP CONNECT, SOCKS,
editor visual, estimador adaptativo ni dialectos adicionales.
