#!/bin/sh
set -eu

CHECKOUT_URL="${CHECKOUT_URL:-http://localhost:18080/checkout}"
REQUEST_COUNT="${REQUEST_COUNT:-20}"
REQUEST_TIMEOUT="${REQUEST_TIMEOUT:-4}"
CONCURRENCY="${CONCURRENCY:-5}"
RUN_ID="${RUN_ID:-small-pool-incident}"

case "$REQUEST_COUNT:$CONCURRENCY" in
  *[!0-9:]*) echo "REQUEST_COUNT e CONCURRENCY devem ser inteiros positivos" >&2; exit 2 ;;
esac
[ "$REQUEST_COUNT" -gt 0 ] && [ "$CONCURRENCY" -gt 0 ] || {
  echo "REQUEST_COUNT e CONCURRENCY devem ser maiores que zero" >&2
  exit 2
}

# Cada worker absorve a falha esperada e sempre termina, de modo que wait não
# interrompe os lotes seguintes durante a contenção do pool.
send_request() {
  number="$1"
  code=$(curl --silent --show-error --max-time "$REQUEST_TIMEOUT" \
    --output /dev/null --write-out '%{http_code}' \
    --header 'Content-Type: application/json' \
    --data "{\"order_id\":\"${RUN_ID}-${number}\",\"amount_cents\":1990}" \
    "$CHECKOUT_URL") || code="transport-error"
  printf '%s request=%s status=%s\n' "$RUN_ID" "$number" "$code"
}

request_number=1
active=0
while [ "$request_number" -le "$REQUEST_COUNT" ]; do
  send_request "$request_number" &
  active=$((active + 1))
  request_number=$((request_number + 1))
  if [ "$active" -ge "$CONCURRENCY" ]; then
    wait
    active=0
  fi
done
[ "$active" -eq 0 ] || wait
