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
source_binary="${MODELCAIRN_BINARY:-}"
expected_commit="${MODELCAIRN_EXPECTED_COMMIT:-}"
measure_seconds="${MEASURE_SECONDS:-0}"
output_file="${OUTPUT_FILE:-}"
port="${MODELCAIRN_HITO3_PORT:-18083}"
origin="http://127.0.0.1:$port"
service_pid=""
sampler_pid=""

cleanup() {
  if [[ -n "$service_pid" ]] && kill -0 "$service_pid" 2>/dev/null; then
    kill "$service_pid" 2>/dev/null || true
    wait "$service_pid" 2>/dev/null || true
  fi
  if [[ -n "$sampler_pid" ]] && kill -0 "$sampler_pid" 2>/dev/null; then kill "$sampler_pid" 2>/dev/null || true; fi
  rm -rf -- "$working_directory"
}
trap cleanup EXIT

cd "$repository_root"
[[ "$measure_seconds" =~ ^[0-9]+$ ]] || { echo "MEASURE_SECONDS must be a non-negative integer" >&2; exit 2; }
if [[ -n "$source_binary" ]]; then
  [[ -x "$source_binary" ]] || { echo "MODELCAIRN_BINARY must be executable" >&2; exit 2; }
  [[ -n "$expected_commit" ]] || { echo "MODELCAIRN_EXPECTED_COMMIT is required with a precompiled binary" >&2; exit 2; }
  cp "$source_binary" "$binary"
  "$binary" version | grep -Fq "$expected_commit" || { echo "precompiled binary commit mismatch" >&2; exit 2; }
else
  go build -trimpath -o "$binary" ./cmd/modelcairn
fi
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

samples_file="$working_directory/rss-samples"
(
  while kill -0 "$service_pid" 2>/dev/null; do
    rss="$(awk '/^VmRSS:/ {print $2}' "/proc/$service_pid/status" 2>/dev/null || true)"
    swap="$(awk '/^VmSwap:/ {print $2}' "/proc/$service_pid/status" 2>/dev/null || true)"
    [[ "$rss" =~ ^[0-9]+$ ]] && printf '%s %s\n' "$rss" "${swap:-0}" >>"$samples_file"
    sleep 0.05
  done
) &
sampler_pid="$!"

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
if (( measure_seconds > 0 )); then sleep "$measure_seconds"; fi
"$binary" admin logout --server "$origin" --session-file "$session_file" >/dev/null

if [[ -e "$session_file" ]]; then echo "logout retained the session file" >&2; exit 1; fi
if grep -Fq 'provider-secret-value' "$service_log" "$export_file"; then echo "secret leaked to observable output" >&2; exit 1; fi
if grep -Fq 'a secure password' "$service_log"; then echo "password leaked to service log" >&2; exit 1; fi
if ! grep -Fq 'mc_at_v1_' "$issue_file"; then echo "one-time agent bearer missing" >&2; exit 1; fi

kill "$sampler_pid" 2>/dev/null || true
wait "$sampler_pid" 2>/dev/null || true
sampler_pid=""
sample_count="$(wc -l <"$samples_file")"
(( sample_count > 0 )) || { echo "no service memory samples collected" >&2; exit 1; }
average_rss_kib="$(awk '{sum += $1} END {printf "%d", sum/NR}' "$samples_file")"
peak_rss_kib="$(awk 'BEGIN {max=0} $1>max {max=$1} END {print max}' "$samples_file")"
peak_swap_kib="$(awk 'BEGIN {max=0} $2>max {max=$2} END {print max}' "$samples_file")"

if [[ -n "$output_file" ]]; then
  mkdir -p "$(dirname "$output_file")"
  cat >"$output_file" <<EOF
# ModelCairn Milestone 3 administrative resource report

- Recorded at (UTC): $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Kernel: $(uname -srmo)
- Architecture: $(uname -m)
- Logical CPUs: $(getconf _NPROCESSORS_ONLN)
- Total memory: $(awk '/^MemTotal:/ {print $2}' /proc/meminfo) KiB
- Available memory after flow: $(awk '/^MemAvailable:/ {print $2}' /proc/meminfo) KiB
- Binary: $($binary version)
- Binary size: $(stat -c %s "$binary") bytes
- Flow: bootstrap, login, CSRF recovery, secret, config plan/apply/export, AgentToken issue/revoke, logout
- Additional steady-state duration: ${measure_seconds}s
- Samples: $sample_count at approximately 50 ms
- Average service RSS: ${average_rss_kib} KiB
- Peak service RSS: ${peak_rss_kib} KiB
- Peak service swap: ${peak_swap_kib} KiB
EOF
fi

kill "$service_pid"
wait "$service_pid" || true
service_pid=""
echo "Milestone 3 integrated administrative flow passed (peak RSS ${peak_rss_kib} KiB)"
