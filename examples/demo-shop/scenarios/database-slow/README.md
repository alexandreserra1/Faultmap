# Cenário: banco lento

O `payment-service` passa a esperar `750ms` antes da operação em PostgreSQL. A baseline usa a configuração normal; o incidente usa a versão `1.1.0-database-slow`. O atraso é explícito e reversível, sem alterar dados ou schema.

## Como executar

Na raiz do repositório, suba a baseline e gere uma amostra limitada:

```bash
docker compose -f examples/demo-shop/compose.yaml up --build -d --wait
RUN_ID=database-slow-baseline REQUEST_COUNT=12 \
  examples/demo-shop/scenarios/database-slow/generate-traffic.sh
sleep 3
```

Aplique somente o override do cenário e gere o incidente:

```bash
docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/database-slow/compose.yaml \
  up --build -d --wait payment-service
examples/demo-shop/scenarios/database-slow/generate-traffic.sh
```

Execute o diagnóstico logo após a carga:

```bash
docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/database-slow/compose.yaml \
  exec faultmap faultmap diagnose incident \
  --config /etc/faultmap/faultmap.yaml \
  --service payment-service --since 15s --baseline 30s --limit 500
```

## Resultado esperado

O ranking deve priorizar `payment-service`. A evidência esperada é `latency_delta`, comparando o p95 HTTP do pagamento antes e depois do atraso. Dependendo do timeout configurado, `database_timeout` também pode aparecer. O resultado é uma hipótese: atraso controlado e correlação temporal não provam sozinhos causalidade.

## Fonte OTLP

O script envia exatamente 12 checkouts, um por segundo, com `order_id` único. A instrumentação real produz spans OTLP de checkout, chamada HTTP ao pagamento e PostgreSQL; o Collector os encaminha ao receiver `/v1/traces`. `REQUEST_COUNT`, `REQUEST_TIMEOUT` e `REQUEST_INTERVAL` são limitados e configuráveis.
