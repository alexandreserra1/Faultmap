# Operação, qualidade e escopo

Este documento é parte obrigatória da especificação do MVP. Leia também o [índice normativo](../../FAULTMAP_MVP.md) e todos os documentos que ele referencia.

## Sistema de demonstração

Criar `examples/demo-shop/` com `load-generator`, `checkout-service`, `payment-service`, PostgreSQL e `otel-collector`. O fluxo é `POST /checkout → checkout-service → payment-service → PostgreSQL`.

Cenários obrigatórios: banco lento, pool de conexões reduzido, payment retornando HTTP 500, retry excessivo, alteração de timeout após deploy e lock na tabela de pagamentos. A infraestrutura e o schema inicial são compartilhados pelo Compose base; cada cenário inclui README, override Compose, gerador de carga limitado, resultado esperado e contrato de aceitação. A telemetria deve ser produzida pela instrumentação OTLP real; fixtures estáticas não substituem a aceitação E2E da demo.

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

O receiver OTLP do MVP não oferece autenticação ou TLS. Em desenvolvimento, os listeners devem ser alterados para `127.0.0.1`; em uma rede compartilhada, o processo deve permanecer em segmento privado, protegido por proxy ou gateway com TLS e autenticação. As portas OTLP e health não devem ser publicadas diretamente na internet. Mensagens de erro HTTP não podem incluir o payload recebido, SQL, atributos, caminhos internos ou causas do banco.

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

O modo servidor expõe `GET /health` em listener separado; a resposta confirma somente que o processo HTTP está vivo. Expor futuramente logs estruturados e métricas internas: sinais recebidos, filas internas, eventos descartados, tempo de normalização/diagnóstico, erros de persistência e tamanho do banco. Sondas distintas `/health/live` e `/health/ready`, além de `/metrics`, permanecem evoluções posteriores.

## Configuração inicial

```yaml
server:
  otlp_http_address: "0.0.0.0:4318"
  health_address: "0.0.0.0:8081"
  max_request_body_bytes: 67108864
  read_header_timeout: "5s"
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "60s"
  shutdown_timeout: "10s"
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

`max_request_body_bytes` limita cada lote recebido e também o resultado da descompactação gzip. Os timeouts protegem, respectivamente, cabeçalhos lentos, leitura e escrita completas, conexões persistentes ociosas e a drenagem no encerramento. Todos devem ser positivos; os listeners devem usar `host:porta`, portas válidas e endereços diferentes. Como `Load` aplica o YAML sobre os defaults, workspaces antigos que ainda não possuem esses campos recebem os valores seguros acima sem migração manual.

## Testes obrigatórios

Unitários: cálculo de janelas, normalização OTLP, cada detector, agregação de score/confiança, relações do grafo e renderização de relatórios.

Integração: OTLP HTTP, SQLite, GitHub mockado, migrations e CLI.

Aceitação: todos os cenários demo-shop, top 1/top 3 esperados, relatório gerado, grafo válido e resultado estável.

Concorrência: executar `go test -race ./...`.

## Fora do escopo do primeiro MVP

Não implementar interface web, Kubernetes, Neo4j, Kafka, múltiplos bancos, CDC completo/logical decoding, ML, inferência causal avançada, LLM obrigatória, agentes autônomos, correção automática, RBAC, multi-tenancy, alertas em tempo real, PagerDuty, Slack, GitLab, telemetria frontend, armazenamento distribuído, ClickHouse ou SaaS. Só considerar esses itens após validar o ranking determinístico.
