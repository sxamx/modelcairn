# Instalación y actualización nativa Linux v1

[English](linux-installation-v1.md)

Requisitos: Linux AMD64 o ARM64 con `systemd`, un binario ModelCairn local para la
arquitectura correcta y privilegios `sudo`. El instalador no descarga software, no
abre puertos y no instala Tailscale.
Si se solicita iniciar el servicio, comprueba que permanezca activo. Un puerto
ocupado u otro fallo de arranque produce error, detiene el bucle de reinicios y
remite al journal; nunca confirma falsamente una instalación lista.

## Primera instalación

```sh
sudo ./scripts/install-linux.sh --binary ./modelcairn
sudo ./scripts/bootstrap-linux.sh
```

El instalador pregunta por separado si debe arrancar tras reiniciar (`enable`) y si
debe iniciarse ahora (`start`). Bootstrap solicita usuario, origen público y
contraseña administrativa. La API key se añade después desde la consola; la
contraseña nunca aparece en argumentos, temporales persistentes ni logs.

```sh
systemctl is-enabled modelcairn.service
systemctl is-active modelcairn.service
curl --fail http://127.0.0.1:8080/readyz
```

## Modo no interactivo

Ambas decisiones son obligatorias:

```sh
sudo ./scripts/install-linux.sh --binary ./modelcairn \
  --listen 127.0.0.1:8080 --enable --start --non-interactive
```

No existe una opción no interactiva para pasar contraseñas.

## Actualización manual

1. Crea y verifica un backup MCB1.
2. Obtén el binario nuevo y verifica su procedencia fuera del script.
3. Ejecuta nuevamente `install-linux.sh` con ese binario.
4. Comprueba versión, readiness y una ruta seleccionada.

`--start` reinicia con el binario nuevo. `--no-start` detiene cualquier instancia
activa; nunca deja la versión anterior ejecutándose accidentalmente. Datos,
generaciones y configuración se conservan.

## Desinstalación conservadora

```sh
sudo ./scripts/uninstall-linux.sh
```

Elimina solamente la unidad y `/usr/local/bin/modelcairn`; conserva
`/etc/modelcairn` y `/var/lib/modelcairn`. No ofrece `--purge`: borrar datos requiere
una acción manual posterior a un backup verificado.

Para red privada o Tailscale consulta la [guía de acceso](acceso-red-v1.es.md). Para
copias y restore consulta la [guía MCB1](backup-recuperacion-v1.es.md). El operador
administra firewall, DNS, certificados, sistema y retención de `journald`.
