# Diagnóstico, integrações e saídas

Este documento é parte obrigatória da especificação do MVP. Leia também o [índice normativo](../../FAULTMAP_MVP.md) e todos os documentos que ele referencia.

## Detectores iniciais

- `error_rate_delta`: detecta aumento relevante da taxa de erros a partir das taxas baseline/incidente e volume mínimo. Produz finding, score, valores comparados e limitação quando o volume é insuficiente.
- `latency_delta`: detecta aumento de p95 e p99; não produz finding com volume insuficiente.
- `deployment_proximity`: eleva o score de serviços com deploy próximo ao incidente, versão modificada ou commits que tocaram arquivos relacionados. Nunca infere causalidade apenas pela proximidade.
- `database_timeout`: identifica spans de banco com timeout, duração acima do limite, erros de contexto/status e aumento frente à baseline.
- `database_http_trace_correlation`: relaciona, como hipótese, timeout PostgreSQL a erro HTTP 5xx ou latência HTTP acima do p95 da baseline quando os sinais compartilham o mesmo `trace_id`. Conta traces distintos, ignora identificadores vazios e nunca apresenta correlação como prova causal.
- `database_error`: identifica crescimento de falhas de conexão, erros de query, respostas inválidas e transações abortadas.
- `retry_storm`: identifica repetição anormal da mesma operação dentro de traces comparáveis entre baseline e incidente. O finding explica as tentativas por trace observadas nas duas janelas e sempre ressalva que spans repetidos podem representar fan-out, paginação ou instrumentação duplicada, não necessariamente uma política de retry.
- `dependency_failure`: identifica downstream que falha antes de upstream, traces conectando os serviços e aumento correlacionado de erros.
- `trace_break`: identifica ausência de propagação, traces interrompidos, chamadas conhecidas sem ligação entre spans ou perda de contexto entre serviços.
- `version_regression`: compara duas versões do mesmo serviço.

## Ranking

Pesos configuráveis iniciais:

```yaml
ranking:
  weights:
    error_rate_delta: 0.25
    deployment_proximity: 0.20
    database_evidence: 0.20
    graph_proximity: 0.15
    latency_delta: 0.10
    log_correlation: 0.10
```

O score final fica entre 0 e 1; toda contribuição é armazenada; os pesos são configuráveis; findings com pouco volume têm peso reduzido; evidências contraditórias diminuem confiança; ausência de dados vira limitação, não conclusão inventada. Retornar top 3 por padrão, com limite configurável pelo usuário.

Na implementação inicial, `error_rate_delta` usa `error_rate_delta`, `latency_delta` usa `latency_delta`, `database_timeout` usa `database_evidence`, e tanto `database_http_trace_correlation` quanto `retry_storm` usam `graph_proximity`. A correlação de banco representa a ligação entre sinais no mesmo trace; o retry storm representa repetição estrutural da mesma operação dentro do trace. Esse mapeamento reutiliza deliberadamente o peso existente e não adiciona outro campo ao YAML, preservando o contrato e a soma configurada dos pesos.

Cada finding contribui individualmente com `score do finding × peso da classe`. Por exemplo, um `retry_storm` com score `0.80` e `graph_proximity: 0.15` contribui `0.12`. Se houver mais de um finding da classe estrutural, cada contribuição permanece visível e auditável; o total agregado continua limitado ao intervalo de 0 a 1. Esse total indica prioridade de investigação e não deve ser descrito como probabilidade causal. Pesos do YAML ficam entre 0 e 1, com soma maior que zero e no máximo 1.

## Integração GitHub

```bash
faultmap ingest github --repo acme/checkout --deployments --commits --environment staging
```

Coletar commits, SHA, autor, mensagem, arquivos modificados, horário, deployments, statuses, ambiente, versão e repositório. Relacionar `service.version → commit → deployment → service → incident`. A autenticação usa exclusivamente `GITHUB_TOKEN`, que nunca pode ser registrado em logs.

A primeira fatia operacional limita cada execução a uma página de até 100 commits e uma página de até 100 deployments, persiste ambos atomicamente e é idempotente. Como os endpoints de listagem não incluem arquivos por commit nem o status atual de cada deployment, esses dois detalhes permanecem explicitamente vazios/desconhecidos nesta etapa; não é permitido buscá-los com uma requisição N+1. Uma evolução deve usar uma operação em lote ou justificar uma estratégia limitada e medida.

