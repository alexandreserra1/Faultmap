# Faultmap

> Investigação determinística de incidentes para aplicações backend.

O Faultmap recebe telemetria, compara o comportamento normal com uma janela de incidente e aponta os principais suspeitos — sempre mostrando as evidências, o score e as limitações da hipótese.

![Arquitetura do MVP do Faultmap](docs/images/faultmap-mvp-architecture.png)

## O que estamos construindo

Um monólito modular em Go, distribuído como um único binário e orientado à CLI. A etapa atual recebe traces OpenTelemetry, correlaciona esses sinais com deploys e commits do GitHub e operações PostgreSQL, persiste o contexto localmente em SQLite e gera um ranking explicável de suspeitos. Logs e métricas fazem parte da evolução planejada, mas ainda não são recebidos pelo servidor OTLP.

Em vez de responder apenas “algo está errado”, o objetivo é responder:

> Para este serviço e esta janela de tempo, onde devemos começar a investigar e quais evidências sustentam essa prioridade?

Exemplo da experiência desejada:

```bash
faultmap diagnose incident \
  --service checkout \
  --since 30m \
  --baseline 60m \
  --environment staging
```

O resultado deve listar os suspeitos mais prováveis, suas contribuições de score e as limitações. Um deploy próximo, por exemplo, aumenta a prioridade de investigação, mas nunca é apresentado como prova de causalidade.

## Como funciona

1. Aplicações enviam traces por OTLP ao OpenTelemetry Collector.
2. O Faultmap normaliza e armazena os sinais no SQLite.
3. Ao diagnosticar um incidente, ele compara a janela atual com uma baseline anterior.
4. Detectores determinísticos encontram mudanças como aumento de erros, latência, timeout de banco, retry storm e falhas em dependências.
5. Um grafo de evidências conecta serviços, traces, deploys, commits e operações de banco.
6. O mecanismo de ranking gera os principais suspeitos e relatórios em terminal, JSON, Markdown e Mermaid.

O motor de diagnóstico não depende de LLM. Integrações futuras com LLMs, agentes ou MCP apenas consumirão e explicarão os resultados estruturados.

## Escopo do MVP

- CLI em Go e banco SQLite local;
- ingestão OTLP e importação de fixtures;
- comparação entre incident e baseline;
- detectores determinísticos e ranking auditável;
- correlação inicial com GitHub e PostgreSQL;
- ambiente `demo-shop` reproduzível com Docker Compose;
- relatórios para terminal, JSON, Markdown e Mermaid.

Interface web, Kubernetes, Neo4j, Kafka, SaaS e LLM obrigatória estão fora do primeiro MVP.

## Desenvolvimento local

### Requisitos

- Go `1.24.0` ou compatível com a versão declarada em [`go.mod`](go.mod);
- `make`, para executar os atalhos de qualidade.

Não é necessário instalar o binário globalmente para trabalhar no projeto. Os comandos abaixo usam o código local; o Go baixa as dependências declaradas em `go.mod` quando necessário.

### Verificações de qualidade

Execute estes comandos na raiz do repositório:

```bash
make fmt
make test
make test-race
make vet
```

- `make fmt` aplica a formatação padrão do Go;
- `make test` executa a suíte de testes;
- `make test-race` executa a suíte com detecção de condições de corrida;
- `make vet` executa as verificações estáticas padrão do Go.

### Criar um workspace local

Crie um diretório exclusivo para os arquivos gerados pelo Faultmap:

```bash
go run ./cmd/faultmap init --directory ./faultmap-local
```

O comando imprime `Faultmap inicializado.` e cria os seguintes artefatos dentro de `./faultmap-local`:

- `faultmap.yaml`: configuração inicial local, sem tokens ou credenciais;
- `faultmap.db`: banco SQLite com o schema inicial migrado;
- `faultmap-out/`: diretório reservado para relatórios e outras saídas futuras.

O `init` não sobrescreve artefatos existentes. Para criar novamente o mesmo workspace, remova explicitamente apenas o diretório que você escolheu para ele:

```bash
rm -rf ./faultmap-local
```

