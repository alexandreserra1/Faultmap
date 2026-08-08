# Telemetria real de instrumentação de terceiros

As fixtures deste diretório **não foram escritas por nós**. Cada arquivo é a
saída OTLP JSON de uma biblioteca oficial de instrumentação do OpenTelemetry,
capturada por um OpenTelemetry Collector a partir de execuções verdadeiras, com
consultas e falhas verdadeiras.

Essa distinção é a razão de o diretório existir. As fixtures em `fixtures/otel/`
foram escritas por nós e usam os mesmos nomes de atributo que o código sob teste
espera — elas provam que o Faultmap funciona contra si mesmo. Uma release chegou
a ser publicada completamente cega para aplicações reais sem que nenhum teste
falhasse, porque quem escolhe os nomes dos atributos é a biblioteca de
instrumentação, não a aplicação.

| Arquivo | Origem | Biblioteca |
| --- | --- | --- |
| `postgres-psycopg2.json` | PostgreSQL 17.10 real, com timeout imposto por `statement_timeout` | `opentelemetry-instrumentation-psycopg2` 0.62b1 |
| `sqlite3.json` | SQLite real, com erro de tabela inexistente | `opentelemetry-instrumentation-sqlite3` 0.62b1 |
| `fastapi-strideredge.json` | Aplicação FastAPI de terceiros, instrumentada sem alteração de código | `opentelemetry-instrumentation-fastapi` 0.62b1 |

## O que elas contêm que a nossa demo não produz

- `db.system` em vez de `db.system.name`, e `http.status_code` em vez de
  `http.response.status_code` — a convenção anterior, ainda emitida pelas
  instrumentações mais usadas.
- Nenhum atributo de operação: a instrumentação DBAPI não emite `db.operation`.
- Falhas sinalizadas por status do span e por evento `exception`, não por um
  atributo `error.type`.
- Spans internos auxiliares (`http send`) que repetem o código de resposta do
  span principal.

## Única alteração feita na captura

O valor de `exception.stacktrace` foi substituído por `<truncado na captura>`,
porque carregava caminhos absolutos da máquina de captura e não acrescenta nada
ao teste. Chaves, estrutura e todos os demais valores estão intactos.

## Como recapturar

O procedimento está descrito em `docs/adr/0007-telemetria-real-como-base-de-teste.md`.
