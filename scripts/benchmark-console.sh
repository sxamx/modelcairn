#!/usr/bin/env bash
set -euo pipefail

binary="${1:?usage: benchmark-console.sh /path/to/modelcairn}"
duration="${DURATION_SECONDS:-180}"
interval="${SAMPLE_INTERVAL_SECONDS:-2}"
listen="${LISTEN_ADDRESS:-127.0.0.1:18085}"
work="$(mktemp -d)"
pid=""
cleanup() {
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then kill -TERM "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi
  rm -rf -- "$work"
}
trap cleanup EXIT

chmod 700 "$binary"
"$binary" serve --listen "$listen" >"$work/server.log" 2>&1 &
pid=$!
for _ in {1..50}; do
  curl --fail --silent --max-time 1 "http://$listen/healthz" >/dev/null && break
  kill -0 "$pid" 2>/dev/null || { cat "$work/server.log" >&2; exit 1; }
  sleep .1
done
curl --fail --silent --max-time 2 "http://$listen/" | grep -q '<div id="root"></div>'
curl --fail --silent --max-time 2 "http://$listen/manifest.webmanifest" | grep -q 'ModelCairn'
curl --fail --silent --max-time 2 -D "$work/headers" -o /dev/null "http://$listen/"
grep -qi '^Cache-Control: no-cache' "$work/headers"

samples=0; sum=0; peak=0; swap_peak=0; deadline=$((SECONDS + duration))
while (( SECONDS < deadline )); do
  rss="$(awk '/^VmRSS:/ {print $2}' "/proc/$pid/status")"
  swap="$(awk '/^VmSwap:/ {print $2}' "/proc/$pid/status")"
  ((samples+=1)); ((sum+=rss)); ((rss>peak)) && peak=$rss; ((swap>swap_peak)) && swap_peak=$swap
  sleep "$interval"
done
printf 'duration_seconds=%s\nsamples=%s\naverage_rss_kib=%s\npeak_rss_kib=%s\npeak_swap_kib=%s\nbinary_bytes=%s\n' \
  "$duration" "$samples" "$((sum/samples))" "$peak" "$swap_peak" "$(stat -c %s "$binary")"
(( peak <= 131072 ))
(( swap_peak == 0 ))
