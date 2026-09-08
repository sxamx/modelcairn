# Hito 2 — Rotación de clave maestra

[English](hito-02-rotation.md)

Estado: implementada; pruebas locales, CI y ejecución representativa aprobadas.

`RotateMasterKey` publica una clave privada de forma durable, recifra fila por
fila en una transacción, actualiza fingerprints y autenticación de instalación,
y confirma también la auditoría de éxito. Verifica los datos persistidos antes
de retirar claves sin referencias. `CollectUnusedKeys` admite reintentos y limpia
temporales de claves abandonados. Mantiene versiones lógicas y fechas de secretos.

Las pruebas de desarrollo Windows cubren doce terminaciones reales de procesos:
creación temporal, sync del archivo, renombrado, sync del directorio, una fila
recifrada parcialmente, antes del commit, después del commit, filas verificadas,
borrado de clave antigua, sync de GC, borrado temporal y sync final. Cada reinicio
descifra los valores, comprueba fingerprints y auditoría y conserva la clave necesaria.

También se prueban rollback al fallar la auditoría, instalaciones vacías, claves
no activas referenciadas, prueba de autenticación ausente, corrupción y claves
ausentes tras commit, commit cancelado y callbacks que vuelven a llamar al almacén.
Detectar un estado de clave inválido deshabilita operaciones hasta reabrir,
incluso cuando se descubre antes de iniciar la rotación.

QA estático independiente señaló pruebas faltantes, temporales abandonados,
bloqueo de callbacks y escrituras tras detectar una clave activa ausente. Se
añadieron regresiones y correcciones. También se corrigió un error previo que
podía ignorar fallos de permisos del archivo de clave. Durante este trabajo
pasaron las pruebas Go completas, vet y validación de documentación y esquema.

CI volvió a ejecutar las pruebas con detector de carreras, validó documentación y
esquema, compiló Linux AMD64/ARM64 y no detectó vulnerabilidades alcanzables. En la
VM representativa de 998.465.536 bytes de RAM, el 8 de septiembre de 2026:

- el servidor permaneció 120 segundos con 11.476 KiB de RSS promedio y pico, sin
  swap; `/healthz` respondió 200 y `/readyz` respondió el 503 esperado sin una
  configuración aplicada;
- las pruebas dirigidas de commit, rollback, doce interrupciones, instalación
  vacía y fallo seguro terminaron correctamente, con 16.496 KiB de RSS máximo y
  cero swap;
- el binario de prueba correspondió al commit `6873aa9` y se verificó tras la
  transferencia antes de ejecutarlo.

Límites: terminar procesos no simula pérdida eléctrica. La exposición mediante
CLI pertenece al punto de entrega de CLI. Esta evidencia acepta el punto de clave
maestra y almacén de secretos, no el Hito 2 completo.
