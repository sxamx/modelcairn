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

printf '%s\n' 'a secure password' |
  "$binary" admin login --server "$origin" --session-file "$work/session.json" --username owner >/dev/null
printf '%s\n' 'benchmark-provider-secret' |
  "$binary" secret set --server "$origin" --session-file "$work/session.json" example-key-secret >/dev/null
cp docs/contratos/config/example-v1alpha1.yaml "$work/config.yaml"
sed -i -e "s#https://api.example.invalid/v1#$upstream_origin/v1#" -e 's/allowPrivateNetwork: false/allowPrivateNetwork: true/' -e 's/attemptTimeoutMs: 60000/attemptTimeoutMs: 5000/' -e 's/totalTimeoutMs: 120000/totalTimeoutMs: 10000/' "$work/config.yaml"
"$binary" config plan --server "$origin" --session-file "$work/session.json" --out "$work/plan.json" "$work/config.yaml" >/dev/null
"$binary" config apply --server "$origin" --session-file "$work/session.json" --plan "$work/plan.json" "$work/config.yaml" >/dev/null
"$binary" agent-token issue --server "$origin" --session-file "$work/session.json" example-agent >"$work/issued.json"
agent_token="$("$python_command" -c 'import json,sys; value=json.load(open(sys.argv[1]))["token"]; assert value.startswith("mc_at_v1_"); print(value)' "$work/issued.json")"
printf 'header = "Authorization: Bearer %s"\n' "$agent_token" >"$work/curl-auth"
chmod 600 "$work/curl-auth"
unset agent_token

printf '%s\n' '{"model":"example-assistant","messages":[{"role":"user","content":"nonstream-canary"}]}' >"$work/nonstream.json"
curl --fail --silent --show-error --config "$work/curl-auth" -H 'Content-Type: application/json' --data-binary "@$work/nonstream.json" "$origin/v1/chat/completions" >"$work/nonstream-response.json"
"$python_command" -c 'import json,sys; value=json.load(open(sys.argv[1])); assert value["model"]=="example-assistant"; assert value["choices"][0]["message"]["content"]=="benchmark-ok"' "$work/nonstream-response.json"

printf '%s\n' '{"model":"example-assistant","messages":[{"role":"user","content":"stream-canary"}],"stream":true}' >"$work/stream.json"
: >"$work/results"
for concurrency in 1 2 5 10 20; do
  pids=()
  for worker in $(seq 1 "$concurrency"); do
    curl --fail --silent --show-error --config "$work/curl-auth" -H 'Content-Type: application/json' --data-binary "@$work/stream.json" -o "$work/stream-$concurrency-$worker.sse" -w '%{time_total}' "$origin/v1/chat/completions" >"$work/time-$concurrency-$worker" &
    pids+=("$!")
  done
  for pid in "${pids[@]}"; do wait "$pid"; done
  "$python_command" -c 'import glob,sys; root,n=sys.argv[1],int(sys.argv[2]); files=glob.glob(f"{root}/stream-{n}-*.sse"); assert len(files)==n; bodies=[open(path).read() for path in files]; assert all(body.endswith("data: [DONE]\n\n") and "\"model\":\"example-assistant\"" in body and "modelcairn_error" not in body for body in bodies)' "$work" "$concurrency"
  average="$(awk '{sum += $1} END {printf "%.6f", sum/NR}' "$work"/time-"$concurrency"-*)"
  maximum="$(awk 'BEGIN {max=0} $1>max {max=$1} END {printf "%.6f", max}' "$work"/time-"$concurrency"-*)"
  printf '%s %s %s\n' "$concurrency" "$average" "$maximum" >>"$work/results"
done

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
   grep -R -a -Fq --exclude='modelcairn.lock' 'stream-canary' "$data"; then
  echo "request content was persisted" >&2
  exit 1
fi
grep -Fq 'benchmark-provider-secret' "$work/service.log" &&
  { echo "provider secret leaked to service log" >&2; exit 1; }

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
    echo
    echo "| Concurrent streams | Successful | Average latency (s) | Maximum latency (s) |"
    echo "|---:|---:|---:|---:|"
    while read -r concurrency average maximum; do
      echo "| $concurrency | $concurrency | $average | $maximum |"
    done <"$work/results"
  } >"$output_file"
fi

echo "Milestone 4 streaming gate passed (peak RSS $peak_rss KiB)"