O workspace da CLI é independente da demonstração Docker descrita a seguir.

## Demo Shop

A [`demo-shop`](examples/demo-shop/README.md) executa localmente o caminho completo `checkout-service → payment-service → PostgreSQL → OpenTelemetry Collector → Faultmap`.

Suba os cinco componentes e aguarde os health checks:

```bash
make demo-up
```

Gere um checkout saudável:

```bash
curl --fail-with-body \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"manual-1","amount_cents":1990}' \
  http://127.0.0.1:18080/checkout
```

Depois de aguardar ao menos um segundo pelo batch do Collector, consulte os spans persistidos:

```bash
docker compose -f examples/demo-shop/compose.yaml exec -T faultmap \
  faultmap telemetry list \
  --config /etc/faultmap/faultmap.yaml \
  --service checkout-service \
  --since 5m \
  --limit 20
```

Os seis cenários controlados e seus diagnósticos estão documentados no [guia da demo](examples/demo-shop/README.md#cenários). Para acompanhar os processos use `make demo-logs`; para encerrá-los preservando os volumes use:

```bash
make demo-down
```

`docker compose -f examples/demo-shop/compose.yaml down --volumes` também apaga conscientemente os bancos locais da demonstração; não use `--volumes` se quiser manter o histórico.

### Receber traces por OTLP/HTTP

Inicie o receiver usando o mesmo workspace criado pelo `init`:

```bash
go run ./cmd/faultmap serve \
  --config ./faultmap-local/faultmap.yaml
```

O processo mantém dois listeners independentes: `POST /v1/traces` recebe lotes OTLP no endereço `server.otlp_http_address`, enquanto `GET /health` responde no endereço `server.health_address`. A ingestão reutiliza o mesmo normalizador e o mesmo pool SQLite durante todo o ciclo de vida do processo. Reenviar spans com os mesmos IDs é seguro: a persistência ignora duplicidades.

Envie uma fixture no formato OTLP JSON:

```bash
curl --fail-with-body \
  -H 'Content-Type: application/json' \
  --data-binary @./fixtures/otel/checkout-normal.json \
  http://127.0.0.1:4318/v1/traces
```

Uma ingestão aceita retorna `200 OK` e o `ExportTraceServiceResponse` vazio, representado como `{}` em JSON. O endpoint também aceita OTLP protobuf com `Content-Type: application/x-protobuf`; nesse caso, o corpo de sucesso é um protobuf vazio. O formato é determinado pelo `Content-Type`, e não pela extensão ou pelo conteúdo aparente do corpo.

Verifique a saúde do processo separadamente:

```bash
curl --fail http://127.0.0.1:8081/health
```

Resposta esperada:

```json
{"status":"ok"}
```

Para encaminhar traces de aplicações reais, configure um OpenTelemetry Collector. O exporter `otlp_http` acrescenta `/v1/traces` ao `endpoint` para o pipeline de traces:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

processors:
  batch:

exporters:
  otlp_http/faultmap:
    endpoint: http://faultmap:4318

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp_http/faultmap]
```

O receiver aceita corpo sem compactação ou com `Content-Encoding: gzip`. Ele limita cada requisição e também o corpo descompactado a 64 MiB por padrão, além de configurar timeouts de cabeçalho, leitura, escrita, conexão ociosa e encerramento. Payload inválido retorna `400`, método incorreto `405`, corpo acima do limite `413`, formato não suportado `415` e falha interna `500`, sem expor detalhes de persistência.

Esta primeira versão não implementa autenticação nem TLS no receiver. Em uma máquina de desenvolvimento, altere os listeners para `127.0.0.1`; em rede compartilhada, mantenha o Faultmap em uma rede privada e coloque autenticação e TLS em um proxy ou gateway confiável. Não exponha as portas diretamente à internet.

### Importar uma fixture OTLP

Depois de criar o workspace, importe uma fixture de trace OpenTelemetry:

```bash
go run ./cmd/faultmap ingest file \
  --config ./faultmap-local/faultmap.yaml \
  --input ./fixtures/otel/checkout-normal.json
