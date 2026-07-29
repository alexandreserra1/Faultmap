# Fixtures OpenTelemetry

Os arquivos em `otel/` são payloads JSON no formato OTLP `resourceSpans`. Eles
existem para exercitar a ingestão de traces sem depender do Demo Shop ou de um
Collector em execução.

| Arquivo | Intenção | Sinais esperados |
| --- | --- | --- |
| `checkout-normal.json` | Requisição de checkout concluída normalmente. | Serviço `checkout-service`, HTTP `POST /checkout` com resposta `201`, e operação PostgreSQL `INSERT` bem-sucedida. |
| `checkout-error-latency.json` | Requisição de checkout lenta que falha durante a persistência. | Serviço `checkout-service`, resposta HTTP `500`, spans com status de erro e operação PostgreSQL `INSERT` lenta com timeout. |

Os identificadores e os tempos são fixos para que os testes possam verificar
deduplicação, duração, ordenação e a relação pai-filho dentro de cada trace.
