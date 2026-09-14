# ADR 0010 — Teto por classe de peso no ranking

- Status: aceito
- Data: 2026-08-10
- Revisita: [ADR 0001](0001-ranking-reutiliza-peso-graph-proximity.md)

## Contexto

O ADR 0001 decidiu que dois detectores estruturais compartilhariam o peso
`graph_proximity` e registrou, textualmente, que **um terceiro exigiria
revisitar a decisão antes de reutilizar o peso de novo**. A implementação dos
quatro detectores restantes do documento normativo trouxe o terceiro e o quarto:
`dependency_failure` e `trace_break` se somam a `database_http_trace_correlation`
e `retry_storm`.

Somando livremente, um serviço com as quatro regras acumularia
`4 × 0.15 = 0.60` só de evidência estrutural, contra `0.25` de
`error_rate_delta` sozinho. A proporção entre as classes deixaria de ser a
configurada no YAML e passaria a depender de quantas regras existem em cada
classe — mudando sozinha a cada detector acrescentado.

O mesmo já valia, em menor grau, para `database_evidence`, agora com
`database_timeout` e `database_error`, e para `deployment_proximity`, agora com
`deployment_proximity` e `version_regression`.

## Decisão

O ranking acumula por **classe de peso** e limita o total de cada classe ao peso
configurado para ela. As contribuições individuais continuam todas visíveis na
saída, com seus valores originais: o que é limitado é o total da classe, não o
que é apresentado a quem investiga.

A estrutura interna passou a nomear as classes (`weightClassForRule`,
`weightForClass`), tornando explícito no código quais regras dividem orçamento —
antes isso só existia implicitamente, no retorno repetido do mesmo campo.

## Consequências

- Os pesos do YAML voltam a significar o que dizem. Acrescentar um detector a
  uma classe redistribui aquele orçamento em vez de inflar o score.
- Um serviço que dispara duas regras da mesma classe pontua menos do que
  pontuava antes desta mudança. Foi verificado na matriz E2E: no cenário
  `retry-storm`, o serviço de pagamento soma `0.29` em contribuições e recebe
  `0.19` de score, com o excedente da classe estrutural cortado.
- O teto pode esconder acúmulo legítimo: um serviço com quatro evidências
  estruturais fortes fica indistinguível, em score, de um com uma só. As quatro
  continuam listadas na explicação, e a leitura das contribuições é o que
  diferencia os dois casos.
- O peso `log_correlation`, de `0.10`, permanece configurado e **não é usado por
  nenhuma regra**, porque logs ainda não são ingeridos. A soma dos pesos
  efetivamente aplicados é `0.90`. Isso não é corrigido aqui: mexer no YAML
  padrão exigiria migração, e o campo passa a fazer sentido quando a ingestão de
  logs existir.

## Adendo — teto relativo para evidência de apoio

O teto por classe limita quanto cada classe soma. Ele não impede que a classe de
mudança — `deployment_proximity`, `version_regression`, `schema_change_proximity`
— supere as evidências medidas do mesmo suspeito.

Medindo contra a demo-shop, uma latência que regrediu de 4 ms para 12 ms valia
0.07 e a proximidade de migração valia 0.20 sozinha. Proximidade não mede nada do
serviço: ela constata que algo mudou por perto. A contribuição da classe de
mudança passa a ser limitada à soma das evidências medidas do mesmo suspeito.

Sem sintoma algum o apoio vira zero, e o serviço deixa de ser suspeito. O commit
fica de fora do teto: ele só tem evidência de mudança por construção, e aplicá-lo
ali apagaria a acusação do commit implantado.