```

O comando normaliza os spans do arquivo, aplica as migrations necessárias e persiste apenas sinais ainda não existentes. A fixture normal imprime:

```text
Ingeridos 2 sinais; 2 novos.
```

Executar o mesmo comando outra vez é seguro: os dois spans são identificados pelos IDs de trace e span, portanto o resultado terá `0 novos`.

### Importar commits e deployments do GitHub

Defina o token somente no ambiente e importe uma janela limitada:

```bash
export GITHUB_TOKEN="seu-token"

go run ./cmd/faultmap ingest github \
  --config ./faultmap-local/faultmap.yaml \
  --repo acme/checkout \
  --commits \
  --deployments \
  --service checkout-service \
  --environment staging \
  --since 168h \
  --limit 100
```

O token nunca é gravado no YAML, no SQLite ou nas mensagens de erro. A coleta aceita no máximo 100 itens por recurso e grava commits e deployments na mesma transação curta e idempotente. Repetir a janela não cria duplicidades.

Esta primeira fatia usa uma única chamada REST para commits e outra para deployments. Por isso, ainda não importa a lista de arquivos de cada commit nem o status individual de cada deployment: esses detalhes exigiriam uma requisição adicional por item e não serão implementados como N+1. Até existir uma estratégia em lote, `files_json` fica vazio e o estado do deployment é registrado como `unknown`.

Depois de importar, informe o mesmo ambiente ao diagnosticar:

```bash
go run ./cmd/faultmap diagnose incident \
  --config ./faultmap-local/faultmap.yaml \
  --service checkout-service \
  --environment staging \
  --since 15m \
  --baseline 30m
```

O Faultmap consulta somente deployments já persistidos, dentro de até uma hora antes do início do incidente. Um deployment próximo recebe o finding `deployment_proximity`; quando o commit também corresponde à `service.version` observada nos spans do incidente, a confiança aumenta. O score usa uma queda linear pela distância temporal: um deployment seis minutos antes recebe score `0.90`, que com peso `0.20` contribui `0.18` para o suspeito. A saída sempre declara que proximidade e correspondência de versão não provam causalidade.

O detector `retry_storm` compara quantas vezes a mesma operação aparece por trace na baseline e no incidente. Quando a repetição cresce de forma anormal, ele registra as médias, o volume analisado e a operação afetada. Para o ranking, a regra reutiliza deliberadamente `graph_proximity`, pois mede uma repetição estrutural dentro do grafo do trace; não existe um peso adicional no YAML. A contribuição segue `score × graph_proximity`: com score `0.80` e peso `0.15`, o valor auditável é `0.12`.

Repetição não prova que houve retry nem que ele causou o incidente. Fan-out legítimo, paginação, loops de negócio e instrumentação duplicada podem produzir spans semelhantes. Por isso, a saída apresenta essa conclusão como hipótese, preserva as limitações do finding e recomenda confirmar a política de retry e o fluxo da aplicação.

### Consultar sinais no terminal

Liste a telemetria persistida de um serviço em uma janela temporal limitada:

```bash
go run ./cmd/faultmap telemetry list \
  --config ./faultmap-local/faultmap.yaml \
  --service checkout-service \
  --since 8760h \
  --limit 10
```

A saída mostra somente campos seguros e úteis para investigação: horário, severidade, nome do span, status HTTP, operação de banco, duração, tipo de erro e trace ID. Ela ainda não calcula causa raiz; essa visualização será a entrada dos detectores de erro, latência e timeout.

### Diagnosticar um incidente

Compare uma janela de incidente com a baseline imediatamente anterior:

```bash
go run ./cmd/faultmap diagnose incident \
  --config ./faultmap-local/faultmap.yaml \
  --service checkout-service \
  --since 1m \
  --baseline 1m \
  --until 2025-12-01T10:02:00Z \
  --limit 100
