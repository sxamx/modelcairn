# Backup y recuperación MCB1 v1

[English](backup-recovery-v1.md)

Estas operaciones son offline: detén ModelCairn para que la CLI adquiera el
bloqueo exclusivo. Ningún comando acepta la contraseña como argumento.

## Crear y verificar

El directorio de destino debe existir y el archivo no debe existir:

```sh
sudo systemctl stop modelcairn.service
sudo -u modelcairn /usr/local/bin/modelcairn backup create \
  --data-dir /var/lib/modelcairn \
  --out /ruta/privada/modelcairn-2026-09-14.mcb.age
sudo -u modelcairn /usr/local/bin/modelcairn backup verify \
  /ruta/privada/modelcairn-2026-09-14.mcb.age
sudo systemctl start modelcairn.service
```

La terminal solicita la contraseña y la confirma durante creación. Para automatizar,
la entrada estándar puede conectarse a un descriptor secreto administrado por el
operador; no uses argumentos, variables de entorno ni texto visible en el historial.

`create` toma una instantánea SQLite consistente, cifra el tar sin compresión,
sincroniza el archivo y lo verifica antes de renombrarlo al destino. No sobrescribe.

## Restaurar

Conserva primero un backup verificable del estado actual. Luego:

```sh
sudo -u modelcairn /usr/local/bin/modelcairn backup restore \
  --data-dir /var/lib/modelcairn \
  /ruta/privada/modelcairn-2026-09-14.mcb.age
sudo systemctl start modelcairn.service
curl --fail http://127.0.0.1:8080/readyz
```

Restore autentica todo antes de crear una generación. En una segunda lectura vuelve
a validar límites y checksums, migra la copia, genera identidad y clave maestra
nuevas, recifra cada secreto y elimina las sesiones administrativas. Solo entonces
cambia `current` atómicamente.

Después de readiness, inicia sesión otra vez y prueba una ruta elegida. Los tokens
de agente se conservan; las sesiones web anteriores dejan de funcionar.

## Volver a la generación anterior

Si readiness o la prueba funcional fallan:

```sh
sudo systemctl stop modelcairn.service
sudo -u modelcairn /usr/local/bin/modelcairn backup rollback \
  --data-dir /var/lib/modelcairn
sudo systemctl start modelcairn.service
curl --fail http://127.0.0.1:8080/readyz
```

Rollback selecciona el predecesor registrado y no borra ninguna generación.
ModelCairn v1 tampoco elimina generaciones automáticamente.

## Fallos seguros

- Contraseña incorrecta, archivo truncado, formato desconocido o checksum inválido
  terminan durante preflight y no crean una generación.
- Base incompatible, secreto discordante o falta de espacio eliminan únicamente la
  generación aún inactiva.
- Un destino de backup existente nunca se reemplaza.
- `installation_in_use` indica que otro proceso posee el directorio: detén el
  servicio y reintenta sin borrar el archivo de bloqueo.

