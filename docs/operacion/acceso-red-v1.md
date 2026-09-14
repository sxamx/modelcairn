# Network access v1

[Español](acceso-red-v1.es.md)

ModelCairn listens on `127.0.0.1:8080` by default. This prevents accidental console
publication and is the basis for all three supported modes. The installer does not
change the firewall, DNS, certificates, or Tailscale.

## Choose one mode

| Mode | Exposure | HTTPS | Recommended use |
|---|---|---|---|
| Localhost | VM only; remote access through an SSH tunnel | Not required inside the tunnel | Administration or diagnostics |
| Private network | Network that can reach the reverse proxy | Required at the proxy | Operator-managed private networks |
| Tailscale Serve | Authorized tailnet devices only | Terminated by Tailscale | Recommended private remote access |

Do not combine modes without an explicit decision. In particular, Tailscale Funnel
makes a service public on the Internet and is outside this guide.

## Localhost and SSH tunnel

Install with the default listener and retain the suggested public origin:

```sh
sudo ./scripts/install-linux.sh --binary ./modelcairn
sudo ./scripts/bootstrap-linux.sh
```

To open the console remotely, create this tunnel from the client machine:

```sh
ssh -L 8080:127.0.0.1:8080 user@server
```

While the SSH session remains open, visit `http://127.0.0.1:8080`. SSH encrypts the
path while the ModelCairn port remains unreachable from the network.

## Private network with HTTPS

Keep ModelCairn on loopback and configure an operator-managed reverse proxy to
publish a private HTTPS name. The proxy must:

1. listen only on the intended network;
2. forward to `http://127.0.0.1:8080`;
3. present a valid certificate for its name;
4. preserve `Host`, `X-Forwarded-Proto: https`, and the client address;
5. restrict access through a firewall or equivalent controls.

During `bootstrap-linux.sh`, use the proxy's exact HTTPS URL as the public origin,
for example `https://modelcairn.internal.example`. The advanced `--listen` option
can change the listening interface but does not add TLS and must not expose plain
HTTP to an untrusted network.

## Tailscale Serve

Prerequisites: Tailscale is installed and authenticated on the VM, MagicDNS/HTTPS
can be enabled for the tailnet, and its administrator has defined access. ModelCairn
stores neither Tailscale credentials nor SSH keys.

With ModelCairn listening on loopback, configure the persistent private proxy:

```sh
sudo tailscale serve --bg http://127.0.0.1:8080
sudo tailscale serve status
```

The first command prints the `*.ts.net` HTTPS URL. Use that exact URL as the public
origin during `bootstrap-linux.sh`. `--bg` resumes the configuration after VM or
Tailscale restarts; enable ModelCairn separately with
`systemctl enable modelcairn.service`.

Check both components:

```sh
systemctl is-active modelcairn.service
sudo tailscale serve status
curl --fail http://127.0.0.1:8080/readyz
```

To stop publishing ModelCairn to the tailnet:

```sh
sudo tailscale serve reset
```

`reset` removes the node's entire Serve configuration, not just one route. Inspect
`tailscale serve status` first on a shared node.

## Minimum diagnostics

```sh
systemctl status modelcairn.service
journalctl -u modelcairn.service --since today
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8080/readyz
```

`healthz` confirms the process responds; `readyz` confirms it is ready for traffic.
Logs may contain operational metadata, so retain them under the local policy and do
not publish them as evidence without review.

## Optional component references

- [Tailscale Serve](https://tailscale.com/docs/features/tailscale-serve)
- [`tailscale serve` reference](https://tailscale.com/docs/reference/tailscale-cli/serve)

