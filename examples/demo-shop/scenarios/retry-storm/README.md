# Cenário: tempestade de retries

O checkout tenta chamar o pagamento até quatro vezes. O pagamento responde 503, portanto chamadas equivalentes aparecem repetidas no mesmo trace. O cenário usa oito traces, acima da amostra mínima do detector.

## Como executar

```bash
docker compose -f examples/demo-shop/compose.yaml up --build -d --wait
RUN_ID=retry-storm-baseline REQUEST_COUNT=8 \
  examples/demo-shop/scenarios/retry-storm/generate-traffic.sh
sleep 3

docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/retry-storm/compose.yaml \
  up --build -d --wait checkout-service payment-service
examples/demo-shop/scenarios/retry-storm/generate-traffic.sh
```

O detector analisa os spans client do checkout:

```bash
docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/retry-storm/compose.yaml \
  exec faultmap faultmap diagnose incident \
  --config /etc/faultmap/faultmap.yaml \
  --service checkout-service --since 15s --baseline 30s --limit 500
```

## Resultado esperado

A saída deve conter `retry_storm`, média próxima de uma tentativa por trace na baseline e quatro no incidente, além de `error_rate_delta`. O texto deve preservar as limitações: fan-out, paginação, loop de negócio ou instrumentação duplicada também podem criar spans semelhantes.

## Fonte OTLP

Cada POST externo produz um trace real. `PAYMENT_MAX_ATTEMPTS=4` cria a repetição internamente, preservando os mesmos `trace_id` e operação cliente. O script limita a carga a oito requisições e aplica timeout em cada uma.
