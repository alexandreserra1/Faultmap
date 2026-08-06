# Cenário: timeout depois de uma mudança

A baseline executa a versão normal. O override muda `SERVICE_VERSION` para `2.0.0-timeout-regression`, reduz o timeout do checkout para `150ms` e adiciona `800ms` antes da operação do pagamento. Isso cria uma regressão temporal identificável nos spans.

## Como executar

```bash
docker compose -f examples/demo-shop/compose.yaml up --build -d --wait
RUN_ID=timeout-deploy-baseline REQUEST_COUNT=12 \
  examples/demo-shop/scenarios/timeout-after-deploy/generate-traffic.sh
sleep 3

docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/timeout-after-deploy/compose.yaml \
  up --build -d --wait checkout-service payment-service
examples/demo-shop/scenarios/timeout-after-deploy/generate-traffic.sh
```

Diagnostique o checkout logo após a carga:

```bash
docker compose -f examples/demo-shop/compose.yaml \
  -f examples/demo-shop/scenarios/timeout-after-deploy/compose.yaml \
  exec faultmap faultmap diagnose incident \
  --config /etc/faultmap/faultmap.yaml \
  --service checkout-service --since 15s --baseline 30s --limit 500
```

Para incluir apenas a proximidade temporal de `deployment_proximity`, importe pelo comando `ingest github` um deployment do mesmo serviço e ambiente. Para elevar a confiança por correspondência exata, inicie o override com `TIMEOUT_DEPLOY_VERSION=<commit_sha>` usando o SHA real importado como `service.version`; o texto `2.0.0-timeout-regression` é apenas o fallback local e não corresponde a um SHA do GitHub. Depois repita o diagnóstico com `--environment demo`. O token deve vir somente de `GITHUB_TOKEN`.

## Resultado esperado

Sem dados de mudança, o Faultmap deve apontar aumento de latência/erros e registrar a versão observada, mas não inventará um commit. Com um deployment real previamente importado, `deployment_proximity` deve contribuir para o ranking e declarar que proximidade e versão correspondente não comprovam causalidade.

## Fonte OTLP

O script produz 12 traces reais depois da troca de versão. `service.version` e `deployment.environment.name` viajam como atributos de Resource, permitindo a correlação com um deployment persistido sem guardar estado de sessão na API.
