#!/usr/bin/env bash
set -euo pipefail

(( EUID == 0 )) || { echo "Run this uninstaller with sudo." >&2; exit 1; }

cat <<'EOF'
ModelCairn conservative uninstaller

This removes only the service unit and executable. It preserves:
  /var/lib/modelcairn
  /etc/modelcairn
EOF

answer=""
if [[ "${1:-}" == "--yes" ]]; then
  (($# == 1)) || { echo "Usage: sudo ./scripts/uninstall-linux.sh [--yes]" >&2; exit 2; }
  answer=yes
elif (($# == 0)); then
  read -r -p "Continue? [y/N] " reply </dev/tty
  [[ "$reply" =~ ^[sSyY]$ ]] && answer=yes
else
  echo "Usage: sudo ./scripts/uninstall-linux.sh [--yes]" >&2
  exit 2
fi

if [[ "$answer" != yes ]]; then
  echo "Uninstall cancelled."
  exit 0
fi

systemctl disable --now modelcairn.service >/dev/null 2>&1 || true
rm -f -- /etc/systemd/system/modelcairn.service
rm -f -- /usr/local/bin/modelcairn
systemctl daemon-reload

cat <<'EOF'
ModelCairn executable and service were removed.
Configuration and data remain in /etc/modelcairn and /var/lib/modelcairn.
Create and verify a backup before removing either preserved directory manually.
EOF
