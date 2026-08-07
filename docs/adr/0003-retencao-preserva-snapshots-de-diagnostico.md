# ADR 0003 — Retenção apaga telemetria bruta e preserva snapshots de diagnóstico

- Status: aceito
- Data: 2026-08-06

## Contexto

`storage.retention` era validado no YAML mas não produzia efeito. Ao
implementá-lo, foi preciso decidir o alcance da limpeza. O banco guarda dois
tipos muito diferentes de dado:

1. telemetria bruta (`signals`), volumosa e contínua;
2. snapshots de investigações já concluídas (`incidents`, `findings`,
   `ranking_results`), pequenos e auditáveis.

Apagar os dois faria `incident show` e `export report` falharem para qualquer
diagnóstico mais antigo que a janela de retenção — justamente os que alguém
consultaria em um postmortem.

## Decisão

A retenção alcança apenas a tabela `signals`. Snapshots são preservados
indefinidamente.

A execução é explícita (`faultmap retention apply`), nunca acionada durante a
ingestão OTLP, e avança por lotes limitados com teto de 200 lotes por execução.
Um lote incompleto encerra o trabalho; o teto impede um laço indefinido quando
telemetria nova expira durante a própria limpeza.

## Consequências

- Uma investigação publicada continua legível depois que sua telemetria expira.
- Os IDs de sinais preservados como proveniência podem apontar para telemetria
  ausente. `blame trace` sobre um trace expirado não devolverá spans; a evidência
  no snapshot permanece, mas deixa de ser navegável.
- O crescimento do banco fica dominado por `signals`, que é o alvo da limpeza;
  snapshots crescem devagar e ainda não têm política própria. Se isso mudar, uma
  retenção separada para snapshots deve ser discutida em um novo ADR.
- O comando não faz retry automático: falhar no meio deixa os lotes já
  confirmados removidos e basta executá-lo de novo para continuar.
