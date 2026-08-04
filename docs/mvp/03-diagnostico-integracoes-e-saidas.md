# Diagnóstico, integrações e saídas

Este documento é parte obrigatória da especificação do MVP. Leia também o [índice normativo](../../FAULTMAP_MVP.md) e todos os documentos que ele referencia.

## Detectores iniciais

- `error_rate_delta`: detecta aumento relevante da taxa de erros a partir das taxas baseline/incidente e volume mínimo. Produz finding, score, valores comparados e limitação quando o volume é insuficiente.
- `latency_delta`: detecta aumento de p95 e p99; não produz finding com volume insuficiente.
- `deployment_proximity`: eleva o score de serviços com deploy próximo ao incidente, versão modificada ou commits que tocaram arquivos relacionados. Nunca infere causalidade apenas pela proximidade.
- `database_timeout`: identifica spans de banco com timeout, duração acima do limite, erros de contexto/status e aumento frente à baseline.
- `database_http_trace_correlation`: relaciona, como hipótese, timeout PostgreSQL a erro HTTP 5xx ou latência HTTP acima do p95 da baseline quando os sinais compartilham o mesmo `trace_id`. Conta traces distintos, ignora identificadores vazios e nunca apresenta correlação como prova causal.
- `database_error`: identifica crescimento de falhas de conexão, erros de query, respostas inválidas e transações abortadas.
- `retry_storm`: identifica repetição anormal de chamadas semelhantes no mesmo trace ou janela.
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

Na implementação inicial, `error_rate_delta` usa `error_rate_delta`, `latency_delta` usa `latency_delta`, `database_timeout` usa `database_evidence` e `database_http_trace_correlation` usa `graph_proximity`, pois representa a ligação entre sinais no mesmo trace. Cada contribuição é calculada como `score do finding × peso`. O total indica prioridade de investigação e não deve ser descrito como probabilidade causal. Pesos do YAML ficam entre 0 e 1, com soma maior que zero e no máximo 1.

## Integração GitHub

```bash
faultmap ingest github --repo acme/checkout --deployments --commits --environment staging
```

Coletar commits, SHA, autor, mensagem, arquivos modificados, horário, deployments, statuses, ambiente, versão e repositório. Relacionar `service.version → commit → deployment → service → incident`. A autenticação usa exclusivamente `GITHUB_TOKEN`, que nunca pode ser registrado em logs.

## Integração PostgreSQL

Na primeira fase, usar spans OpenTelemetry para detectar operações lentas, timeout, erros, excesso de queries, possível N+1 e banco/operação envolvidos. Uma fase posterior poderá usar `pg_stat_activity`, `pg_stat_statements`, `pg_locks` e `pg_stat_database`. CDC e logical decoding não pertencem à primeira entrega funcional.

## CLI

```bash
faultmap init
faultmap serve
faultmap ingest file --input ./fixtures/otel-sample.json
faultmap ingest github --repo acme/checkout --deployments --commits
faultmap diagnose incident --service checkout --since 30m --baseline 60m --environment staging
faultmap blame trace 7f3b0b3f1b4d
faultmap explain suspect checkout-service
faultmap export report --incident inc_001 --format markdown
faultmap export graph --incident inc_001 --format mermaid
```

`blame trace` consulta somente o `trace_id` solicitado, com limite obrigatório, e apresenta o fluxo cronológico e suas relações usando uma allowlist de campos. SQL bruto, credenciais e atributos arbitrários não são exibidos.

`diagnose incident` persiste o resultado depois de concluir as duas leituras limitadas e o cálculo determinístico. A escrita usa o mesmo pool do comando e uma única transação para incidente, findings e ranking. O ID determinístico torna retries idempotentes; um snapshot existente não é atualizado silenciosamente.

Enquanto incidentes persistidos ainda não estiverem disponíveis, `export graph --trace <id> --format mermaid` exporta o subgrafo de um trace para a saída padrão. A serialização é determinística, usa IDs sintéticos, escapa rótulos externos e rejeita arestas que apontem para nós ausentes.

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
