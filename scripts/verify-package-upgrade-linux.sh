#!/usr/bin/env bash
set -euo pipefail

old_binary="${1:-}"
new_binary="${2:-}"
old_version="${3:-}"
new_version="${4:-}"
port="${MODELCAIRN_UPGRADE_TEST_PORT:-18088}"

[[ "${MODELCAIRN_ALLOW_DESTRUCTIVE_SYSTEM_TEST:-}" == "yes" ]] || {
  echo "set MODELCAIRN_ALLOW_DESTRUCTIVE_SYSTEM_TEST=yes to authorize this isolated-host test" >&2
  exit 2
}
(( EUID == 0 )) || { echo "run as root on a disposable or explicitly prepared host" >&2; exit 2; }
[[ -x "$old_binary" && -x "$new_binary" ]] || { echo "two executable Linux binaries are required" >&2; exit 2; }
[[ "$old_version" =~ ^0\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || { echo "invalid old version" >&2; exit 2; }
[[ "$new_version" =~ ^0\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || { echo "invalid new version" >&2; exit 2; }
[[ "$port" =~ ^[1-9][0-9]{0,4}$ ]] && (( port <= 65535 )) || { echo "invalid test port" >&2; exit 2; }

repository_root="$(cd "$(dirname "$0")/.." && pwd)"
installer="$repository_root/scripts/install-linux.sh"
uninstaller="$repository_root/scripts/uninstall-linux.sh"
[[ -x "$installer" && -x "$uninstaller" ]] || { echo "installation scripts must be executable" >&2; exit 2; }

# Refuse any host with an active/installed ModelCairn service or binary. The test
# is destructive only to preserved, inactive state that it snapshots and restores.
[[ ! -e /usr/local/bin/modelcairn ]] || { echo "refusing host with an installed binary" >&2; exit 1; }
[[ ! -e /etc/systemd/system/modelcairn.service ]] || { echo "refusing host with an installed unit" >&2; exit 1; }
systemctl is-active --quiet modelcairn.service && { echo "refusing active service" >&2; exit 1; } || true
[[ -d /etc/modelcairn && -d /var/lib/modelcairn ]] || { echo "expected preserved test state is absent" >&2; exit 1; }

work="$(mktemp -d /tmp/modelcairn-package-upgrade.XXXXXX)"
snapshot="$work/pre-test.tar"
backup_dir=""
tar --acls --xattrs --numeric-owner -C / -cpf "$snapshot" etc/modelcairn var/lib/modelcairn
tar -tf "$snapshot" | awk '!/^(etc\/modelcairn|var\/lib\/modelcairn)(\/|$)/ { bad=1 } END { exit bad }'

restored=0
current_step="prepare isolated host"
restore_host() {
  if (( restored )); then return; fi
  systemctl disable --now modelcairn.service >/dev/null 2>&1 || true
  rm -f -- /etc/systemd/system/modelcairn.service /usr/local/bin/modelcairn
  systemctl daemon-reload >/dev/null 2>&1 || true
  rm -rf -- /etc/modelcairn /var/lib/modelcairn
  tar --acls --xattrs --numeric-owner -C / -xpf "$snapshot"
  tar --acls --xattrs --numeric-owner -C / -dpf "$snapshot"
  restored=1
}
cleanup() {
  status=$?
  if (( status != 0 )); then
    printf 'package upgrade gate failed during: %s\n' "$current_step" >&2
  fi
  restore_host
  [[ -z "$backup_dir" ]] || rm -rf -- "$backup_dir"
  rm -rf -- "$work"
}
trap cleanup EXIT

rm -rf -- /etc/modelcairn /var/lib/modelcairn

current_step="install old package"
"$installer" --binary "$old_binary" --listen "127.0.0.1:$port" --no-enable --no-start --non-interactive >/dev/null
settings_file="/etc/modelcairn/.release-test-settings.json"
printf '{"apiVersion":"modelcairn.io/v1alpha1","kind":"AdminSettings","spec":{"publicOrigin":"http://127.0.0.1:%s","listen":"127.0.0.1:%s","transport":"loopback-http"}}\n' "$port" "$port" >"$settings_file"
chown root:modelcairn "$settings_file"
chmod 0640 "$settings_file"
printf '%s\n' 'temporary release validation password' |
  runuser -u modelcairn -- /usr/local/bin/modelcairn admin bootstrap \
    --data-dir /var/lib/modelcairn --username release-test --settings "$settings_file" >/dev/null
rm -f -- "$settings_file"
admin_identity() {
  rm -f -- "$work/session.json" "$work/whoami.json"
  printf '%s\n' 'temporary release validation password' |
    /usr/local/bin/modelcairn admin login --server "http://127.0.0.1:$port" \
      --session-file "$work/session.json" --username release-test >/dev/null
  /usr/local/bin/modelcairn admin whoami --server "http://127.0.0.1:$port" \
    --session-file "$work/session.json" >"$work/whoami.json"
  python3 -c 'import json,sys; value=json.load(open(sys.argv[1])); identity=value["id"]; assert identity; print(identity)' "$work/whoami.json"
}
systemctl start modelcairn.service
current_step="verify old package and admin session"
for _ in {1..100}; do
  curl --fail --silent "http://127.0.0.1:$port/readyz" >/dev/null && break
  sleep 0.05
done
curl --fail --silent "http://127.0.0.1:$port/readyz" >/dev/null
/usr/local/bin/modelcairn version | grep -Fq "modelcairn $old_version "
admin_id="$(admin_identity)"

current_step="create and verify pre-upgrade backup"
backup_dir="$(mktemp -d /tmp/modelcairn-mcb-backup.XXXXXX)"
chown modelcairn:modelcairn "$backup_dir"
chmod 0700 "$backup_dir"
systemctl stop modelcairn.service
current_step="create pre-upgrade backup"
printf '%s\n' 'temporary backup passphrase' |
  runuser -u modelcairn -- /usr/local/bin/modelcairn backup create \
    --data-dir /var/lib/modelcairn --out "$backup_dir/pre-upgrade.mcb.age" >/dev/null
current_step="verify pre-upgrade backup"
printf '%s\n' 'temporary backup passphrase' |
  runuser -u modelcairn -- /usr/local/bin/modelcairn backup verify \
    "$backup_dir/pre-upgrade.mcb.age" >/dev/null
current_step="verify pre-upgrade backup permissions"
if ! backup_mode="$(stat -c %a "$backup_dir/pre-upgrade.mcb.age")"; then
  printf 'backup output is missing after successful verification\n' >&2
  ls -la -- "$backup_dir" >&2 || true
  exit 1
fi
[[ "$backup_mode" == "600" ]] || {
  printf 'unexpected backup mode: %s (expected 600)\n' "$backup_mode" >&2
  exit 1
}
current_step="restart old package after backup"
systemctl start modelcairn.service
for _ in {1..100}; do
  curl --fail --silent "http://127.0.0.1:$port/readyz" >/dev/null && break
  sleep 0.05
done
curl --fail --silent "http://127.0.0.1:$port/readyz" >/dev/null

current_step="update to new package"
"$installer" --binary "$new_binary" --listen "127.0.0.1:$port" --no-enable --start --non-interactive >/dev/null
/usr/local/bin/modelcairn version | grep -Fq "modelcairn $new_version "
curl --fail --silent "http://127.0.0.1:$port/readyz" >/dev/null
[[ "$(admin_identity)" == "$admin_id" ]]

current_step="roll back to old package"
"$installer" --binary "$old_binary" --listen "127.0.0.1:$port" --no-enable --start --non-interactive >/dev/null
/usr/local/bin/modelcairn version | grep -Fq "modelcairn $old_version "
curl --fail --silent "http://127.0.0.1:$port/readyz" >/dev/null
[[ "$(admin_identity)" == "$admin_id" ]]

current_step="uninstall and restore preserved host state"
"$uninstaller" --yes >/dev/null
restore_host
[[ ! -e /usr/local/bin/modelcairn && ! -e /etc/systemd/system/modelcairn.service ]]
printf 'package install, update, rollback, uninstall, and host restoration passed\n'
