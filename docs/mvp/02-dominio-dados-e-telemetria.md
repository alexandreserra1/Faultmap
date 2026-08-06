# Domínio, dados e telemetria

Este documento é parte obrigatória da especificação do MVP. Leia também o [índice normativo](../../FAULTMAP_MVP.md) e todos os documentos que ele referencia.

## Modelo de domínio inicial

```go
type Signal struct {
    ID string; Type SignalType; ServiceName string; Timestamp time.Time
    TraceID string; SpanID string; Severity string
    Attributes map[string]string; Measurements map[string]float64
}
type Incident struct {
    ID string; ServiceName string; Environment string
    StartedAt time.Time; EndedAt *time.Time; Status IncidentStatus
}
type TimeWindow struct { Start time.Time; End time.Time }
type InvestigationWindow struct { Baseline TimeWindow; Incident TimeWindow }
type Finding struct {
    ID string; RuleID string; SubjectID string; Score float64; Confidence Confidence
    Evidence []Evidence; Limitations []string
}
type Evidence struct {
    ID string; Type EvidenceType; Description string; Source EvidenceSource
    Timestamp time.Time; Attributes map[string]string
}
type EvidenceNode struct { ID string; Type NodeType; Label string; Attributes map[string]string }
type EvidenceEdge struct {
    From string; To string; Relation RelationType; Confidence float64; EvidenceIDs []string
}
type Suspect struct {
    ID string; Type SuspectType; Label string; Score float64; Confidence Confidence
    Contributions []ScoreContribution; Evidence []Evidence; Limitations []string
}
type ScoreContribution struct { RuleID string; Value float64; Reason string }
```

## Grafo de evidências

Tipos iniciais de nó: `incident`, `service`, `endpoint`, `trace`, `span`, `database`, `database_operation`, `deployment`, `commit`, `log_cluster`, `metric_anomaly` e `finding`.

Tipos futuros, fora do MVP inicial: `table`, `schema`, `migration`, `frontend_route`, `browser_session`, `user_action`, `queue`, `topic`, `consumer` e `producer`.

Relações iniciais: `calls`, `queries`, `belongs_to`, `depends_on`, `affected`, `deployed_as`, `contains`, `changed_by`, `introduced_after`, `correlated_with`, `failed_before`, `failed_after`, `supports` e `contradicts`.

Toda aresta deve conter origem, destino, relação, confiança, evidências e proveniência.

A primeira fatia funcional do grafo reconstrói um trace sob demanda, com nós de serviço, trace e span. Relações `contains` preservam a hierarquia e `queries` liga o span chamador à operação de banco. O parentesco usa `span.parent_id`, normalizado de `parentSpanId`; o fallback sem parentesco só é permitido quando há exatamente um span HTTP e um span de banco, evitando associações ambíguas.

## Persistência SQLite

Todos os acessos ocorrem por repositórios. O esquema inicial contém:

```sql
CREATE TABLE signals (
  id TEXT PRIMARY KEY, signal_type TEXT NOT NULL, service_name TEXT,
  timestamp DATETIME NOT NULL, trace_id TEXT, span_id TEXT, severity TEXT,
  attributes_json TEXT NOT NULL, measurements_json TEXT NOT NULL
);
CREATE TABLE incidents (
  id TEXT PRIMARY KEY, service_name TEXT NOT NULL, environment TEXT,
  started_at DATETIME NOT NULL, ended_at DATETIME, status TEXT NOT NULL
);
CREATE TABLE findings (
  id TEXT PRIMARY KEY, incident_id TEXT NOT NULL, rule_id TEXT NOT NULL,
  subject_id TEXT NOT NULL, score REAL NOT NULL, confidence TEXT NOT NULL,
  evidence_json TEXT NOT NULL, limitations_json TEXT NOT NULL
);
CREATE TABLE evidence_nodes (
  id TEXT PRIMARY KEY, node_type TEXT NOT NULL, label TEXT NOT NULL,
  attributes_json TEXT NOT NULL
);
CREATE TABLE evidence_edges (
  id TEXT PRIMARY KEY, source_id TEXT NOT NULL, target_id TEXT NOT NULL,
  relation TEXT NOT NULL, confidence REAL NOT NULL, evidence_ids_json TEXT NOT NULL
);
CREATE TABLE deployments (
  id TEXT PRIMARY KEY, repository TEXT NOT NULL, environment TEXT,
  service_name TEXT, commit_sha TEXT, deployed_at DATETIME NOT NULL,
  metadata_json TEXT NOT NULL
);
CREATE TABLE commits (
  sha TEXT PRIMARY KEY, repository TEXT NOT NULL, author TEXT, message TEXT,
  committed_at DATETIME NOT NULL, files_json TEXT NOT NULL
);
CREATE TABLE ranking_results (
  id TEXT PRIMARY KEY, incident_id TEXT NOT NULL, generated_at DATETIME NOT NULL,
  suspects_json TEXT NOT NULL
);
```