```

`--until` é opcional e existe para reproduzir telemetria histórica; sem ele, o Faultmap usa o horário atual. O diagnóstico inicial compara taxa de erro HTTP, duração p95 e timeout de banco. Ele também relaciona timeout PostgreSQL a erro ou alta latência HTTP quando os sinais possuem o mesmo `trace_id`. Cada hipótese mostra score, evidências, confiança e limitações. Com as fixtures mínimas `checkout-normal.json` e `checkout-error-latency.json`, a confiança é baixa porque há somente um trace por janela — correlação não é apresentada como causalidade.

Para validar o diagnóstico com uma amostra maior e resultados determinísticos, use um workspace separado:

```bash
go run ./cmd/faultmap init --directory ./faultmap-volume

go run ./cmd/faultmap ingest file \
  --config ./faultmap-volume/faultmap.yaml \
  --input ./fixtures/otel/checkout-baseline-sample.json

go run ./cmd/faultmap ingest file \
  --config ./faultmap-volume/faultmap.yaml \
  --input ./fixtures/otel/checkout-incident-sample.json

go run ./cmd/faultmap diagnose incident \
  --config ./faultmap-volume/faultmap.yaml \
  --service checkout-service \
  --since 1m \
  --baseline 1m \
  --until 2025-12-01T10:02:00Z \
  --limit 100
```

Essa amostra contém 20 traces por janela. O resultado esperado inclui taxa de erro HTTP de 0% para 40%, duração p95 de 160 ms para 2.500 ms, 6 timeouts em 20 operações PostgreSQL e confiança alta. Os 6 traces com timeout também apresentam impacto HTTP no mesmo fluxo distribuído, fortalecendo a hipótese sem transformá-la em prova causal.

O terminal agrega essas evidências no ranking do `checkout-service`. Com os pesos padrão, o score é `0.40`: erros HTTP contribuem `0.10`, latência `0.09`, timeout de banco `0.06` e correlação pelo trace `0.15`. Esse número representa prioridade determinística de investigação, não probabilidade de causa. Cada parcela permanece visível e limitações repetidas são consolidadas ao final do relatório.

Ao concluir, o comando salva atomicamente o incidente, seus findings e o ranking no SQLite e imprime um identificador como `inc_be37a8fae2744b8cea62ed08`. Esse ID deriva do serviço e das janelas UTC. Repetir a mesma investigação é idempotente: o snapshot original não é substituído nem duplicado, e a CLI informa `Diagnóstico já existente`. Se a janela do incidente ainda não possuir sinais, a análise é exibida, mas não é persistida; isso permite repetir o comando depois que a telemetria chegar.

### Consultar incidentes persistidos

Liste os snapshots mais recentes sem executar novamente os detectores ou o ranking:

```bash
go run ./cmd/faultmap incident list \
  --config ./faultmap-volume/faultmap.yaml \
  --limit 20
```

A listagem lê apenas o resumo persistido de cada incidente: ID, serviço, status e janela. `--limit` define quantos registros mais recentes serão apresentados, aceita valores de 1 a 1.000 e evita uma leitura ilimitada. A ordem é estável, do início de incidente mais recente para o mais antigo, com o ID como desempate. Nesta versão ainda não há cursor nem `offset`; para acessar um snapshot fora do limite atual, aumente `--limit` dentro do máximo permitido.

Use o ID retornado pelo diagnóstico ou pela listagem para recuperar o snapshot completo:

```bash
go run ./cmd/faultmap incident show \
  --config ./faultmap-volume/faultmap.yaml \
  --id inc_be37a8fae2744b8cea62ed08
```

`incident show` lê as janelas, contagens, findings, evidências, limitações e ranking que foram gravados no momento do diagnóstico. Ele não consulta novamente a telemetria nem recalcula scores; assim, uma investigação continua auditável mesmo quando novos sinais chegam ao banco. A leitura aceita no máximo 1.000 findings por incidente para proteger memória e tempo de resposta.

Snapshots criados antes da inclusão dos metadados de baseline continuam compatíveis. Nesses diagnósticos legados, o comando apresenta a janela do incidente e os findings ou ranking disponíveis, informa explicitamente que a baseline e as contagens não estão disponíveis e não inventa valores zero.

### Exportar relatórios JSON e Markdown

Exporte o mesmo snapshot persistido em Markdown para leitura humana:

```bash
go run ./cmd/faultmap export report \
  --config ./faultmap-volume/faultmap.yaml \
  --incident inc_be37a8fae2744b8cea62ed08 \
  --format markdown > report.md
