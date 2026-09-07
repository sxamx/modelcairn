# Hito 2 — Rotación de clave maestra

[English](hito-02-rotation.md)

Estado: implementada y probada localmente; falta la aceptación completa del issue #10.

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

Límites: terminar procesos no simula pérdida eléctrica. La ejecución Linux y
detección de carreras se asignan a CI; ejecutar en la VM representativa requiere
reautenticar Tailscale. Este avance no afirma mediciones de memoria en la VM ni
aceptación completa del issue. La exposición mediante CLI pertenece al punto de CLI.