O diagnóstico persistido é um snapshot imutável. Seu ID é derivado do serviço e das janelas UTC; findings recebem IDs estáveis e o ranking usa `ranking:<incident_id>`. Incidente, findings e ranking são gravados na mesma transação curta. Retries não substituem o snapshot original. `findings.incident_id` e `ranking_results.incident_id` possuem chaves estrangeiras para impedir registros órfãos.

As consultas de histórico leem esse snapshot sem recalcular detectores, scores ou ranking a partir da telemetria atual. A listagem seleciona somente os campos de resumo, possui limite obrigatório entre 1 e 1.000 e usa ordenação estável por início decrescente e ID crescente. A primeira versão retorna apenas a página limitada mais recente, sem cursor ou `offset`; a consulta individual carrega no máximo 1.000 findings e o ranking do incidente em uma transação curta de leitura.

As colunas adicionadas para início/fim da baseline e contagens de sinais permanecem anuláveis para preservar bancos criados por versões anteriores. Ao ler um snapshot legado sem esses metadados, a aplicação representa a ausência explicitamente, mantém a janela conhecida do incidente e devolve os findings e o ranking existentes. Camadas superiores não devem interpretar campos ausentes como zero nem disparar um novo diagnóstico implicitamente.

## Ingestão OTLP

O Faultmap não substitui o OpenTelemetry Collector:

```text
Aplicações → OTLP → OpenTelemetry Collector → OTLP → Faultmap
```

```bash
faultmap serve --config ./faultmap-local/faultmap.yaml
faultmap ingest file --config ./faultmap-local/faultmap.yaml --input ./fixtures/otel-sample.json
```

O receiver expõe `POST /v1/traces` no listener OTLP e aceita os formatos normativos `application/json` e `application/x-protobuf`. O formato da resposta acompanha o da requisição: sucesso é um `ExportTraceServiceResponse` vazio (`{}` em JSON ou zero bytes em protobuf). Métodos diferentes de `POST`, tipo de mídia desconhecido e lote acima do limite são rejeitados antes de iniciar a persistência. Um `ExportTraceServiceRequest` vazio é válido e retorna sucesso sem iniciar persistência. Erros externos usam mensagens estáveis e não expõem detalhes do banco.

A ingestão valida o payload; extrai resource attributes, `service.name`, `service.version`, ambiente, trace/span IDs, status, duração, atributos HTTP e de banco; normaliza e persiste; e ignora duplicidades pelo ID. Tanto arquivo quanto HTTP reutilizam o mesmo caso de uso e o mesmo normalizador para evitar contratos divergentes. No modo servidor, o contexto da requisição chega à persistência e o processo mantém um único pool SQLite até o encerramento controlado.

O health check usa um listener separado e responde a `GET /health` com `{"status":"ok"}`. Nesta primeira fatia ele indica que o processo HTTP está vivo; não deve ser interpretado como verificação profunda de prontidão do banco.

A normalização de spans propaga para cada sinal somente os atributos de Resource necessários à identidade operacional: `service.version`, `service.instance.id` e `deployment.environment.name`. A allowlist evita duplicar atributos arbitrários do Resource e permite relacionar a versão observada ao commit de um deployment.

Consultas por trace usam um índice composto em `(trace_id, timestamp, id)`, limite obrigatório e ordenação determinística. O grafo é derivado dos sinais persistidos por uma única consulta; nenhuma leitura N+1 é necessária.

## Atributos OpenTelemetry prioritários

| Contexto | Atributos |
| --- | --- |
| Serviço | `service.name`, `service.version`, `service.instance.id`, `deployment.environment.name` |
| HTTP | `http.request.method`, `http.response.status_code`, `http.route`, `url.path`, `server.address` |
| Código | `code.function.name`, `code.file.path`, `code.line.number` |
| Banco | `db.system`, `db.namespace`, `db.operation.name`, `db.query.summary`, `db.response.status_code`, `server.address` |
| Erro | `error.type`, `error.message`, `exception.type`, `exception.message` |

## Janelas de investigação

Cada incidente contém a janela **Incident** (onde a regressão ocorreu) e a **Baseline** (período anterior). Para:

```bash
faultmap diagnose incident --service checkout --since 30m --baseline 60m
```

A janela Incident vai de agora menos 30 minutos até agora; Baseline, de 90 a 30 minutos atrás.

A comparação deve calcular error rate, p50/p95/p99, volume, número de spans e spans com erro, chamadas a dependências, operações de banco, distribuição de status e versões observadas.
