#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
data="$work/data"
binary="$work/modelcairn"
origin="http://127.0.0.1:${MODELCAIRN_HITO4_PORT:-18084}"
upstream_origin="http://127.0.0.1:${MODELCAIRN_HITO4_UPSTREAM_PORT:-18085}"
max_rss_kib="${MAX_RSS_KIB:-131072}"
output_file="${OUTPUT_FILE:-}"
python_command="${PYTHON_COMMAND:-python3}"
batches_per_level="${BATCHES_PER_LEVEL:-1}"
sustained_seconds="${SUSTAINED_SECONDS:-0}"
sustained_concurrency="${SUSTAINED_CONCURRENCY:-10}"
service_pid=""
upstream_pid=""
sampler_pid=""

cleanup() {
  for pid in "$sampler_pid" "$service_pid" "$upstream_pid"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  rm -rf -- "$work"
}
trap cleanup EXIT

command -v curl >/dev/null || { echo "curl is required" >&2; exit 2; }
if ! "$python_command" --version >/dev/null 2>&1; then
  if python --version >/dev/null 2>&1; then
    python_command="python"
  else
    echo "Python 3 is required" >&2
    exit 2
  fi
fi
[[ "$max_rss_kib" =~ ^[1-9][0-9]*$ ]] ||
  { echo "MAX_RSS_KIB must be a positive integer" >&2; exit 2; }
[[ "$batches_per_level" =~ ^[1-9][0-9]*$ ]] ||
  { echo "BATCHES_PER_LEVEL must be a positive integer" >&2; exit 2; }
[[ "$sustained_seconds" =~ ^[0-9]+$ ]] ||
  { echo "SUSTAINED_SECONDS must be a non-negative integer" >&2; exit 2; }
[[ "$sustained_concurrency" =~ ^[1-9][0-9]*$ ]] ||
  { echo "SUSTAINED_CONCURRENCY must be a positive integer" >&2; exit 2; }
cd "$repository_root"

if [[ -n "${MODELCAIRN_BINARY:-}" ]]; then
  [[ -x "$MODELCAIRN_BINARY" ]] ||
    { echo "MODELCAIRN_BINARY must be executable" >&2; exit 2; }
  [[ -n "${MODELCAIRN_EXPECTED_COMMIT:-}" ]] ||
    { echo "MODELCAIRN_EXPECTED_COMMIT is required with a precompiled binary" >&2; exit 2; }
  cp "$MODELCAIRN_BINARY" "$binary"
  "$binary" version | grep -Fq "$MODELCAIRN_EXPECTED_COMMIT" ||
    { echo "precompiled binary commit mismatch" >&2; exit 2; }
else
  go build -trimpath -o "$binary" ./cmd/modelcairn
fi

"$python_command" scripts/hito4-upstream.py "${MODELCAIRN_HITO4_UPSTREAM_PORT:-18085}" >"$work/upstream.log" 2>&1 &
upstream_pid="$!"
printf '%s\n' "{\"apiVersion\":\"modelcairn.io/v1alpha1\",\"kind\":\"AdminSettings\",\"spec\":{\"publicOrigin\":\"$origin\",\"listen\":\"${origin#http://}\",\"transport\":\"loopback-http\"}}" >"$work/settings.json"
printf '%s\n' 'a secure password' |
  "$binary" admin bootstrap --data-dir "$data" --username owner --settings "$work/settings.json" >/dev/null
"$binary" serve --data-dir "$data" >"$work/service.log" 2>&1 &
service_pid="$!"
for _ in $(seq 1 100); do
  curl --fail --silent "$origin/readyz" >/dev/null && break
  kill -0 "$service_pid" 2>/dev/null ||
    { echo "service exited before readiness" >&2; exit 1; }
  sleep 0.05
done
curl --fail --silent "$origin/readyz" >/dev/null

(
  while kill -0 "$service_pid" 2>/dev/null; do
    rss="$(awk '/^VmRSS:/ {print $2}' "/proc/$service_pid/status" 2>/dev/null || true)"
    swap="$(awk '/^VmSwap:/ {print $2}' "/proc/$service_pid/status" 2>/dev/null || true)"
    [[ "$rss" =~ ^[0-9]+$ ]] &&
      printf '%s %s\n' "$rss" "${swap:-0}" >>"$work/rss-samples"
    sleep 0.05
  done
) &
sampler_pid="$!"
cpu_ticks_start="$(awk '{print $14+$15}' "/proc/$service_pid/stat")"
wall_start_ns="$(date +%s%N)"

