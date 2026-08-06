#!/bin/sh
set -eu

CHECKOUT_URL="${CHECKOUT_URL:-http://localhost:18080/checkout}"
REQUEST_COUNT="${REQUEST_COUNT:-12}"
REQUEST_TIMEOUT="${REQUEST_TIMEOUT:-3}"
REQUEST_INTERVAL="${REQUEST_INTERVAL:-1}"
RUN_ID="${RUN_ID:-timeout-deploy-incident}"

case "$REQUEST_COUNT" in
  ''|*[!0-9]*) echo "REQUEST_COUNT deve ser um inteiro positivo" >&2; exit 2 ;;
esac
[ "$REQUEST_COUNT" -gt 0 ] || { echo "REQUEST_COUNT deve ser maior que zero" >&2; exit 2; }

# SERVICE_VERSION diferencia os sinais depois da mudança; o gerador mantém IDs
# únicos para que retries manuais não sejam confundidos com deduplicação.
request_number=1
while [ "$request_number" -le "$REQUEST_COUNT" ]; do
  code=$(curl --silent --show-error --max-time "$REQUEST_TIMEOUT" \
    --output /dev/null --write-out '%{http_code}' \
    --header 'Content-Type: application/json' \
    --data "{\"order_id\":\"${RUN_ID}-${request_number}\",\"amount_cents\":1990}" \
    "$CHECKOUT_URL") || code="transport-error"
  printf '%s request=%s status=%s\n' "$RUN_ID" "$request_number" "$code"
  request_number=$((request_number + 1))
  [ "$request_number" -gt "$REQUEST_COUNT" ] || sleep "$REQUEST_INTERVAL"
done
