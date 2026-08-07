# ADR 0001 — Detectores estruturais reutilizam o peso `graph_proximity`

- Status: aceito
- Data: 2026-08-06

## Contexto

A configuração normativa do MVP fixa seis pesos de ranking: `error_rate_delta`,
`deployment_proximity`, `database_evidence`, `graph_proximity`, `latency_delta` e
`log_correlation`. Dois detectores implementados depois dessa definição não
possuem peso próprio: `database_http_trace_correlation`, que liga um timeout de
banco a um erro HTTP dentro do mesmo trace, e `retry_storm`, que identifica
repetição anormal da mesma operação dentro de traces comparáveis.

Havia duas saídas: acrescentar campos ao YAML ou mapear os dois detectores para
um peso existente.

## Decisão

Ambos usam o peso `graph_proximity`. A correlação de banco representa a ligação
entre sinais no mesmo trace; o retry storm representa repetição estrutural da
mesma operação dentro do trace. As duas são, em essência, evidência de
proximidade no grafo.

Cada finding continua contribuindo individualmente com `score × peso`, e cada
contribuição permanece visível na saída e no snapshot.

## Consequências

- O contrato do YAML permanece estável e a soma configurada dos pesos é preservada;
  workspaces existentes não precisam migrar.
- Um serviço com correlação de banco **e** retry storm acumula duas contribuições
  da mesma classe. O total agregado continua limitado ao intervalo de 0 a 1 e
  cada parcela permanece auditável, mas a classe pesa mais nesse caso.
- Se um terceiro detector estrutural surgir com semântica diferente das duas
  atuais, este ADR deve ser revisitado antes de simplesmente reaproveitar o peso
  uma terceira vez.
