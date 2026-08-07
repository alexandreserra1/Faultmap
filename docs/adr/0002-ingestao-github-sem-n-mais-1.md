# ADR 0002 — Ingestão do GitHub limitada a uma página, sem requisições N+1

- Status: aceito
- Data: 2026-08-06

## Contexto

A especificação pede commits com arquivos modificados e deployments com o status
atual. Os endpoints de listagem do GitHub não trazem nenhum dos dois: obter os
arquivos exige `GET /repos/{owner}/{repo}/commits/{sha}` por commit, e o status
exige `GET /repos/{owner}/{repo}/deployments/{id}/statuses` por deployment.

Buscá-los durante a importação significaria uma requisição HTTP por registro —
exatamente o padrão N+1 que as diretrizes de implementação proíbem, agravado
pelo limite de taxa da API.

## Decisão

Cada execução importa no máximo uma página de 100 commits e uma página de 100
deployments, persistidas atomicamente e de forma idempotente pelo identificador.
Arquivos por commit e status de deployment permanecem explicitamente vazios ou
desconhecidos.

O detector `deployment_proximity` declara a limitação correspondente sempre que
não consegue confirmar o sucesso do deployment.

## Consequências

- A importação tem custo previsível: duas requisições, independentemente do volume.
- A confiança de `deployment_proximity` é sustentada pela correspondência entre
  `commit_sha` e `service.version`, não pelo status do deployment.
- Uma evolução que precise desses campos deve usar uma operação em lote (por
  exemplo GraphQL) ou justificar e medir uma estratégia limitada — nunca um laço
  de requisições por registro.
