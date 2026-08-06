# Cenário: pagamento retorna HTTP 500

O `payment-service` passa a responder HTTP 500 de forma determinística. O controle `FORCE_HTTP_STATUS` existe somente para a demo e não expõe um endpoint administrativo.

## Como executar

```bash
docker compose -f examples/demo-shop/compose.yaml up --build -d --wait
RUN_ID=payment-500-baseline REQUEST_COUNT=12 \
  examples/demo-shop/scenarios/payment-500/generate-traffic.sh
sleep 3

docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/payment-500/compose.yaml \
  up --build -d --wait payment-service
examples/demo-shop/scenarios/payment-500/generate-traffic.sh
```

Diagnostique o pagamento e, se quiser observar a propagação, repita para `checkout-service`:

```bash
docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/payment-500/compose.yaml \
  exec faultmap faultmap diagnose incident \
  --config /etc/faultmap/faultmap.yaml \
  --service payment-service --since 15s --baseline 30s --limit 500
```

## Resultado esperado

`error_rate_delta` deve mostrar a taxa de HTTP 500 saindo de zero na janela do incidente, com `payment-service` no ranking. O trace permite investigar a chamada que saiu do checkout e terminou no pagamento, mas a correlação ainda é apresentada como evidência, não como causalidade definitiva.

## Fonte OTLP

O script realiza 12 POSTs com IDs distintos e captura o status sem usar `curl --fail`, pois HTTP 500 é o resultado observado. Falhas de transporte aparecem separadamente como `transport-error`. Os spans são produzidos pelos serviços, não por uma fixture sintética.
