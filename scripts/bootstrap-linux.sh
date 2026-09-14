#!/usr/bin/env bash
set -euo pipefail

(( EUID == 0 )) || { echo "Run this bootstrap with sudo." >&2; exit 1; }
[[ -x /usr/local/bin/modelcairn ]] || { echo "ModelCairn is not installed." >&2; exit 1; }
id -u modelcairn >/dev/null 2>&1 || { echo "The modelcairn service identity is missing." >&2; exit 1; }
[[ -r /etc/modelcairn/service.env ]] || { echo "Service settings are missing." >&2; exit 1; }

listen="$(sed -n 's/^MODELCAIRN_LISTEN=//p' /etc/modelcairn/service.env)"
[[ -n "$listen" ]] || { echo "The service listen address is missing." >&2; exit 1; }
[[ "$listen" =~ ^(127\.0\.0\.1|\[::1\]): ]] || { echo "Guided bootstrap supports loopback/Tailscale Serve. Configure TLS settings explicitly for a network listener." >&2; exit 2; }
default_origin="http://127.0.0.1:${listen##*:}"

echo "ModelCairn · configuración inicial"
echo "La API key del proveedor se añadirá después desde la consola web."
read -r -p "Usuario administrador: " username </dev/tty
[[ "$username" =~ ^[A-Za-z0-9_.@-]{1,120}$ ]] || { echo "Invalid administrator username." >&2; exit 2; }
read -r -p "Origen público de la consola [$default_origin]: " public_origin </dev/tty
public_origin="${public_origin:-$default_origin}"
[[ "$public_origin" =~ ^https://[A-Za-z0-9._:\[\]-]+$ || "$public_origin" =~ ^http://(127\.0\.0\.1|\[::1\]):[1-9][0-9]{0,4}$ ]] || {
  echo "Use HTTPS, or HTTP only with a loopback origin." >&2; exit 2;
}

settings="$(mktemp /etc/modelcairn/.bootstrap.XXXXXX)"
cleanup(){ rm -f -- "$settings"; }
trap cleanup EXIT
chown root:modelcairn "$settings"; chmod 0640 "$settings"
cat >"$settings" <<EOF
apiVersion: modelcairn.io/v1alpha1
kind: AdminSettings
spec:
  publicOrigin: "$public_origin"
  listen: "$listen"
  transport: loopback-http
EOF

was_active=no
if systemctl is-active --quiet modelcairn.service; then was_active=yes; systemctl stop modelcairn.service; fi
restore_service(){ [[ "$was_active" == yes ]] && systemctl start modelcairn.service || true; }
trap 'restore_service; cleanup' EXIT
if ! runuser -u modelcairn -- /usr/local/bin/modelcairn admin bootstrap --data-dir /var/lib/modelcairn --username "$username" --settings "$settings" </dev/tty; then
  echo "Bootstrap failed; existing state was not replaced." >&2
  exit 1
fi
restore_service
was_active=no
echo "Configuración inicial completada. Ya puedes iniciar el servicio y abrir $public_origin"
