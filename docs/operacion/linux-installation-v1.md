# Native Linux installation and update v1

[Español](instalacion-linux-v1.es.md)

Requirements: AMD64 or ARM64 Linux with `systemd`, a local ModelCairn executable for
the correct architecture, and `sudo`. The installer downloads no software, opens no
ports, and does not install Tailscale.

## First installation

```sh
sudo ./scripts/install-linux.sh --binary ./modelcairn
sudo ./scripts/bootstrap-linux.sh
```

The installer separately asks whether to start after reboot (`enable`) and whether
to start now (`start`). Bootstrap requests the administrator name, public origin,
and password. Add API keys later through the console; the password never appears in
arguments, persistent temporary files, or logs.

```sh
systemctl is-enabled modelcairn.service
systemctl is-active modelcairn.service
curl --fail http://127.0.0.1:8080/readyz
```

## Non-interactive mode

Both choices are mandatory:

```sh
sudo ./scripts/install-linux.sh --binary ./modelcairn \
  --listen 127.0.0.1:8080 --enable --start --non-interactive
```

There is no non-interactive option for passing passwords.

## Manual update

1. Create and verify an MCB1 backup.
2. Obtain the new executable and verify its provenance outside the script.
3. Run `install-linux.sh` again with that executable.
4. Check the version, readiness, and one selected route.

`--start` restarts with the new executable. `--no-start` stops any active instance;
it never leaves the old version running accidentally. Data, generations, and
configuration are preserved.

## Conservative uninstall

```sh
sudo ./scripts/uninstall-linux.sh
```

It removes only the unit and `/usr/local/bin/modelcairn`, preserving
`/etc/modelcairn` and `/var/lib/modelcairn`. It has no `--purge` option: deleting
data requires a separate manual action after a verified backup.

For private networking or Tailscale, see the [access guide](acceso-red-v1.md). For
backup and restore, see the [MCB1 guide](backup-recovery-v1.md). The operator manages
firewall, DNS, certificates, the system, and `journald` retention.

