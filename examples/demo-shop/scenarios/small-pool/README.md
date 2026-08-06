# Cenário: pool pequeno

O `payment-service` é reiniciado com apenas uma conexão aberta/ociosa e atraso de `300ms` no banco. Cinco requisições concorrentes por lote disputam esse recurso, tornando a contenção reproduzível sem abrir pools adicionais.

## Como executar

```bash
docker compose -f examples/demo-shop/compose.yaml up --build -d --wait
RUN_ID=small-pool-baseline REQUEST_COUNT=20 CONCURRENCY=5 \
  examples/demo-shop/scenarios/small-pool/generate-traffic.sh
sleep 3

docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/small-pool/compose.yaml \
  up --build -d --wait payment-service
examples/demo-shop/scenarios/small-pool/generate-traffic.sh
```

Diagnostique o serviço que possui o pool e os spans PostgreSQL:

```bash
docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/small-pool/compose.yaml \
  exec faultmap faultmap diagnose incident \
  --config /etc/faultmap/faultmap.yaml \
  --service payment-service --since 15s --baseline 30s --limit 500
```

## Resultado esperado

O Faultmap deve mostrar aumento de latência em `payment-service` e pode registrar timeouts quando a espera ultrapassar o contexto. O MVP observa o efeito nos spans; sem métrica explícita de espera pelo pool, não deve afirmar que o tamanho do pool é a causa comprovada.

## Fonte OTLP

`generate-traffic.sh` cria 20 pedidos em lotes concorrentes de no máximo cinco. Cada worker possui timeout, absorve falhas esperadas e termina; não há loop infinito. Os serviços geram os spans reais encaminhados pelo Collector.