```

Ou gere o contrato JSON versionado para automações e integrações:

```bash
go run ./cmd/faultmap export report \
  --config ./faultmap-volume/faultmap.yaml \
  --incident inc_be37a8fae2744b8cea62ed08 \
  --format json > incident-summary.json
```

Os dois formatos são escritos na saída padrão para permitir redirecionamento ou composição com outras ferramentas. Nenhum deles relê a telemetria ou recalcula detectores e ranking. O Markdown arredonda scores para duas casas para facilitar a leitura; o JSON preserva a precisão numérica do snapshot e inclui `schema_version: "1"`. Em snapshots legados, `baseline` é `null` no JSON e o Markdown declara que os metadados estão indisponíveis.

### Investigar um trace

Use um `trace_id` apresentado pelo diagnóstico ou pela listagem de telemetria para reconstruir seu fluxo:

```bash
go run ./cmd/faultmap blame trace \
  --config ./faultmap-volume/faultmap.yaml \
  --trace 30000000000000000000000000000001 \
  --limit 20
```

O comando faz uma única consulta parametrizada e limitada, constrói o grafo em memória e mostra somente campos seguros. Para a fixture de incidente, a saída liga `POST /checkout` com HTTP `500` à operação `INSERT orders` que terminou em timeout PostgreSQL. A relação usa o `parentSpanId` do OTLP quando disponível; telemetria antiga só recebe o fallback quando existe exatamente um span HTTP e um span de banco no trace.

### Exportar o grafo em Mermaid

O mesmo grafo pode ser exportado em um formato renderizável pelo GitHub e por ferramentas compatíveis com Mermaid:

```bash
go run ./cmd/faultmap export graph \
  --config ./faultmap-volume/faultmap.yaml \
  --trace 30000000000000000000000000000001 \
  --format mermaid \
  --limit 20
```

Por padrão, o diagrama é escrito na saída padrão. Para criar um artefato no diretório reservado pelo `init`, use o redirecionamento do terminal:

```bash
go run ./cmd/faultmap export graph \
  --config ./faultmap-volume/faultmap.yaml \
  --trace 30000000000000000000000000000001 \
  --format mermaid \
  --limit 20 \
  > ./faultmap-volume/faultmap-out/trace-checkout.mmd
```

Os identificadores Mermaid são sintéticos e os rótulos são escapados. Assim, nomes vindos da telemetria não são interpretados como sintaxe do diagrama.

## Especificação

A especificação é modular e sua leitura completa é obrigatória antes de implementar ou revisar o projeto. Comece por [FAULTMAP_MVP.md](FAULTMAP_MVP.md), que direciona para todos os documentos normativos em [`docs/mvp/`](docs/mvp/).

## Estado atual

O núcleo funcional do MVP está implementado. A CLI inicializa o workspace, recebe traces OTLP HTTP em JSON/protobuf (incluindo gzip), importa traces OTLP de arquivo, coleta commits/deployments do GitHub, consulta a telemetria, diagnostica e persiste incidentes, recupera o histórico de snapshots, exporta relatórios JSON/Markdown, reconstrói o grafo de um trace e o exporta em Mermaid. Os detectores atuais cobrem aumento de erros, aumento de latência, timeout PostgreSQL, correlação desses impactos pelo mesmo `trace_id`, proximidade de deployment com correspondência de versão e repetição anormal da mesma operação por trace. O ranking agrega essas evidências com pesos configuráveis e contribuições auditáveis. A `demo-shop` instrumentada reproduz seis falhas controladas, e a matriz E2E automatizada cobre os seis cenários com bancos isolados, telemetria OTLP real e expectativas determinísticas; o cenário de timeout também importa commit/deployment de um mock GitHub local e comprova a correspondência com `service.version`.
