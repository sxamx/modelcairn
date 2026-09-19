# Modelo de amenazas de la Fase 1

[English](phase-1-threat-model.md)

- Estado: vigente para validación del Hito 7
- Alcance: nodo principal autohospedado, consola, API de datos, SQLite, llavero,
  backups MCB1 y egreso directo a proveedores
- Fuera de alcance: relays, estimador adaptativo y alta disponibilidad

## Activos y objetivos

Los activos prioritarios son API keys de proveedores, contraseña y sesiones
administrativas, AgentTokens, configuración/afinidades, backups y contenido en
tránsito. Deben conservar confidencialidad e integridad; configuración confirmada
y recuperación deben conservar disponibilidad razonable.

ModelCairn no pretende proteger una instancia frente a un administrador root del
host ya comprometido. Sí debe limitar exposición accidental, clientes no
autenticados, entradas hostiles, proveedores maliciosos y fallos del proceso.

## Límites de confianza

1. **Cliente → API de datos:** el cliente no es confiable hasta validar AgentToken,
   tamaño, JSON, modelo y capacidades.
2. **Navegador/CLI → API administrativa:** contraseña, sesión y CSRF son distintos
   de AgentToken. Un rol no concede el otro.
3. **Proceso → almacenamiento local:** SQLite y llavero requieren propietario y
   permisos exclusivos; archivos, YAML y backups importados son hostiles.
4. **Nodo → proveedor:** URL, DNS, redirects, headers, errores y cuerpos del
   proveedor no son confiables. El proveedor recibe necesariamente el prompt y la
   credencial que requiere esa solicitud.
5. **Operador → red:** loopback HTTP es local; acceso privado/público requiere los
   modos TLS/proxy documentados. ModelCairn no convierte HTTP remoto en seguro.

No existe límite de relay en Fase 1. Añadirlo exigirá revisar este modelo antes de
implementar RF-101–RF-108.

## Amenazas y controles exigidos

| Amenaza | Control de Fase 1 | Evidencia principal |
|---|---|---|
| Robo de API key desde disco/salida | XChaCha20-Poly1305 ligado a identidad, llavero separado, redactor común, API solo de escritura | pruebas `secret_crypto`, `secret_store`, `redact`; ADR-0005 |
| Fuerza bruta administrativa | Argon2id acotado, admisión global/cliente, backoff, error uniforme y estadística agregada | pruebas `admin_login` y `failed_login_statistics` |
| Robo/fijación de sesión y CSRF | cookie protegida según transporte, expiración idle/absoluta, ventana CSRF y revocación atómica | pruebas `admin_session` y contrato de sesiones |
| AgentToken robado o excesivo | hash en reposo, entrega única, rutas permitidas, expiración y revocación | pruebas `agent_token` |
| SSRF o filtración a un destino | esquemas y red privada controlados, resolución validada, redirect rechazado, credencial inyectada después de validar | pruebas `parse`, `direct_transport` y `openaiadapter` |
| Retry/fallback amplificador | presupuesto de intentos/tiempo, clasificación allowlist, cancelación y prohibición después de comprometer stream | pruebas `router` y `streaming` |
| Contenido o secreto en historial/log | tablas sin campos de contenido, flags fijados a cero, logs con rutas tipadas, canarios y redacción | pruebas `operational`, `server` y `redact` |
| Configuración concurrente o manipulada | plan firmado de un uso, versión optimista, transacción y propietario exclusivo | pruebas `plan_token`, `config` y `lifecycle` |
| Migración/rotación/restore interrumpido | transacciones, publicación generacional, clave autenticada, verificación previa y rollback | pruebas `lifecycle`, `rotation`, `backupmcb1`; evidencia Hito 6 |
| Backup leído o reemplazado | cifrado autenticado con passphrase, creación sin reemplazo, límites de archivo y restauración en generación nueva | pruebas `mcb1`; contrato MCB1 |
| Consola/API abusada | autenticación obligatoria, límites de entrada, paginación y concurrencia optimista | pruebas HTTP, almacenamiento y web |

## Privacidad y datos

- No existe telemetría externa incorporada.
- El [inventario de egresos](egress-inventory.json) enumera cada archivo de
  producción capaz de iniciar red; CI falla ante un punto nuevo no declarado.
- Prompts y respuestas no se persisten por defecto.
- Metadatos operativos permanecen localmente hasta que una política implementada
  los elimine; el mapa de retención declara qué controles siguen diferidos.
- Exportar, respaldar o copiar logs es una acción del operador y amplía el lugar
  donde debe protegerlos.

## Riesgos residuales aceptados

- El nodo principal es punto único de fallo.
- Un root comprometido puede leer memoria o reemplazar el binario.
- El proveedor conoce el contenido enviado y puede registrar su tráfico.
- El HTTP de loopback depende de la seguridad del host; la exposición remota sin
  TLS está prohibida, no corregida mágicamente por la aplicación.
- La retención general configurable es RF-202; hasta entonces el operador debe
  vigilar disco y administrar copias conforme al runbook.

Un hallazgo que exponga secretos, permita saltar autenticación/autorización, eluda
la política de red o invalide recuperación bloquea el cierre de la Fase 1.
