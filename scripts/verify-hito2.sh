#!/usr/bin/env bash
set -euo pipefail

duration_seconds="${DURATION_SECONDS:-60}"
sample_interval="${SAMPLE_INTERVAL_SECONDS:-1}"
max_rss_kib="${MAX_RSS_KIB:-131072}"
max_data_bytes="${MAX_DATA_BYTES:-16777216}"
listen_address="${LISTEN_ADDRESS:-127.0.0.1:18082}"
output_file="${OUTPUT_FILE:-benchmark-results/hito-02-$(date -u +%Y%m%dT%H%M%SZ).md}"

for value in "$duration_seconds" "$sample_interval" "$max_rss_kib" "$max_data_bytes"; do
  [[ "$value" =~ ^[1-9][0-9]*$ ]] || { echo "gate values must be positive integers" >&2; exit 2; }
done
command -v curl >/dev/null || { echo "curl is required" >&2; exit 2; }
[[ -x /usr/bin/time ]] || { echo "/usr/bin/time is required" >&2; exit 2; }

work_directory="$(mktemp -d)"
chmod 700 "$work_directory"
binary="$work_directory/modelcairn"
data_directory="$work_directory/data"
plan_file="$work_directory/plan.json"
export_file="$work_directory/export.json"
noop_plan_file="$work_directory/noop-plan.json"
command_log="$work_directory/commands.log"
server_log="$work_directory/server.log"
contention_log="$work_directory/contention.log"
server_pid=""
canary=""
cleanup() {
  status=$?
  trap - EXIT
  set +e
  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then
    kill -TERM "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  canary_leaked=0
  if [[ -n "$canary" ]] && grep -R -a -F -q -- "$canary" "$work_directory"; then
    canary_leaked=1
  fi
  rm -rf -- "$work_directory"
  if (( canary_leaked != 0 )); then
    echo "secret canary escaped a captured output or persisted artifact" >&2
    exit 1
  fi
  exit "$status"
}
trap cleanup EXIT

if [[ -n "${MODELCAIRN_BINARY:-}" ]]; then
  [[ -x "$MODELCAIRN_BINARY" ]] || { echo "MODELCAIRN_BINARY must be executable" >&2; exit 2; }
  [[ -n "${MODELCAIRN_EXPECTED_COMMIT:-}" ]] || { echo "MODELCAIRN_EXPECTED_COMMIT is required with a prebuilt binary" >&2; exit 2; }
  cp "$MODELCAIRN_BINARY" "$binary"
else
  command -v go >/dev/null || { echo "go is required unless MODELCAIRN_BINARY is set" >&2; exit 2; }
  commit="$(git rev-parse --verify HEAD)"
  version="$(git describe --tags --always --dirty)"
  build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  build_package="github.com/sxamx/modelcairn/internal/buildinfo"
  go build -trimpath -ldflags="-s -w -X ${build_package}.Version=${version} -X ${build_package}.Commit=${commit} -X ${build_package}.Date=${build_date}" -o "$binary" ./cmd/modelcairn
fi
binary_bytes="$(stat -c %s "$binary")"
binary_sha256="$(sha256sum "$binary" | awk '{print $1}')"
binary_version="$($binary version)"
if [[ -n "${MODELCAIRN_BINARY:-}" ]] && [[ "$binary_version" != *"commit=$MODELCAIRN_EXPECTED_COMMIT"* ]]; then
  echo "prebuilt binary does not match MODELCAIRN_EXPECTED_COMMIT" >&2
  exit 2
fi
if command -v go >/dev/null; then
  go_runtime="$(go version)"
else
  go_runtime="not installed; prebuilt binary supplied"
fi

canary="mc-$(date +%s%N)-$RANDOM-$RANDOM"
printf '%s\n' "$canary" | "$binary" secret set --data-dir "$data_directory" example-key-secret >>"$command_log" 2>&1
"$binary" config validate docs/contratos/config/example-v1alpha1.yaml >>"$command_log" 2>&1
"$binary" config plan --data-dir "$data_directory" --out "$plan_file" docs/contratos/config/example-v1alpha1.yaml >>"$command_log" 2>&1
"$binary" config apply --data-dir "$data_directory" --plan "$plan_file" docs/contratos/config/example-v1alpha1.yaml >>"$command_log" 2>&1
"$binary" config export --data-dir "$data_directory" >"$export_file" 2>>"$command_log"
"$binary" config validate "$export_file" >>"$command_log" 2>&1
"$binary" config plan --data-dir "$data_directory" --out "$noop_plan_file" "$export_file" >>"$command_log" 2>&1
"$binary" config apply --data-dir "$data_directory" --plan "$noop_plan_file" "$export_file" >>"$command_log" 2>&1
resource_count=$(( $(grep -c '"kind":' "$export_file") - 1 ))
(( resource_count > 0 )) || { echo "export contained no configured resources" >&2; exit 1; }