O detector `deployment_proximity` consulta somente deployments persistidos do mesmo serviço e ambiente, em uma janela de até uma hora antes do início do incidente. Seu score decai linearmente com a distância temporal. A correspondência entre `commit_sha` e `service.version` eleva a confiança e a mudança de versões entre baseline/incidente é registrada na evidência. O ID do deployment é preservado como proveniência, separado dos IDs de sinais, e a limitação causal é obrigatória.

## Integração PostgreSQL

Na primeira fase, usar spans OpenTelemetry para detectar operações lentas, timeout, erros, excesso de queries, possível N+1 e banco/operação envolvidos. Uma fase posterior poderá usar `pg_stat_activity`, `pg_stat_statements`, `pg_locks` e `pg_stat_database`. CDC e logical decoding não pertencem à primeira entrega funcional.

## CLI

```bash
faultmap init
faultmap serve
faultmap ingest file --input ./fixtures/otel-sample.json
faultmap ingest github --repo acme/checkout --deployments --commits
faultmap diagnose incident --service checkout --since 30m --baseline 60m --environment staging
faultmap incident list --limit 20
faultmap incident show --id inc_001
faultmap blame trace 7f3b0b3f1b4d
faultmap explain suspect checkout-service
faultmap export report --incident inc_001 --format markdown
faultmap export graph --incident inc_001 --format mermaid
```

`blame trace` consulta somente o `trace_id` solicitado, com limite obrigatório, e apresenta o fluxo cronológico e suas relações usando uma allowlist de campos. SQL bruto, credenciais e atributos arbitrários não são exibidos.

`diagnose incident` persiste o resultado depois de concluir as duas leituras limitadas e o cálculo determinístico. A escrita usa o mesmo pool do comando e uma única transação para incidente, findings e ranking. O ID determinístico torna retries idempotentes; um snapshot existente não é atualizado silenciosamente. Quando a janela do incidente não contém sinais, nenhuma transação é iniciada e o resultado não é persistido, evitando congelar uma investigação executada antes da chegada da telemetria.

`incident list` consulta somente resumos persistidos e não recalcula diagnósticos. A leitura exige limite entre 1 e 1.000, seleciona apenas ID, serviço, status e janela, e ordena deterministicamente por início do incidente decrescente e ID crescente. A versão atual implementa uma página limitada dos registros mais recentes, sem cursor nem `offset`; paginação por cursor deverá ser adicionada quando o volume justificar essa evolução.

`incident show` recupera por ID o snapshot gravado, incluindo metadados das janelas, contagens, findings e ranking, sem reler sinais ou executar detectores. A consistência entre essas partes é protegida por uma transação curta de leitura, e a quantidade de findings é limitada a 1.000. Incidentes legados cujas colunas de baseline e contagens sejam nulas continuam legíveis: a saída declara os metadados ausentes, preserva a janela do incidente e apresenta os findings e o ranking disponíveis sem fabricar zeros.

`export report --incident <id> --format json|markdown` recupera o mesmo snapshot persistido usado por `incident show` e escreve o relatório na saída padrão. O formato JSON possui contrato explícito com `schema_version`, timestamps UTC em RFC 3339, scores com a precisão armazenada e `baseline: null` para snapshots legados. O Markdown prioriza leitura humana, apresenta scores com duas casas e declara metadados legados ausentes. Ambos possuem ordenação determinística e não recalculam a investigação.

`export graph --trace <id> --format mermaid` exporta o subgrafo de um trace para a saída padrão. A serialização é determinística, usa IDs sintéticos, escapa rótulos externos e rejeita arestas que apontem para nós ausentes.

`faultmap init` cria `faultmap.yaml`, `faultmap.db` e `faultmap-out/`.

## Artefatos gerados

```text
faultmap-out/
├── report.md
├── ranking.json
├── evidence-graph.mmd
├── incident-summary.json
└── timeline.json
```

`ranking.json` deve conter `incident_id`, `generated_at` e uma lista de suspeitos. Cada suspeito inclui ID, rótulo, score, confiança, contribuições (`rule_id`, valor e motivo) e limitações. Exemplo: `checkout-service`, score `0.91`, contribuições de aumento de erros e deploy seis minutos antes, e a limitação de que proximidade não prova causalidade.
