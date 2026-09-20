#!/usr/bin/env bash
set -euo pipefail

binary=""
listen="127.0.0.1:8080"
listen_set=0
enable="ask"
start="ask"
interactive=1
binary_tmp=""
environment_tmp=""

cleanup_install() {
  [[ -z "$binary_tmp" || ! -e "$binary_tmp" ]] || rm -f -- "$binary_tmp"
  [[ -z "$environment_tmp" || ! -e "$environment_tmp" ]] || rm -f -- "$environment_tmp"
}
trap cleanup_install EXIT

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
    --listen) (($# >= 2)) || { usage >&2; exit 2; }; listen="$2"; listen_set=1; shift 2 ;;
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
if (( ! listen_set )) && [[ -r /etc/modelcairn/service.env ]]; then
  installed_listen="$(sed -n 's/^MODELCAIRN_LISTEN=//p' /etc/modelcairn/service.env)"
  [[ -z "$installed_listen" ]] || listen="$installed_listen"
fi
[[ "$listen" =~ ^(127\.0\.0\.1|0\.0\.0\.0|\[::1\]|\[::\]|[A-Za-z0-9._:-]+):([1-9][0-9]{0,4})$ ]] || { echo "Invalid --listen address." >&2; exit 2; }
port="${BASH_REMATCH[2]}"; (( port <= 65535 )) || { echo "Invalid --listen port." >&2; exit 2; }

ask_choice() {
  local prompt="$1" default="$2" reply
  read -r -p "$prompt" reply </dev/tty
  reply="${reply:-$default}"
  [[ "$reply" =~ ^[sSyY]$ ]] && printf yes || printf no
}

verify_service_started() {
  local attempt
  for attempt in {1..20}; do
    sleep 0.1
    if ! systemctl is-active --quiet modelcairn.service; then
      systemctl stop modelcairn.service >/dev/null 2>&1 || true
      echo "ModelCairn did not remain active. Check whether $listen is already in use, then inspect: journalctl -u modelcairn.service" >&2
      return 1
    fi
  done
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
if id -u modelcairn >/dev/null 2>&1; then
  [[ "$(id -gn modelcairn)" == modelcairn && "$(getent passwd modelcairn | cut -d: -f6-7)" == "/nonexistent:/usr/sbin/nologin" ]] || {
    echo "Existing modelcairn user does not match the required service identity." >&2
    exit 1
  }
else
  useradd --system --gid modelcairn --home-dir /nonexistent --no-create-home --shell /usr/sbin/nologin modelcairn
fi
install -d -o root -g modelcairn -m 0750 /etc/modelcairn
install -d -o modelcairn -g modelcairn -m 0700 /var/lib/modelcairn
binary_tmp="$(mktemp /usr/local/bin/.modelcairn.XXXXXX)"
install -o root -g root -m 0755 "$binary" "$binary_tmp"
mv -f "$binary_tmp" /usr/local/bin/modelcairn
binary_tmp=""
environment_tmp="$(mktemp /etc/modelcairn/.service.env.XXXXXX)"
printf 'MODELCAIRN_LISTEN=%s\n' "$listen" > "$environment_tmp"
chown root:modelcairn "$environment_tmp"
chmod 0640 "$environment_tmp"
mv -f "$environment_tmp" /etc/modelcairn/service.env
environment_tmp=""
install -o root -g root -m 0644 "$unit_source" /etc/systemd/system/modelcairn.service
systemctl daemon-reload
if [[ "$enable" == yes ]]; then
  systemctl enable modelcairn.service
else
  systemctl disable modelcairn.service
  systemctl is-enabled --quiet modelcairn.service && { echo "ModelCairn remained enabled." >&2; exit 1; } || true
fi
if [[ "$start" == yes ]]; then
  systemctl restart modelcairn.service
  verify_service_started
else
  systemctl stop modelcairn.service
  systemctl is-active --quiet modelcairn.service && { echo "ModelCairn remained active." >&2; exit 1; } || true
fi

cat <<EOF

ModelCairn quedó instalado.
Escucha: $listen
Inicio automático: $enable
Servicio iniciado: $start

Si aún no creaste el administrador, detén el servicio y ejecuta:
  sudo ./scripts/bootstrap-linux.sh
El asistente detendrá y restaurará el servicio cuando sea necesario.
EOF
