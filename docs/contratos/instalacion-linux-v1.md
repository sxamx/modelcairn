# Native Linux installation contract v1

[Español](instalacion-linux-v1.es.md)

The supported path requires systemd Linux, AMD64 or ARM64, and administrative
privileges during installation. The resulting service does not run as root.

| Element | Path/identity | Mode |
|---|---|---:|
| Binary | `/usr/local/bin/modelcairn`, root:root | `0755` |
| Service settings | `/etc/modelcairn`, root:modelcairn | `0750` |
| State | `/var/lib/modelcairn`, modelcairn:modelcairn | `0700` |
| Unit | `/etc/systemd/system/modelcairn.service`, root:root | `0644` |
| Process | `modelcairn` system user/group | no shell/home |

The installer requires a local executable and never downloads code through a
shell pipeline. Re-running atomically replaces the binary, unit, and listen address
while preserving the complete data directory. It does not open a firewall, install
Tailscale, or remove data.

`--enable` controls boot startup; `--start` controls current execution. Interactive
mode asks separately. Non-interactive mode requires both choices explicitly.
`--no-start` stops an already-running instance, so updating the executable never
leaves the old version silently running.

The default address is `127.0.0.1:8080`. Using `0.0.0.0` or `[::]` exposes the
service to reachable networks and requires operator-managed firewall and HTTPS.
For Tailscale, keep loopback and publish externally through Tailscale Serve.

Removing the unit or binary never authorizes deleting `/var/lib/modelcairn`.
Deleting data requires a separate, explicit, documented action after a verified
backup.
