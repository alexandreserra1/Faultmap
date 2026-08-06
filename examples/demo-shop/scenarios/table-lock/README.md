# Cenário: lock na tabela de pagamentos

Um container efêmero abre uma transação, adquire `ACCESS EXCLUSIVE` em `payments`, espera oito segundos e executa `COMMIT`. `statement_timeout=10s` impede espera ilimitada. O lock não altera nem apaga registros.

## Como executar

```bash
docker compose -f examples/demo-shop/compose.yaml up --build -d --wait
RUN_ID=table-lock-baseline REQUEST_COUNT=12 \
  examples/demo-shop/scenarios/table-lock/generate-traffic.sh
sleep 3

docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/table-lock/compose.yaml \
  up --build -d --wait payment-service
docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/table-lock/compose.yaml \
  run --rm -d lock-holder
examples/demo-shop/scenarios/table-lock/generate-traffic.sh
```

Diagnostique os spans de banco pertencentes ao pagamento:

```bash
docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/table-lock/compose.yaml \
  exec faultmap faultmap diagnose incident \
  --config /etc/faultmap/faultmap.yaml \
  --service payment-service --since 15s --baseline 30s --limit 500
```

## Resultado esperado

Durante o lock, algumas inserções devem atrasar ou respeitar o timeout recebido. O diagnóstico deve elevar `latency_delta` e, quando o span PostgreSQL terminar como timeout, `database_timeout`. Após oito segundos, o `COMMIT` libera a tabela e novas requisições voltam ao padrão normal.

## Fonte OTLP

O lock e a carga são reais e determinísticos: duração fixa, 12 requisições e timeout por chamada. Os spans de PostgreSQL saem do `payment-service` e são encaminhados pelo Collector; nenhuma query completa ou parâmetro sensível precisa ser registrado pelo Faultmap.
