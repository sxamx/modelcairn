# Acceso de red v1

[English](acceso-red-v1.md)

ModelCairn escucha por defecto en `127.0.0.1:8080`. Esta configuración evita
publicar la consola accidentalmente y sirve como base para los tres modos
soportados. El instalador no modifica el firewall, DNS, certificados ni Tailscale.

## Elegir un modo

| Modo | Exposición | HTTPS | Uso recomendado |
|---|---|---|---|
| Localhost | Solo la VM; acceso remoto mediante túnel SSH | No es necesario dentro del túnel | Administración o diagnóstico |
| Red privada | La red que alcance el proxy inverso | Obligatorio en el proxy | Redes privadas administradas por el operador |
| Tailscale Serve | Solo dispositivos autorizados del tailnet | Terminado por Tailscale | Opción recomendada para acceso remoto privado |

No se debe combinar más de un modo sin una decisión explícita. En particular,
Tailscale Funnel hace público el servicio en Internet y no forma parte de esta guía.

## Localhost y túnel SSH

Instala con la dirección predeterminada y conserva el origen público sugerido:

```sh
sudo ./scripts/install-linux.sh --binary ./modelcairn
sudo ./scripts/bootstrap-linux.sh
```

Para abrir la consola desde otro equipo, crea el túnel desde ese equipo:

```sh
ssh -L 8080:127.0.0.1:8080 usuario@servidor
```

Mientras esa sesión permanezca abierta, visita `http://127.0.0.1:8080`. El túnel
SSH cifra el trayecto; el puerto de ModelCairn continúa inaccesible desde la red.

## Red privada con HTTPS

Conserva ModelCairn en loopback y configura un proxy inverso administrado por el
operador para publicar un nombre HTTPS privado. El proxy debe:

1. escuchar únicamente en la red deseada;
2. reenviar a `http://127.0.0.1:8080`;
3. entregar un certificado válido para su nombre;
4. preservar `Host`, `X-Forwarded-Proto: https` y la dirección del cliente;
5. limitar el acceso mediante firewall o controles equivalentes.

Durante `bootstrap-linux.sh`, usa como origen público la URL HTTPS exacta del proxy,
por ejemplo `https://modelcairn.interna.example`. La opción avanzada `--listen`
permite cambiar la interfaz de escucha, pero no añade TLS: no debe usarse para
publicar HTTP directamente en una red no confiable.

## Tailscale Serve

Prerrequisitos: Tailscale ya instalado y autenticado en la VM, MagicDNS/HTTPS
habilitables en el tailnet y permisos de acceso definidos por su administrador.
ModelCairn no almacena credenciales de Tailscale ni claves SSH.

Con ModelCairn escuchando en loopback, configura el proxy privado persistente:

```sh
sudo tailscale serve --bg http://127.0.0.1:8080
sudo tailscale serve status
```

El primer comando muestra la URL HTTPS `*.ts.net`. Usa esa URL exacta como origen
público durante `bootstrap-linux.sh`. `--bg` hace que la configuración se reanude
después de reiniciar la VM o Tailscale; el servicio ModelCairn debe habilitarse por
separado con `systemctl enable modelcairn.service`.

Comprueba ambos componentes:

```sh
systemctl is-active modelcairn.service
sudo tailscale serve status
curl --fail http://127.0.0.1:8080/readyz
```

Para dejar de publicar ModelCairn en el tailnet:

```sh
sudo tailscale serve reset
```

`reset` elimina toda la configuración Serve del nodo, no solamente una ruta. Antes
de usarlo en un nodo compartido, revisa `tailscale serve status`.

## Diagnóstico mínimo

```sh
systemctl status modelcairn.service
journalctl -u modelcairn.service --since today
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8080/readyz
```

`healthz` prueba que el proceso responde; `readyz` prueba que está listo para
atender tráfico. Los logs pueden contener metadatos operativos, por lo que deben
conservarse según la política local y no publicarse como evidencia sin revisarlos.

## Referencias del componente opcional

- [Tailscale Serve](https://tailscale.com/docs/features/tailscale-serve)
- [Referencia de `tailscale serve`](https://tailscale.com/docs/reference/tailscale-cli/serve)

