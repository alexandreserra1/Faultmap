# Produto e arquitetura

Este documento é parte obrigatória da especificação do MVP. Leia também o [índice normativo](../../FAULTMAP_MVP.md) e todos os documentos que ele referencia.

## Visão do produto

O **Faultmap** é uma ferramenta open source, CLI-first, para investigação determinística de incidentes e regressões em aplicações backend.

O MVP responde: dado um serviço afetado e uma janela de tempo, quais são os principais suspeitos da falha e quais evidências sustentam esse ranking?

Ele correlaciona traces, logs e métricas OpenTelemetry; serviços, endpoints e dependências; operações de banco; commits e deploys do GitHub; e mudanças entre uma janela baseline e uma janela de incidente. O núcleo não depende de LLMs; futuros LLMs/agentes apenas consomem os resultados estruturados do backend.

## Objetivo funcional

O MVP deve receber/importar telemetria OpenTelemetry, persistir sinais localmente, identificar anomalias na janela de incidente, compará-las à baseline, executar detectores determinísticos, construir um grafo de evidências, ranquear suspeitos explicavelmente, correlacionar commits/deploys e gerar saídas de terminal, JSON, Markdown e Mermaid.

Exemplo:

```bash
faultmap diagnose incident --service checkout --since 30m --baseline 60m --environment staging
```

O resultado deve apontar suspeitos, score e confiança, listar as evidências (por exemplo: aumento de erros/latência, deploy próximo, timeout PostgreSQL e commit relacionado) e declarar limitações. Proximidade temporal não é prova de causalidade.

## Princípios

### Backend determinístico

Decisões principais usam regras, cálculos estatísticos simples, comparação de janelas, análise de grafos, correlação temporal e evidências rastreáveis. Nenhuma regra de negócio depende de LLM.

### Explicabilidade

Todo score possui contribuições auditáveis, por exemplo `error_rate_delta`, `deployment_proximity`, `database_timeout`, `graph_proximity`, `latency_delta` e `log_correlation`. Nunca retornar apenas um score sem explicar sua origem.

### CLI-first e local-first

O primeiro produto não requer interface web. Funciona localmente com binário único, SQLite, Docker Compose e OpenTelemetry Collector. Saídas: terminal, JSON, Markdown e Mermaid.

### Infraestrutura existente e evolução controlada

Usar OpenTelemetry Collector/OTLP para ingestão, GitHub API para commits/deploys, PostgreSQL e suas estatísticas quando aplicável, e SQLite local. LLMs/agentes futuros podem explicar evidências, resumir, criar postmortems ou integrar MCP, mas não substituem o motor de diagnóstico.

## Arquitetura

O MVP é um **monólito modular em Go**: um repositório, binário, processo principal e banco SQLite. Módulos isolados comunicam-se por chamadas em memória e interfaces; não há HTTP entre módulos internos.

```text
CLI / OTLP entradas → Application (casos de uso) → Domain
                                                ↑
             Infrastructure (SQLite, GitHub, PostgreSQL, OTLP, reports)
```

### Linguagem e stack

Go é a linguagem operacional por viabilizar binário único, distribuição simples, concorrência, servidores/CLIs, filas/backpressure, tipagem forte, suporte a OpenTelemetry e baixo consumo. Stack inicial: Go, Cobra, SQLite, OTLP, OpenTelemetry Collector, GitHub API, PostgreSQL, Docker Compose, YAML, JSON, Markdown e Mermaid.

Python poderá servir a notebooks, experimentos e validação de algoritmos; TypeScript, à futura UI web; LLMs, à explicação/conversa/MCP. Python não é dependência operacional do MVP.

## Estrutura inicial do repositório

```text
faultmap/
├── cmd/faultmap/main.go
├── internal/
│   ├── application/        # ingest, diagnose, explain, import, export
│   ├── telemetry/          # domain, normalizer, otlp
│   ├── incidents/          # domain, service
│   ├── evidence/           # domain, graph
│   ├── detection/          # domain, detectors
│   ├── ranking/            # domain, engine
│   ├── integrations/       # github, postgres
│   ├── reporting/          # terminal, json, markdown, mermaid
│   ├── storage/sqlite/
│   └── platform/           # config, logging, migrations, lifecycle, health
├── examples/demo-shop/
├── migrations/  scenarios/  docs/  research/
├── docker-compose.yml  faultmap.example.yaml  go.mod  Makefile  README.md
```

## Regras de dependência

Permitido: `cmd → application`, `application → domain`, `infrastructure → domain` e `reporting → domain`.

Não permitido: `domain → SQLite/GitHub/Cobra`, `ranking → OTLP`, `detection → Markdown` ou `telemetry → GitHub`. Módulos trocam estruturas internas do domínio, nunca tipos de bibliotecas externas.

```go
type Detector interface {
    ID() string
    Detect(ctx context.Context, input DetectionInput) ([]Finding, error)
}
```

Não acoplar detectores a `*otlptrace.ResourceSpans`, `*sql.DB` ou `map[string]any`.

## Responsabilidades dos módulos

- `telemetry`: recebe OTLP/importa JSON, normaliza traces/logs/métricas, deduplica e produz `Signal`; não diagnostica.
- `incidents`: cria incidentes, serviço/ambiente/janelas e estado da investigação.
- `evidence`: grafo, nós, arestas, proveniência, confiança e subgrafos.
- `detection`: detectores independentes e determinísticos.
- `ranking`: normaliza findings, aplica pesos, agrega, usa proximidade do grafo, ordena e explica.
- `integrations`: GitHub, PostgreSQL e integração com o Collector.
- `reporting`: terminal, JSON, Markdown e Mermaid.
- `storage`: migrations, repositórios/transações e persistência de sinais, incidentes, findings, grafo, deploys e commits.
- `platform`: configuração, logs, ciclo de vida, shutdown, health, IDs, relógio e migrations.

Evitar um pacote genérico `utils`.
