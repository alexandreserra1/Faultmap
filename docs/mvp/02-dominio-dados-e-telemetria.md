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

## Ingestão OTLP

O Faultmap não substitui o OpenTelemetry Collector:

```text
Aplicações → OTLP → OpenTelemetry Collector → OTLP → Faultmap
```

```bash
faultmap serve --otlp-http-listen 0.0.0.0:4318 --database ./faultmap.db
faultmap ingest file --input ./fixtures/otel-sample.json
```

A ingestão deve validar o payload; extrair resource attributes, `service.name`, `service.version`, ambiente, trace/span IDs, status, duração, atributos HTTP e de banco; normalizar e persistir; e ignorar duplicidades pelo ID.

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