printf '%s\n' 'a secure password' |
"$binary" admin login --server "$origin" --session-file "$work/session.json" --username owner >/dev/null
session_token="$("$python_command" -c 'import json,sys; print(json.load(open(sys.argv[1]))["sessionToken"])' "$work/session.json")"
csrf_token="$("$python_command" -c 'import json,sys; print(json.load(open(sys.argv[1]))["csrfToken"])' "$work/session.json")"
printf 'header = "Cookie: mc_session=%s"\n' "$session_token" >"$work/curl-admin"
printf 'header = "Origin: %s"\n' "$origin" >>"$work/curl-admin"
printf 'header = "X-CSRF-Token: %s"\n' "$csrf_token" >>"$work/curl-admin"
chmod 600 "$work/curl-admin"
unset session_token csrf_token
printf '%s\n' 'benchmark-provider-secret' |
  "$binary" secret set --server "$origin" --session-file "$work/session.json" onboarding-secret >/dev/null
cp docs/contratos/config/onboarding-first-route-v1.json "$work/config.json"
"$python_command" - "$work/config.json" "$upstream_origin/v1" <<'PY'
import json
import sys

path, origin = sys.argv[1:]
with open(path, encoding="utf-8") as source:
    document = json.load(source)
for resource in document["resources"]:
    if resource["kind"] == "ProviderConnection":
        resource["spec"]["baseUrl"] = origin
        resource["spec"]["allowPrivateNetwork"] = True
    if resource["kind"] == "Strategy":
        resource["spec"]["attemptTimeoutMs"] = 5000
        resource["spec"]["totalTimeoutMs"] = 10000
with open(path, "w", encoding="utf-8") as target:
    json.dump(document, target, separators=(",", ":"))
PY
"$binary" config plan --server "$origin" --session-file "$work/session.json" --out "$work/plan.json" "$work/config.json" >/dev/null
"$binary" config apply --server "$origin" --session-file "$work/session.json" --plan "$work/plan.json" "$work/config.json" >/dev/null
"$binary" agent-token issue --server "$origin" --session-file "$work/session.json" onboarding-agent >"$work/issued.json"
agent_token="$("$python_command" -c 'import json,sys; value=json.load(open(sys.argv[1]))["token"]; assert value.startswith("mc_at_v1_"); print(value)' "$work/issued.json")"
printf 'header = "Authorization: Bearer %s"\n' "$agent_token" >"$work/curl-auth"
chmod 600 "$work/curl-auth"
unset agent_token

printf '%s\n' '{"model":"assistant","messages":[{"role":"user","content":"nonstream-canary"}]}' >"$work/nonstream.json"
curl --fail --silent --show-error --config "$work/curl-auth" -H 'Content-Type: application/json' --data-binary "@$work/nonstream.json" "$origin/v1/chat/completions" >"$work/nonstream-response.json"
"$python_command" -c 'import json,sys; value=json.load(open(sys.argv[1])); assert value["model"]=="assistant"; assert value["choices"][0]["message"]["content"]=="benchmark-ok"' "$work/nonstream-response.json"

printf '%s\n' '{"model":"assistant","messages":[{"role":"user","content":"stream-canary"}],"stream":true}' >"$work/stream.json"
: >"$work/results"
for concurrency in 1 2 5 10 20; do
  for batch in $(seq 1 "$batches_per_level"); do
    pids=()
    for worker in $(seq 1 "$concurrency"); do
      curl --fail --silent --show-error --config "$work/curl-auth" -H 'Content-Type: application/json' --data-binary "@$work/stream.json" -o "$work/stream-$concurrency-$batch-$worker.sse" -w '%{time_total}' "$origin/v1/chat/completions" >"$work/time-$concurrency-$batch-$worker" &
      pids+=("$!")
    done
    for pid in "${pids[@]}"; do wait "$pid"; done
  done
  "$python_command" -c 'import glob,sys; root,n,b=sys.argv[1],int(sys.argv[2]),int(sys.argv[3]); files=glob.glob(f"{root}/stream-{n}-*.sse"); assert len(files)==n*b; bodies=[open(path).read() for path in files]; assert all(body.endswith("data: [DONE]\n\n") and "\"model\":\"assistant\"" in body and "modelcairn_error" not in body for body in bodies)' "$work" "$concurrency" "$batches_per_level"
  average="$(awk '{sum += $1} END {printf "%.6f", sum/NR}' "$work"/time-"$concurrency"-*)"
  maximum="$(awk 'BEGIN {max=0} $1>max {max=$1} END {printf "%.6f", max}' "$work"/time-"$concurrency"-*)"
  printf '%s %s %s %s\n' "$concurrency" "$((concurrency * batches_per_level))" "$average" "$maximum" >>"$work/results"
