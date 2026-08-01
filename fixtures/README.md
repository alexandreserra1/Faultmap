# Fixtures OpenTelemetry

Os arquivos em `otel/` são payloads JSON no formato OTLP `resourceSpans`. Eles
existem para exercitar a ingestão de traces sem depender do Demo Shop ou de um
Collector em execução.

| Arquivo | Intenção | Sinais esperados |
| --- | --- | --- |
| `checkout-normal.json` | Requisição de checkout concluída normalmente. | Serviço `checkout-service`, HTTP `POST /checkout` com resposta `201`, e operação PostgreSQL `INSERT` bem-sucedida. |
| `checkout-error-latency.json` | Requisição de checkout lenta que falha durante a persistência. | Serviço `checkout-service`, resposta HTTP `500`, spans com status de erro e operação PostgreSQL `INSERT` lenta com timeout. |
| `checkout-baseline-sample.json` | Janela representativa de operação normal entre `2025-12-01T10:00:00Z` e `10:01:00Z`. | 20 traces e 40 spans: 20 respostas HTTP `201`, nenhuma falha PostgreSQL, latência HTTP entre 100 e 160 ms e p95 de 160 ms. |
| `checkout-incident-sample.json` | Janela representativa de incidente entre `2025-12-01T10:01:00Z` e `10:02:00Z`. | 20 traces e 40 spans: 8 respostas HTTP `500` (40%), 6 timeouts PostgreSQL (30%), latência HTTP entre 800 e 2.500 ms e p95 de 2.500 ms. Os 6 timeouts compartilham o `trace_id` com impacto HTTP. |

Os identificadores e os tempos são fixos para que os testes possam verificar
deduplicação, duração, ordenação e a relação pai-filho dentro de cada trace.

As fixtures de amostra usam um span HTTP `POST /checkout` e seu span filho
PostgreSQL `INSERT orders` em cada trace. Elas foram dimensionadas para que os
detectores comparem baseline e incidente com confiança alta, mantendo resultados
determinísticos para taxa de erro, timeout e p95.
