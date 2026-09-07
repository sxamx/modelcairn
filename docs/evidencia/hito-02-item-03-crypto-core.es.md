# Hito 2 — Avance del núcleo de cifrado de secretos

[English](hito-02-item-03-crypto-core.md)

Estado: implementación parcial; el punto de entrega 3 **no está aceptado**.

Implementado: cifrado y descifrado XChaCha20-Poly1305 que autentican instalación,
identidad del secreto y versiones; límites de bytes UTF-8; fingerprints locales
con clave derivada para ese propósito. Son funciones internas, todavía no una API
utilizable de almacén de secretos.

Validación en el equipo de desarrollo Windows AMD64:

- `go test ./...`, `go vet ./...` y `node scripts/validate-docs.cjs`: correctos.
- La revisión estática independiente encontró carencias de cobertura en
  fingerprints, contexto y entradas malformadas. Se añadieron un vector de
  fingerprint calculado por separado con HMAC de .NET, un vector de bytes del
  contexto, comparación entre secretos distintos y rechazo de nonce alterado,
  claves inválidas, ciphertext sobredimensionado/truncado y UTF-8 inválido con
  autenticación válida. QA no evaluó persistencia ni rotación.
- `go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 -show verbose ./...`:
  sin vulnerabilidades alcanzables ni hallazgos en paquetes importados. El aviso
  de módulo [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) corresponde a
  `golang.org/x/crypto/openpgp`, que no usamos: ModelCairn importa
  `chacha20poly1305`, no OpenPGP. El análisis queda configurado en CI; aquí no se
  afirma que CI ya se haya ejecutado.
- Un ensayo de 100 iteraciones de `BenchmarkSecretSeal` informó 274 B/op para
  secretos de 32 bytes y 18.656 B/op para 16.384 bytes, con seis asignaciones por
  operación. No es RSS de la VM, rendimiento estable ni aceptación del hito.

Pendiente: llavero privado durable, inicialización y verificación de claves al
arrancar, repositorio de solo metadatos, rotación transaccional y limpieza de
claves obsoletas, ocultación común de secretos en salidas, pruebas de interrupción,
ejecución Linux y medición en la VM representativa. Este avance no demuestra esas
propiedades.
