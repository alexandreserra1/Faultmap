# Demo Shop

Esta demonstração fecha o ciclo do MVP com dois serviços instrumentados, PostgreSQL, OpenTelemetry Collector e o receiver OTLP HTTP do Faultmap. A carga sempre entra pelo `checkout-service`; os spans correlacionam checkout, chamada ao pagamento e persistência em `payments`.

## Pré-requisitos

- Docker com Compose v2;
- `curl`;
- portas locais `18080`, `4318` e `8081` disponíveis.

O ambiente é local e usa as credenciais deliberadamente não secretas `demo/demo`. Não reutilize essa configuração fora da demonstração.

## Fluxo básico

```bash
docker compose -f examples/demo-shop/compose.yaml up --build -d --wait
curl --silent --show-error --max-time 5 http://localhost:8081/health
curl --silent --show-error --max-time 5 \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"manual-1","amount_cents":1990}' \
  http://localhost:18080/checkout
```

O Collector envia OTLP ao Faultmap, que normaliza e persiste os sinais no SQLite. A API permanece stateless; o estado durável fica no banco compartilhado pelo processo.

O exportador agrupa spans por até um segundo. Depois de gerar tráfego, aguarde
ao menos esse intervalo antes de consultar ou diagnosticar:

```bash
sleep 2
docker compose -f examples/demo-shop/compose.yaml exec -T faultmap \
  faultmap telemetry list \
  --config /etc/faultmap/faultmap.yaml \
  --service checkout-service \
  --since 5m \
  --limit 20
```

O gerador opcional produz carga finita e limitada dentro da rede Docker:

```bash
REQUESTS=20 CONCURRENCY=4 \
  docker compose -f examples/demo-shop/compose.yaml \
  --profile tools run --rm load-generator
```

## Cenários

Cada diretório contém um override pequeno, um gerador de carga limitado e o diagnóstico esperado:

- [`database-slow`](scenarios/database-slow/README.md): operação PostgreSQL lenta;
- [`small-pool`](scenarios/small-pool/README.md): contenção com pool de uma conexão;
- [`payment-500`](scenarios/payment-500/README.md): falha HTTP determinística no pagamento;
- [`retry-storm`](scenarios/retry-storm/README.md): repetição da mesma chamada no mesmo trace;
- [`timeout-after-deploy`](scenarios/timeout-after-deploy/README.md): regressão de timeout associável a uma nova versão/deployment;
- [`table-lock`](scenarios/table-lock/README.md): lock exclusivo curto e controlado em `payments`.

Execute um cenário por vez e use `up --build -d --wait` antes de iniciar a carga. `docker compose down` remove os containers e a rede, mas preserva volumes nomeados; acrescente `--volumes` somente se quiser apagar conscientemente os dados locais da demo e obter uma baseline inteiramente limpa.

## Contratos de runtime

Os overrides usam somente controles explícitos dos serviços: `PORT`, `SERVICE_VERSION`, `DB_DELAY`, `DB_MAX_OPEN_CONNS`, `FORCE_HTTP_STATUS`, `PAYMENT_TIMEOUT` e `PAYMENT_MAX_ATTEMPTS`. A carga usa IDs únicos e quantidade configurável, evitando duplicação acidental e execuções ilimitadas.

Todas as portas publicadas usam bind em `127.0.0.1`. A configuração não
possui autenticação nem TLS e não deve ser adaptada para exposição externa.
