#!/usr/bin/env bash
set -euo pipefail

binary=""
listen="127.0.0.1:8080"
enable="ask"
start="ask"
interactive=1

usage() {
  cat <<'EOF'
Usage: sudo ./scripts/install-linux.sh --binary /path/to/modelcairn [options]

Options:
  --listen ADDRESS       Listen address (default: 127.0.0.1:8080)
  --enable | --no-enable  Enable or disable startup after reboot
  --start | --no-start    Start or leave the service stopped after install
  --non-interactive       Require explicit --enable/--no-enable and --start/--no-start
EOF
}

while (($#)); do
  case "$1" in
    --binary) (($# >= 2)) || { usage >&2; exit 2; }; binary="$2"; shift 2 ;;
    --listen) (($# >= 2)) || { usage >&2; exit 2; }; listen="$2"; shift 2 ;;
    --enable) enable=yes; shift ;;
    --no-enable) enable=no; shift ;;
    --start) start=yes; shift ;;
    --no-start) start=no; shift ;;
    --non-interactive) interactive=0; shift ;;
    --help|-h) usage; exit 0 ;;
    *) printf 'Unknown option: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
done

(( EUID == 0 )) || { echo "Run this installer with sudo." >&2; exit 1; }
[[ -n "$binary" && -f "$binary" && -x "$binary" ]] || { echo "--binary must point to an executable ModelCairn binary." >&2; exit 2; }
[[ "$listen" =~ ^(127\.0\.0\.1|0\.0\.0\.0|\[::1\]|\[::\]|[A-Za-z0-9._:-]+):([1-9][0-9]{0,4})$ ]] || { echo "Invalid --listen address." >&2; exit 2; }
port="${BASH_REMATCH[2]}"; (( port <= 65535 )) || { echo "Invalid --listen port." >&2; exit 2; }

ask_choice() {
  local prompt="$1" default="$2" reply
  read -r -p "$prompt" reply </dev/tty
  reply="${reply:-$default}"
  [[ "$reply" =~ ^[sSyY]$ ]] && printf yes || printf no
}
if (( interactive )); then
  echo "ModelCairn native installer"
  echo "Binary: /usr/local/bin/modelcairn"
  echo "Data:   /var/lib/modelcairn"
  [[ "$enable" == ask ]] && enable="$(ask_choice '¿Iniciar automáticamente después de reiniciar? [S/n] ' s)"
  [[ "$start" == ask ]] && start="$(ask_choice '¿Iniciar el servicio ahora? [S/n] ' s)"
else
  [[ "$enable" != ask && "$start" != ask ]] || { echo "Non-interactive mode requires an explicit enable and start choice." >&2; exit 2; }
fi

unit_source="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)/packaging/systemd/modelcairn.service"
[[ -f "$unit_source" ]] || { echo "Missing packaged systemd unit." >&2; exit 1; }

getent group modelcairn >/dev/null || groupadd --system modelcairn
id -u modelcairn >/dev/null 2>&1 || useradd --system --gid modelcairn --home-dir /nonexistent --no-create-home --shell /usr/sbin/nologin modelcairn
install -d -o root -g modelcairn -m 0750 /etc/modelcairn
install -d -o modelcairn -g modelcairn -m 0700 /var/lib/modelcairn
install -o root -g root -m 0755 "$binary" /usr/local/bin/.modelcairn.new
mv -f /usr/local/bin/.modelcairn.new /usr/local/bin/modelcairn
printf 'MODELCAIRN_LISTEN=%s\n' "$listen" > /etc/modelcairn/.service.env.new
chown root:root /etc/modelcairn/.service.env.new
chmod 0644 /etc/modelcairn/.service.env.new
mv -f /etc/modelcairn/.service.env.new /etc/modelcairn/service.env
install -o root -g root -m 0644 "$unit_source" /etc/systemd/system/modelcairn.service
systemctl daemon-reload
if [[ "$enable" == yes ]]; then systemctl enable modelcairn.service; else systemctl disable modelcairn.service >/dev/null 2>&1 || true; fi
if [[ "$start" == yes ]]; then systemctl restart modelcairn.service; fi

cat <<EOF

ModelCairn quedó instalado.
Escucha: $listen
Inicio automático: $enable
Servicio iniciado: $start

Si aún no creaste el administrador, detén el servicio y ejecuta:
  sudo systemctl stop modelcairn
  sudo -u modelcairn /usr/local/bin/modelcairn admin bootstrap --data-dir /var/lib/modelcairn --username TU_USUARIO --settings TU_ARCHIVO
Luego inicia con: sudo systemctl start modelcairn
EOF
