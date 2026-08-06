# Cenário: timeout depois de uma mudança

A baseline executa a versão normal. O override muda `SERVICE_VERSION` para um SHA de commit, reduz o timeout do checkout para `150ms` e adiciona `800ms` antes da operação do pagamento. Isso cria uma regressão temporal identificável nos spans e uma mudança importável pelo Faultmap.

## Como executar

```bash
make demo-down
make demo-test-e2e E2E_SCENARIOS="timeout-after-deploy"
```

O runner cria bancos isolados, gera a baseline, sobe um mock mínimo da API do
GitHub no loopback do próprio container do Faultmap, importa um commit e seu
deployment e só então produz o incidente. O token `e2e-token` não possui acesso
ao GitHub real e autentica exclusivamente esse mock local.

Para reproduzir manualmente a preparação dos serviços, use:

```bash
docker compose -f examples/demo-shop/compose.yaml up --build -d --wait
RUN_ID=timeout-deploy-baseline REQUEST_COUNT=12 \
  examples/demo-shop/scenarios/timeout-after-deploy/generate-traffic.sh
sleep 3

docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/timeout-after-deploy/compose.yaml \
  up --build -d --wait faultmap checkout-service payment-service
examples/demo-shop/scenarios/timeout-after-deploy/generate-traffic.sh
```

Diagnostique o checkout logo após a carga:

```bash
docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/timeout-after-deploy/compose.yaml \
  exec faultmap faultmap diagnose incident \
  --config /etc/faultmap/faultmap.yaml \
  --service checkout-service --environment demo \
  --since 15s --baseline 30s --limit 500
```

Na execução manual, inicie o `github-mock` dentro do container e use `faultmap
ingest github` antes de gerar o incidente. Para substituir o SHA determinístico
do E2E, inicie o override com `TIMEOUT_DEPLOY_VERSION=<commit_sha>`; o mesmo
valor alimenta o mock e `service.version`. Em uma integração real, o token deve
vir somente de `GITHUB_TOKEN`.

## Resultado esperado

O teste automatizado deve apontar `checkout-service`, encontrar
`error_rate_delta`, `latency_delta` e `deployment_proximity` com confiança alta
e afirmar que o commit corresponde à `service.version` observada. O relatório
continua declarando que proximidade temporal e versão correspondente não
comprovam causalidade.

## Fonte OTLP

O script produz 12 traces reais depois da troca de versão. `service.version` e `deployment.environment.name` viajam como atributos de Resource, permitindo a correlação com um deployment persistido sem guardar estado de sessão na API.
