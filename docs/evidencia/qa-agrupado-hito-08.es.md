# QA agrupado del Hito 8

[English](qa-grouped-milestone-08.md)

- Fecha: 20 de septiembre de 2026
- Alcance: cadena de candidata, paquetes, recuperación, documentación y barrera
  de publicación
- Dictamen técnico: **aprobado para decisiones del mantenedor**
- Publicación: **no autorizada**

La revisión independiente no encontró hallazgos P0. Detectó que la primera
evidencia no demostraba restore real, conservación del autoarranque ni rechazos
por corrupción/arquitectura; también encontró una IP privada en una prueba, menor
paridad entre CI y candidata, cobertura incompleta de la SBOM y ausencia de una
barrera de promoción.

Todos los hallazgos técnicos se corrigieron y revalidaron. La IP se sustituyó por
una dirección ficticia; el arnés Linux restauró MCB1 en una generación aislada y
conservó el estado enabled; los casos de checksum incorrecto, truncado y
arquitectura errónea fallaron; candidata y CI comparten puertas estáticas; la SBOM
quedó cubierta por checksum y atestación; y el workflow de publicación solo
promueve bytes de un run privado exitoso del mismo commit de `main`.

El riesgo residual ARM64 permanece explícito: el paquete se construyó e inspeccionó,
pero no se ejecutó en hardware ARM64. La política para migraciones irreversibles
también queda diferida hasta que exista una migración de ese tipo.

No son defectos técnicos pendientes: el mantenedor aún debe decidir versión,
canal, assets públicos, identidad/contacto y protecciones de GitHub. Ningún tag o
GitHub Release debe existir antes de esa aprobación.