database_file="$data_directory/modelcairn.db"
database_bytes="$(stat -c %s "$database_file")"
data_bytes_before="$(du -sb "$data_directory" | awk '{print $1}')"
(( data_bytes_before <= max_data_bytes )) || { echo "configured data directory exceeds budget" >&2; exit 1; }

start_ns="$(date +%s%N)"
"$binary" serve --data-dir "$data_directory" --listen "$listen_address" >"$server_log" 2>&1 &
server_pid=$!
for _ in {1..100}; do
  if curl --fail --silent --max-time 1 "http://$listen_address/healthz" >/dev/null; then
    break
  fi
  kill -0 "$server_pid" 2>/dev/null || { echo "configured service exited before health response" >&2; exit 1; }
  sleep 0.05
done
curl --fail --silent --max-time 2 "http://$listen_address/healthz" >/dev/null
startup_milliseconds=$(( ($(date +%s%N) - start_ns) / 1000000 ))

# A stateful offline command must lose cleanly while the service owns the installation.
if "$binary" config export --data-dir "$data_directory" >"$contention_log" 2>&1; then
  echo "offline CLI unexpectedly acquired the running installation" >&2
  exit 1
fi
grep -Fxq 'installation_in_use' "$contention_log" || { echo "offline CLI failed for an unexpected reason" >&2; exit 1; }

samples=0
sum_rss_kib=0
peak_rss_kib=0
peak_swap_kib=0
deadline=$((SECONDS + duration_seconds))
while (( SECONDS < deadline )); do
  rss_kib="$(awk '/^VmRSS:/ {print $2}' "/proc/$server_pid/status")"
  process_swap_kib="$(awk '/^VmSwap:/ {print $2}' "/proc/$server_pid/status")"
  [[ "$rss_kib" =~ ^[0-9]+$ ]] || { echo "could not read VmRSS" >&2; exit 1; }
  [[ "$process_swap_kib" =~ ^[0-9]+$ ]] || process_swap_kib=0
  (( samples += 1 ))
  (( sum_rss_kib += rss_kib ))
  (( rss_kib > peak_rss_kib )) && peak_rss_kib=$rss_kib
  (( process_swap_kib > peak_swap_kib )) && peak_swap_kib=$process_swap_kib
  remaining=$((deadline - SECONDS))
  (( remaining > 0 )) || break
  sleep_seconds=$sample_interval
  (( sleep_seconds > remaining )) && sleep_seconds=$remaining
  sleep "$sleep_seconds"
done
average_rss_kib=$((sum_rss_kib / samples))
kill -TERM "$server_pid"
wait "$server_pid"
server_pid=""

"$binary" config export --data-dir "$data_directory" >"$work_directory/export-after.json" 2>>"$command_log"
cmp -s "$export_file" "$work_directory/export-after.json" || { echo "configuration changed during service contention" >&2; exit 1; }
data_bytes_after="$(du -sb "$data_directory" | awk '{print $1}')"
(( data_bytes_after <= max_data_bytes )) || { echo "final data directory exceeds budget" >&2; exit 1; }
if grep -R -a -F -q -- "$canary" "$work_directory"; then
  echo "secret canary escaped an output or persisted artifact" >&2
  exit 1
fi

mkdir -p "$(dirname "$output_file")"
cat >"$output_file" <<EOF
# ModelCairn Milestone 2 product integration report

- Recorded at (UTC): $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Kernel: $(uname -srmo)
- Architecture: $(uname -m)
- Logical CPUs: $(getconf _NPROCESSORS_ONLN)
- Total memory: $(awk '/^MemTotal:/ {print $2}' /proc/meminfo) KiB
- Go: $go_runtime
- Binary: $binary_version
- Binary SHA-256: $binary_sha256
- Stripped binary size: ${binary_bytes} bytes
- Configured SQLite database size before service: ${database_bytes} bytes
- Data-directory size before/after service: ${data_bytes_before}/${data_bytes_after} bytes
- Startup to health response: ${startup_milliseconds} ms
- Configured resources: $resource_count
- Duration: ${duration_seconds}s
- Sampling interval: ${sample_interval}s
- Samples: ${samples}
- Average RSS: ${average_rss_kib} KiB
- Peak RSS: ${peak_rss_kib} KiB
- Peak process swap: ${peak_swap_kib} KiB
- Enforced RSS budget: ${max_rss_kib} KiB
- Enforced data-directory budget: ${max_data_bytes} bytes
- Configuration round trip: pass
- Service/CLI exclusive ownership: pass
- Secret canary scan: pass
EOF

cat "$output_file"
(( peak_rss_kib <= max_rss_kib )) || { echo "peak RSS exceeded budget" >&2; exit 1; }