done

sustained_successful=0
sustained_panel_queries=0
sustained_average="n/a"
sustained_maximum="n/a"
if (( sustained_seconds > 0 )); then
  sustained_deadline=$((SECONDS + sustained_seconds))
  pids=()
  for worker in $(seq 1 "$sustained_concurrency"); do
    (
      count=0
      : >"$work/sustained-times-$worker"
      while (( SECONDS < sustained_deadline )); do
        curl --fail --silent --show-error --config "$work/curl-auth" -H 'Content-Type: application/json' \
          --data-binary "@$work/stream.json" -o "$work/sustained-$worker.sse" -w '%{time_total}\n' \
          "$origin/v1/chat/completions" >>"$work/sustained-times-$worker"
        grep -Fq 'data: [DONE]' "$work/sustained-$worker.sse"
        grep -Fq '"model":"assistant"' "$work/sustained-$worker.sse"
        ! grep -Fq 'modelcairn_error' "$work/sustained-$worker.sse"
        ((count += 1))
      done
      printf '%s\n' "$count" >"$work/sustained-count-$worker"
    ) &
    pids+=("$!")
  done
  (
    count=0
    while (( SECONDS < sustained_deadline )); do
      curl --fail --silent --show-error --config "$work/curl-admin" \
        "$origin/api/v1/admin/overview" -o "$work/sustained-overview.json"
      "$python_command" -c 'import json,sys; value=json.load(open(sys.argv[1])); assert isinstance(value.get("recentRequests"),list) and isinstance(value.get("resourceCounts"),dict)' "$work/sustained-overview.json"
      ((count += 1))
      sleep 1
    done
    printf '%s\n' "$count" >"$work/sustained-panel-count"
  ) &
  pids+=("$!")
  for pid in "${pids[@]}"; do wait "$pid"; done
  sustained_successful="$(awk '{sum += $1} END {print sum+0}' "$work"/sustained-count-*)"
  sustained_panel_queries="$(cat "$work/sustained-panel-count")"
  sustained_average="$(awk '{sum += $1; count += 1} END {if (count) printf "%.6f", sum/count; else print "n/a"}' "$work"/sustained-times-*)"
  sustained_maximum="$(awk 'BEGIN {max=0} $1>max {max=$1} END {printf "%.6f", max}' "$work"/sustained-times-*)"
  (( sustained_successful > 0 && sustained_panel_queries > 0 )) ||
    { echo "sustained mixed load produced no successful work" >&2; exit 1; }
fi

cpu_ticks_end="$(awk '{print $14+$15}' "/proc/$service_pid/stat")"
wall_end_ns="$(date +%s%N)"
clock_ticks="$(getconf CLK_TCK)"
cpu_seconds="$(awk -v ticks="$((cpu_ticks_end - cpu_ticks_start))" -v hz="$clock_ticks" 'BEGIN {printf "%.3f", ticks/hz}')"
average_cpu_percent="$(awk -v ticks="$((cpu_ticks_end - cpu_ticks_start))" -v hz="$clock_ticks" -v ns="$((wall_end_ns - wall_start_ns))" 'BEGIN {if (ns<=0) print "0.000"; else printf "%.3f", 100*(ticks/hz)/(ns/1000000000)}')"
data_bytes="$(find "$data" -type f -printf '%s\n' | awk '{sum += $1} END {print sum+0}')"
database_bytes="$(stat -c %s "$data/modelcairn.db")"
wal_bytes="$(stat -c %s "$data/modelcairn.db-wal" 2>/dev/null || printf '0')"
if (( sustained_seconds > 0 )); then
  awk -v actual="$average_cpu_percent" 'BEGIN {exit !(actual <= 50)}' ||
    { echo "average CPU exceeded sustained budget: $average_cpu_percent% > 50%" >&2; exit 1; }
  awk -v actual="$sustained_average" 'BEGIN {exit !(actual <= 0.350)}' ||
    { echo "average latency exceeded sustained budget: $sustained_average s > 0.350 s" >&2; exit 1; }
  awk -v actual="$sustained_maximum" 'BEGIN {exit !(actual <= 2.000)}' ||
    { echo "maximum latency exceeded sustained budget: $sustained_maximum s > 2.000 s" >&2; exit 1; }
  (( data_bytes <= 33554432 )) ||
    { echo "data directory exceeded sustained budget: $data_bytes bytes > 33554432 bytes" >&2; exit 1; }
  if (( sustained_seconds >= 600 )); then
    (( sustained_concurrency >= 10 && sustained_successful >= 10000 && sustained_panel_queries >= 300 )) ||
      { echo "representative throughput floor was not met" >&2; exit 1; }
  fi
