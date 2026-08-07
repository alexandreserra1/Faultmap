# Decisões arquiteturais

Registro das decisões cujo motivo não é dedutível do código. Cada arquivo
descreve o contexto, a decisão e as consequências — inclusive as ruins.

| ADR | Assunto |
| --- | --- |
| [0001](0001-ranking-reutiliza-peso-graph-proximity.md) | Detectores estruturais reutilizam o peso `graph_proximity` |
| [0002](0002-ingestao-github-sem-n-mais-1.md) | Ingestão do GitHub limitada a uma página, sem N+1 |
| [0003](0003-retencao-preserva-snapshots-de-diagnostico.md) | Retenção apaga telemetria e preserva snapshots |
| [0004](0004-timeline-ancora-findings-na-janela-do-incidente.md) | `timeline.json` ancora findings na janela do incidente |
| [0005](0005-error-rate-delta-ignora-ruido-de-amostragem.md) | `error_rate_delta` ignora variação de amostragem |
| [0006](0006-detectores-aceitam-duas-convencoes-http.md) | Detectores aceitam as duas convenções HTTP e ignoram spans internos |
