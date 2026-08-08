# Changelog

## v0.1.1 — 2026-08-07

Release de correção. A v0.1.0 não enxergava aplicações instrumentadas
automaticamente — que provavelmente são a maioria das aplicações reais.
**Recomendamos atualizar; a v0.1.0 não deve ser usada.**

### Corrigido

- **Cegueira para a convenção HTTP anterior.** Os detectores reconheciam apenas
  `http.response.status_code`. Instrumentações automáticas amplamente usadas,
  como a do FastAPI/Python, emitem `http.status_code` — o nome anterior da
  convenção OpenTelemetry. Contra essas aplicações, todos os spans HTTP eram
  lidos como zero sinais e o Faultmap respondia "nenhuma anomalia" mesmo diante
  de falha total, sem qualquer aviso. Os renderizadores já aceitavam os dois
  nomes, então a saída parecia correta e escondia o problema. Ver ADR 0006.
- **Spans internos contados como requisições.** A instrumentação ASGI emite um
  span `http send` por requisição, repetindo o código de resposta do span
  principal. Contá-lo dobrava o denominador da taxa de erro: uma falha de 100%
  seria reportada como 50%. Spans `SPAN_KIND_INTERNAL` passam a ser ignorados.
- **Ruído de amostragem virando evidência.** `error_rate_delta` acusava
  regressão com qualquer aumento acima de zero. Num serviço com falha
  intermitente crônica, 3 falhas em 16 na baseline e 4 em 16 no incidente eram
  apresentadas como "taxa de erro aumentou de 18,75% para 25,00%", com confiança
  alta, sem que nada tivesse mudado. O aumento agora precisa superar um piso de
  2 pontos percentuais e o dobro do erro padrão da diferença. Ver ADR 0005.

### Verificação

- Verificado contra uma aplicação FastAPI real, instrumentada sem alteração de
  código: no mesmo banco e na mesma janela, a v0.1.0 responde "nenhuma anomalia"
  e esta versão identifica o p95 subindo de 1 ms para 15 ms sob concorrência.
- Novo modo difícil da demo (`run-hard-mode.sh`), com cinco cenários que
  verificam se o Faultmap **se cala** quando não há regressão: ruído crônico
  idêntico nas duas janelas, sistema saudável, tráfego irregular, fan-out
  legítimo e duas causas verdadeiras competindo.
- Matriz E2E: 6 de 6. Modo difícil: 5 de 5.

### Limitação conhecida que permanece

Os atributos de banco de dados ainda não foram exercitados fora da demo e podem
conter o mesmo tipo de desencontro de convenção. A demo prova que o produto
funciona contra telemetria que nós mesmos escrevemos; ela não prova
compatibilidade com instrumentações de terceiros.

## v0.1.0 — 2026-08-07

Primeira release do MVP: um binário único em Go que ingere telemetria
OpenTelemetry, compara a janela de um incidente com uma baseline, correlaciona
sinais a deploys, commits e operações PostgreSQL, e devolve um ranking
determinístico, explicável e auditável de suspeitos.

### Ingestão

- Receiver OTLP HTTP em `POST /v1/traces` aceitando JSON e protobuf, com gzip,
  limite de corpo, deduplicação por ID e encerramento controlado.
- Importação de traces OTLP a partir de arquivo.
- Importação de commits e deployments do GitHub, limitada a uma página por
  recurso e sem requisições N+1.

### Diagnóstico

- Detectores: `error_rate_delta`, `latency_delta`, `database_timeout`,
  `database_http_trace_correlation`, `deployment_proximity` e `retry_storm`.
- Correlação `service.version → commit → deployment → service → incident`.
- Grafo de evidências com proveniência e subgrafo por trace.
- Ranking com pesos configuráveis, contribuições auditáveis e confiança.
- Snapshots persistidos com ID determinístico: retries são idempotentes e um
  diagnóstico gravado nunca é alterado silenciosamente.

### Saídas

- Terminal, `report.json`, `report.md`, grafo Mermaid e `timeline.json`.
- Todas as saídas derivam do snapshot persistido e não recalculam a
  investigação.

### Operação

- `faultmap init`, `serve`, `ingest`, `telemetry list`, `diagnose incident`,
  `incident list/show`, `blame trace`, `export report/graph/timeline` e
  `retention apply`.
- Política de retenção configurável, aplicada por comando explícito em lotes
  limitados, preservando snapshots de diagnóstico.

### Qualidade

- `demo-shop` instrumentada com seis cenários de falha controlada.
- Matriz E2E automatizada sobre os seis cenários, com telemetria OTLP real,
  bancos isolados e mock local do GitHub. Cada cenário mede top-1, top-3, tempo
  de diagnóstico, estabilidade entre execuções, proveniência das evidências e a
  geração válida dos quatro artefatos.
- Metas atingidas na demo controlada: top-1 e top-3 em 100% dos cenários,
  diagnóstico mais lento em 247 ms (meta: menos de 10 segundos) e 100% das
  evidências com proveniência.

### Limitações conhecidas

- O receiver OTLP não oferece autenticação nem TLS; ele deve permanecer em
  `127.0.0.1` ou atrás de um proxy autenticado, nunca exposto à internet.
- Logs e métricas ainda não são recebidos: apenas traces.
- Arquivos por commit e status de deployment não são coletados, para evitar
  N+1 na API do GitHub. `deployment_proximity` declara essa limitação.
- Findings não possuem instante próprio; no `timeline.json` eles são ancorados
  ao início da janela do incidente, com a origem declarada em `time_source`.
- A retenção remove telemetria mas preserva snapshots, então evidências antigas
  continuam legíveis sem serem navegáveis por `blame trace`.
- O ranking indica prioridade de investigação. Ele nunca afirma causalidade.