fi

kill "$sampler_pid" 2>/dev/null || true
wait "$sampler_pid" 2>/dev/null || true
sampler_pid=""
samples="$(wc -l <"$work/rss-samples")"
(( samples > 0 )) || { echo "no service memory samples collected" >&2; exit 1; }
average_rss="$(awk '{sum += $1} END {printf "%d", sum/NR}' "$work/rss-samples")"
peak_rss="$(awk 'BEGIN {max=0} $1>max {max=$1} END {print max}' "$work/rss-samples")"
peak_swap="$(awk 'BEGIN {max=0} $2>max {max=$2} END {print max}' "$work/rss-samples")"
(( peak_rss <= max_rss_kib )) ||
  { echo "service exceeded RSS budget: $peak_rss KiB > $max_rss_kib KiB" >&2; exit 1; }
if grep -R -a -Fq --exclude='modelcairn.lock' 'nonstream-canary' "$data" ||
   grep -R -a -Fq --exclude='modelcairn.lock' 'stream-canary' "$data" ||
   grep -R -a -Fq --exclude='modelcairn.lock' 'benchmark-ok' "$data"; then
  echo "request content was persisted" >&2
  exit 1
fi
for canary in benchmark-provider-secret nonstream-canary stream-canary benchmark-ok; do
  grep -Fq "$canary" "$work/service.log" &&
    { echo "secret or content canary leaked to service log" >&2; exit 1; }
done

if [[ -n "$output_file" ]]; then
  mkdir -p "$(dirname "$output_file")"
  {
    echo "# ModelCairn Milestone 4 streaming resource report"
    echo
    echo "- Recorded at (UTC): $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "- Kernel: $(uname -srmo)"
    echo "- Architecture: $(uname -m)"
    echo "- Logical CPUs: $(getconf _NPROCESSORS_ONLN)"
    echo "- Total memory: $(awk '/^MemTotal:/ {print $2}' /proc/meminfo) KiB"
    echo "- Binary: $($binary version)"
    echo "- Binary size: $(stat -c %s "$binary") bytes"
    echo "- Samples: $samples at approximately 50 ms"
    echo "- Average service RSS: $average_rss KiB"
    echo "- Peak service RSS: $peak_rss KiB"
    echo "- Enforced peak RSS budget: $max_rss_kib KiB"
    echo "- Peak service swap: $peak_swap KiB"
    echo "- Process CPU time: $cpu_seconds seconds"
    echo "- Average process CPU: $average_cpu_percent% of one logical CPU"
    echo "- Final data-directory size: $data_bytes bytes"
    echo "- Final SQLite database/WAL size: $database_bytes/$wal_bytes bytes"
    echo "- Batches per concurrency level: $batches_per_level"
    echo "- Sustained mixed-load duration: $sustained_seconds seconds"
    echo "- Sustained stream concurrency: $sustained_concurrency"
    echo "- Sustained successful streams: $sustained_successful"
    echo "- Sustained panel queries: $sustained_panel_queries"
    echo "- Sustained average/maximum stream latency: $sustained_average/$sustained_maximum seconds"
    echo "- Sustained acceptance budgets: CPU <= 50% of one logical CPU; average/maximum latency <= 0.350/2.000 seconds; data <= 33554432 bytes"
    echo "- Representative floors at >=600s: concurrency >= 10; streams >= 10000; panel queries >= 300"
    echo
    echo "| Concurrent streams | Successful | Average latency (s) | Maximum latency (s) |"
    echo "|---:|---:|---:|---:|"
    while read -r concurrency successful average maximum; do
      echo "| $concurrency | $successful | $average | $maximum |"
    done <"$work/results"
  } >"$output_file"
fi

echo "Milestone 4 streaming gate passed (peak RSS $peak_rss KiB)"
