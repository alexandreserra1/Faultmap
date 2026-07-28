# Operação, qualidade e escopo

Este documento é parte obrigatória da especificação do MVP. Leia também o [índice normativo](../../FAULTMAP_MVP.md) e todos os documentos que ele referencia.

## Sistema de demonstração

Criar `examples/demo-shop/` com `load-generator`, `checkout-service`, `payment-service`, PostgreSQL e `otel-collector`. O fluxo é `POST /checkout → checkout-service → payment-service → PostgreSQL`.

Cenários obrigatórios: banco lento, pool de conexões reduzido, payment retornando HTTP 500, retry excessivo, alteração de timeout após deploy e lock na tabela de pedidos. Cada cenário inclui README, script de inicialização, script de execução, resultado esperado, fixture de telemetria e teste de aceitação.

## Critérios de aceite

O MVP é funcional quando:

- gera binários para macOS/Linux, sem Python, cria SQLite automaticamente, aceita YAML válido e sobe a demo via Docker Compose;
- recebe traces OTLP e fixtures, persiste/deduplica sinais e consulta por serviço/janela;
- cria baseline/incidente, executa no mínimo cinco detectores, constrói o grafo, retorna top 3, explica cada contribuição e registra limitações;
- importa commits/deploys, relaciona `service.version` ao commit e usa proximidade temporal no ranking;
- identifica operações PostgreSQL e aumentos de timeout, latência e quantidade de queries;
- imprime no terminal e gera JSON, Markdown e Mermaid;
- possui testes unitários de detectores/ranking, repositórios SQLite, integração OTLP e aceitação da demo, sem race conditions em `go test -race ./...`.

## Métricas de sucesso

Medir top-1/top-3 accuracy, tempo de diagnóstico, estabilidade entre execuções, falsos positivos, evidências sem proveniência, tempo de instalação ao primeiro diagnóstico e passos manuais. Na demo controlada: top-3 ≥ 80%, top-1 ≥ 60%, diagnóstico local < 10 segundos e 100% das evidências com proveniência. Essas metas não se aplicam diretamente à produção real.

## Segurança e privacidade

Nunca registrar tokens; mascarar credenciais; evitar corpos completos de requisição; não armazenar SQL bruto por padrão (preferir `db.query.summary`); permitir bloqueio de atributos; limitar payloads; validar entradas; suportar retenção configurável; documentar riscos de cardinalidade; executar localmente por padrão.

```yaml
privacy:
  blocked_attributes:
    - user.email
    - user.document
    - http.request.body
    - db.statement
  max_attribute_length: 512
  store_raw_logs: false
```

## Observabilidade do Faultmap

Expor logs estruturados, health check e métricas internas: sinais recebidos, filas internas, eventos descartados, tempo de normalização/diagnóstico, erros de persistência e tamanho do banco. No futuro, o modo servidor oferece `/health/live`, `/health/ready` e `/metrics`.

## Configuração inicial

```yaml
server:
  otlp_http_address: "0.0.0.0:4318"
  health_address: "0.0.0.0:8081"
storage:
  driver: "sqlite"
  path: "./faultmap.db"
  retention: "7d"
investigation:
  default_incident_window: "30m"
  default_baseline_window: "60m"
  top_suspects: 3
ranking:
  weights:
    error_rate_delta: 0.25
    deployment_proximity: 0.20
    database_evidence: 0.20
    graph_proximity: 0.15
    latency_delta: 0.10
    log_correlation: 0.10
github:
  enabled: false
  repository: ""
  environment: "staging"
privacy:
  store_raw_logs: false
  max_attribute_length: 512
  blocked_attributes: [http.request.body, db.statement]
```

## Testes obrigatórios

Unitários: cálculo de janelas, normalização OTLP, cada detector, agregação de score/confiança, relações do grafo e renderização de relatórios.

Integração: OTLP HTTP, SQLite, GitHub mockado, migrations e CLI.

Aceitação: todos os cenários demo-shop, top 1/top 3 esperados, relatório gerado, grafo válido e resultado estável.

Concorrência: executar `go test -race ./...`.

## Fora do escopo do primeiro MVP

Não implementar interface web, Kubernetes, Neo4j, Kafka, múltiplos bancos, CDC completo/logical decoding, ML, inferência causal avançada, LLM obrigatória, agentes autônomos, correção automática, RBAC, multi-tenancy, alertas em tempo real, PagerDuty, Slack, GitLab, telemetria frontend, armazenamento distribuído, ClickHouse ou SaaS. Só considerar esses itens após validar o ranking determinístico.
