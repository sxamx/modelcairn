#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
working_directory="$(mktemp -d)"
data_directory="$working_directory/data"
session_file="$working_directory/session.json"
settings_file="$working_directory/settings.json"
plan_file="$working_directory/plan.json"
issue_file="$working_directory/issued.json"
export_file="$working_directory/export.json"
service_log="$working_directory/service.log"
binary="$working_directory/modelcairn"
port="${MODELCAIRN_HITO3_PORT:-18083}"
origin="http://127.0.0.1:$port"
service_pid=""

cleanup() {
  if [[ -n "$service_pid" ]] && kill -0 "$service_pid" 2>/dev/null; then
    kill "$service_pid" 2>/dev/null || true
    wait "$service_pid" 2>/dev/null || true
  fi
  rm -rf -- "$working_directory"
}
trap cleanup EXIT

cd "$repository_root"
go build -trimpath -o "$binary" ./cmd/modelcairn
printf '%s\n' "{\"apiVersion\":\"modelcairn.io/v1alpha1\",\"kind\":\"AdminSettings\",\"spec\":{\"publicOrigin\":\"$origin\",\"listen\":\"127.0.0.1:$port\",\"transport\":\"loopback-http\"}}" >"$settings_file"
printf '%s\n' 'a secure password' | "$binary" admin bootstrap --data-dir "$data_directory" --username owner --settings "$settings_file" >/dev/null

"$binary" serve --data-dir "$data_directory" >"$service_log" 2>&1 &
service_pid="$!"
for _ in $(seq 1 100); do
  if curl --fail --silent "$origin/readyz" >/dev/null; then break; fi
  if ! kill -0 "$service_pid" 2>/dev/null; then
    echo "service exited before readiness" >&2
    exit 1
  fi
  sleep 0.05
done
curl --fail --silent "$origin/readyz" >/dev/null

printf '%s\n' 'a secure password' | "$binary" admin login --server "$origin" --session-file "$session_file" --username owner >/dev/null
"$binary" admin whoami --server "$origin" --session-file "$session_file" >/dev/null
printf '%s\n' 'provider-secret-value' | "$binary" secret set --server "$origin" --session-file "$session_file" example-key-secret >/dev/null
"$binary" secret metadata --server "$origin" --session-file "$session_file" example-key-secret >/dev/null
"$binary" config plan --server "$origin" --session-file "$session_file" --out "$plan_file" docs/contratos/config/example-v1alpha1.yaml >/dev/null
"$binary" config apply --server "$origin" --session-file "$session_file" --plan "$plan_file" docs/contratos/config/example-v1alpha1.yaml >/dev/null
"$binary" config export --server "$origin" --session-file "$session_file" >"$export_file"
"$binary" agent-token status --server "$origin" --session-file "$session_file" example-agent >/dev/null
"$binary" agent-token issue --server "$origin" --session-file "$session_file" example-agent >"$issue_file"
"$binary" agent-token revoke --server "$origin" --session-file "$session_file" example-agent >/dev/null
"$binary" admin logout --server "$origin" --session-file "$session_file" >/dev/null

if [[ -e "$session_file" ]]; then echo "logout retained the session file" >&2; exit 1; fi
if grep -Fq 'provider-secret-value' "$service_log" "$export_file"; then echo "secret leaked to observable output" >&2; exit 1; fi
if grep -Fq 'a secure password' "$service_log"; then echo "password leaked to service log" >&2; exit 1; fi
if ! grep -Fq 'mc_at_v1_' "$issue_file"; then echo "one-time agent bearer missing" >&2; exit 1; fi

kill "$service_pid"
wait "$service_pid" || true
service_pid=""
echo "Milestone 3 integrated administrative flow passed"
