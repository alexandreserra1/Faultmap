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
| [0007](0007-telemetria-real-como-base-de-teste.md) | Telemetria de instrumentação real como base de teste |
| [0008](0008-politica-de-privacidade-aplicada-na-ingestao.md) | Política de privacidade aplicada na ingestão |
| [0009](0009-investigacao-compara-servicos-por-escopo-de-traces.md) | A investigação compara serviços descobertos pelos traces |
| [0010](0010-teto-por-classe-de-peso-no-ranking.md) | Teto por classe de peso no ranking |
| [0011](0011-logs-guardam-apenas-metadados.md) | Logs entram sem o texto da mensagem |
| [0012](0012-bloqueios-de-privacidade-somam-em-vez-de-substituir.md) | A lista de bloqueios do YAML soma aos padrões |
| [0013](0013-evidencia-diz-o-que-o-padrao-costuma-significar.md) | Cada evidência diz o que aquele padrão costuma significar |
| [0014](0014-schema-guarda-identificadores-nao-expressoes.md) | A coleta de schema guarda identificadores, não expressões |
| [0015](0015-retencao-libera-catalogo-e-preserva-mudancas.md) | A retenção libera o catálogo e preserva as mudanças |
| [0016](0016-postgres-como-backend-alternativo.md) | PostgreSQL é backend alternativo, provado por uma bateria compartilhada |
| [0017](0017-cauda-de-banco-e-pergunta-propria-nao-outro-percentil.md) | A cauda do banco é uma pergunta própria, não outro percentil |
| [0019](0019-mudanca-dentro-do-incidente-nao-e-acusada.md) | Mudança de catálogo dentro do incidente não é acusada |
