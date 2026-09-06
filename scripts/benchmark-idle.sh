#!/usr/bin/env bash
set -euo pipefail

duration_seconds="${DURATION_SECONDS:-900}"
sample_interval="${SAMPLE_INTERVAL_SECONDS:-5}"
max_rss_kib="${MAX_RSS_KIB:-131072}"
listen_address="${LISTEN_ADDRESS:-127.0.0.1:18080}"
output_file="${OUTPUT_FILE:-benchmark-results/idle-$(date -u +%Y%m%dT%H%M%SZ).md}"

for value in "$duration_seconds" "$sample_interval" "$max_rss_kib"; do
  [[ "$value" =~ ^[1-9][0-9]*$ ]] || { echo "benchmark values must be positive integers" >&2; exit 2; }
done
command -v curl >/dev/null || { echo "curl is required" >&2; exit 2; }
[[ -x /usr/bin/time ]] || { echo "/usr/bin/time is required" >&2; exit 2; }

work_directory="$(mktemp -d)"
binary="$work_directory/modelcairn"
log_file="$work_directory/server.log"
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then
    kill -TERM "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf -- "$work_directory"
}
trap cleanup EXIT

recorded_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
kernel="$(uname -srmo)"
architecture="$(uname -m)"
logical_cpus="$(getconf _NPROCESSORS_ONLN)"
memory_total_kib="$(awk '/^MemTotal:/ {print $2}' /proc/meminfo)"
memory_available_before_kib="$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo)"
swap_total_kib="$(awk '/^SwapTotal:/ {print $2}' /proc/meminfo)"
swap_free_before_kib="$(awk '/^SwapFree:/ {print $2}' /proc/meminfo)"
resident_services="$(ps -eo comm=,rss= --sort=-rss | sed -n '1,15p')"
(( max_rss_kib <= memory_total_kib )) || { echo "peak budget exceeds total system memory" >&2; exit 2; }
reserved_margin_kib=$((memory_total_kib - max_rss_kib))

commit="$(git rev-parse --verify HEAD)"
build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
version="$(git describe --tags --always --dirty)"
build_package="github.com/sxamx/modelcairn/internal/buildinfo"
go build -trimpath -ldflags="-s -w -X ${build_package}.Version=${version} -X ${build_package}.Commit=${commit} -X ${build_package}.Date=${build_date}" -o "$binary" ./cmd/modelcairn
binary_version="$($binary version)"
base_binary_bytes="$(stat -c %s "$binary")"

sqlite_test_binary="$work_directory/storage-spike.test"
sqlite_time_file="$work_directory/storage-spike.time"
sqlite_test_output="$work_directory/storage-spike.output"
go test -c -o "$sqlite_test_binary" ./internal/storage
if ! (
  cd internal/storage
  /usr/bin/time -f '%M' -o "$sqlite_time_file" "$sqlite_test_binary" \
    -test.v -test.run '^TestSQLitePragmasAndSchema$' -test.count=1
) >"$sqlite_test_output" 2>&1; then
  cat "$sqlite_test_output" >&2
  echo "SQLite initialization probe failed" >&2
  exit 1
fi
grep -q -- '^--- PASS: TestSQLitePragmasAndSchema' "$sqlite_test_output" || { cat "$sqlite_test_output" >&2; echo "SQLite probe test did not run" >&2; exit 1; }
sqlite_binary_bytes="$(stat -c %s "$sqlite_test_binary")"
sqlite_peak_rss_kib="$(cat "$sqlite_time_file")"
"$binary" serve --listen "$listen_address" >"$log_file" 2>&1 &
server_pid=$!

for _ in {1..50}; do
  curl --fail --silent --max-time 1 "http://$listen_address/healthz" >/dev/null && break
  kill -0 "$server_pid" 2>/dev/null || { cat "$log_file" >&2; exit 1; }
  sleep 0.1
done
curl --fail --silent --max-time 2 "http://$listen_address/healthz" >/dev/null

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
(( samples > 0 )) || { echo "benchmark collected no samples" >&2; exit 1; }
average_rss_kib=$((sum_rss_kib / samples))
memory_available_after_kib="$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo)"
swap_free_after_kib="$(awk '/^SwapFree:/ {print $2}' /proc/meminfo)"

mkdir -p "$(dirname "$output_file")"
cat >"$output_file" <<EOF
# ModelCairn empty-server resource report

- Recorded at (UTC): $recorded_at
- Kernel: $kernel
- Architecture: $architecture
- Logical CPUs: $logical_cpus
- Total memory: ${memory_total_kib} KiB
- Available memory before ModelCairn: ${memory_available_before_kib} KiB
- Available memory after sampling: ${memory_available_after_kib} KiB
- System swap total: ${swap_total_kib} KiB
- System swap free before/after: ${swap_free_before_kib}/${swap_free_after_kib} KiB
- Go: $(go version)
- Binary: $binary_version
- Empty-server binary size: ${base_binary_bytes} bytes
- SQLite spike test binary size: ${sqlite_binary_bytes} bytes
- SQLite initialization test peak RSS: ${sqlite_peak_rss_kib} KiB
- Configuration: listen=$listen_address; empty in-memory operational state; no provider routes
- Duration: ${duration_seconds}s
- Sampling interval: ${sample_interval}s
- Samples: $samples
- Average RSS: ${average_rss_kib} KiB
- Peak RSS: ${peak_rss_kib} KiB
- Peak process swap: ${peak_swap_kib} KiB
- Enforced peak budget: ${max_rss_kib} KiB
- Memory reserved outside this process budget: ${reserved_margin_kib} KiB

## Largest resident processes before ModelCairn

```text
$resident_services
```
EOF

cat "$output_file"
if (( peak_rss_kib > max_rss_kib )); then
  echo "peak RSS exceeded budget" >&2
  exit 1
fi
