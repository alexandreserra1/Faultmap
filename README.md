# Faultmap

> Investigação determinística de incidentes para aplicações backend.

O Faultmap recebe telemetria, compara o comportamento normal com uma janela de incidente e aponta os principais suspeitos — sempre mostrando as evidências, o score e as limitações da hipótese.

![Arquitetura do MVP do Faultmap](docs/images/faultmap-mvp-architecture.png)

## O que estamos construindo

Um monólito modular em Go, distribuído como um único binário e orientado à CLI. A etapa atual recebe traces e logs OpenTelemetry, correlaciona esses sinais com deploys e commits do GitHub e operações PostgreSQL, persiste o contexto localmente em SQLite e gera um ranking explicável de suspeitos. Traces são aceitos em JSON e protobuf; logs, apenas em JSON, e sem o texto da mensagem — o Faultmap guarda severidade, instante, correlação de trace e atributos permitidos, e remete ao sistema de logs de origem para quem precisar ler o conteúdo ([ADR 0011](docs/adr/0011-logs-guardam-apenas-metadados.md)). Métricas fazem parte da evolução planejada.

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
4. Detectores determinísticos encontram mudanças como aumento de erros, latência, timeout e falhas de banco, retry storm, regressão entre versões, falha propagada por dependência, perda de ligação entre serviços e mudança recente de schema numa base que o serviço consulta.
5. Um grafo de evidências conecta serviços, traces, deploys, commits e operações de banco.
6. O mecanismo de ranking gera os principais suspeitos e relatórios em terminal, JSON, Markdown e Mermaid.

O motor de diagnóstico não depende de LLM. O servidor MCP (`faultmap mcp`) apenas **lê** o que o motor já produziu: ele não investiga, não ingere e não apaga nada.

## Escopo do MVP

- CLI em Go e banco SQLite local;
- ingestão OTLP e importação de fixtures;
- comparação entre incident e baseline;
- detectores determinísticos e ranking auditável;
- correlação inicial com GitHub e PostgreSQL, incluindo mudanças de catálogo;
- ambiente `demo-shop` reproduzível com Docker Compose;
- relatórios para terminal, JSON, Markdown e Mermaid.

Interface web, Kubernetes, Neo4j, Kafka, SaaS e LLM obrigatória estão fora do primeiro MVP.

## Desenvolvimento local

### Requisitos

- Go `1.24.0` ou compatível com a versão declarada em [`go.mod`](go.mod);
- `make`, para executar os atalhos de qualidade.

Não é necessário instalar o binário globalmente para trabalhar no projeto. Os comandos abaixo usam o código local; o Go baixa as dependências declaradas em `go.mod` quando necessário.

### Verificações de qualidade

Execute na raiz do repositório:

```bash
make verify
```

O alvo encadeia formatação, `go vet`, a suíte de testes e o detector de corrida,
interrompendo no primeiro que falhar. Cada etapa decide pelo próprio código de
saída — filtrar a saída de `go test` com `grep` para enxugar a leitura descarta
justamente esse código e inverte o resultado, porque o `grep` devolve sucesso
quando encontra linhas, ou seja, quando há falhas.

Os alvos individuais continuam disponíveis: `make fmt`, `make fmt-check`,
`make test`, `make test-race` e `make vet`.

- `make fmt` aplica a formatação padrão do Go;
- `make test` executa a suíte de testes;
- `make test-race` executa a suíte com detecção de condições de corrida;
- `make test-integration` sobe um PostgreSQL descartável e roda os testes que exigem um servidor real; sem ele, esses testes se marcam como ignorados e o `make verify` segue verde;
- `make demo-test-hard` roda os cenários de falso positivo, incluindo `migracao-inofensiva`, que aplica uma migração real e exige silêncio em sistema saudável;
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

O processo mantém dois listeners independentes: `POST /v1/traces` e `POST /v1/logs` recebem lotes OTLP no endereço `server.otlp_http_address`, enquanto `GET /health` responde no endereço `server.health_address`. A ingestão reutiliza o mesmo normalizador e o mesmo pool SQLite durante todo o ciclo de vida do processo. Reenviar spans com os mesmos IDs é seguro: a persistência ignora duplicidades.

Envie uma fixture no formato OTLP JSON:

```bash
curl --fail-with-body \
  -H 'Content-Type: application/json' \
  --data-binary @./fixtures/otel/checkout-normal.json \
  http://127.0.0.1:4318/v1/traces
```

Uma ingestão aceita retorna `200 OK` e o `ExportTraceServiceResponse` vazio, representado como `{}` em JSON. O endpoint também aceita OTLP protobuf com `Content-Type: application/x-protobuf`; nesse caso, o corpo de sucesso é um protobuf vazio. O formato é determinado pelo `Content-Type`, e não pela extensão ou pelo conteúdo aparente do corpo.

**Logs são aceitos apenas em JSON.** Enviá-los em protobuf retorna `400`, e a resposta OTLP não pode explicar o motivo porque o protocolo exige mensagens estáveis e sem detalhes internos — então o processo escreve a causa no próprio terminal, uma vez por motivo distinto:

```text
Lote OTLP recusado em /v1/logs: payload OTLP inválido
normalizar logs: payload OTLP inválido: logs OTLP só são aceitos em JSON; no OpenTelemetry Collector, declare `encoding: json` no exportador otlphttp da pipeline de logs (traces seguem aceitos em protobuf)
```

Reenvios do mesmo lote não repetem a mensagem: a causa descreve a configuração, não o lote, e um exportador mal configurado insiste indefinidamente.

Isso torna o **Collector obrigatório para logs**, e não apenas recomendado: os SDKs de aplicação não exportam OTLP em JSON. O SDK Python, por exemplo, aceita apenas `grpc` e `http/protobuf`, e recusa `OTEL_EXPORTER_OTLP_PROTOCOL=http/json` na inicialização com `Unsupported OTLP protocol 'http/json' is configured`. Quem envia direto da aplicação consegue mandar traces, nunca logs; o Collector é quem converte para JSON no caminho.

Verifique a saúde do processo separadamente:

```bash
curl --fail http://127.0.0.1:8081/health
```

Resposta esperada:

```json
{"status":"ok"}
```

Para encaminhar telemetria de aplicações reais, configure um OpenTelemetry Collector. O exporter `otlphttp` acrescenta `/v1/traces` e `/v1/logs` ao `endpoint`. Note os dois exportadores: traces seguem em protobuf, que é o padrão, e logs exigem `encoding: json`.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

processors:
  batch:

exporters:
  otlphttp/faultmap:
    endpoint: http://faultmap:4318
  otlphttp/faultmap-logs:
    endpoint: http://faultmap:4318
    encoding: json

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/faultmap]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/faultmap-logs]
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

### Exportar a cronologia do incidente

O `timeline.json` reúne, em ordem cronológica, as janelas da investigação, os findings e o registro do diagnóstico:

```bash
go run ./cmd/faultmap export timeline \
  --config ./faultmap-volume/faultmap.yaml \
  --incident inc_001 \
  > ./faultmap-volume/faultmap-out/timeline.json
```

O artefato é derivado exclusivamente do snapshot persistido: ele não relê telemetria nem recalcula o diagnóstico. Como os findings não possuem instante próprio no snapshot, eles são ancorados ao início da janela do incidente e trazem `time_source` declarando essa origem — a decisão está registrada no [ADR 0004](docs/adr/0004-timeline-ancora-findings-na-janela-do-incidente.md).

### Aplicar a política de retenção

A limpeza é sempre explícita e nunca acontece durante a ingestão OTLP:

```bash
go run ./cmd/faultmap retention apply \
  --config ./faultmap-volume/faultmap.yaml
```

O comando remove telemetria mais antiga que `storage.retention`, em lotes limitados por `--batch-size`, mantendo transações curtas. Snapshots de diagnóstico são preservados para que investigações antigas continuem auditáveis; o alcance e as consequências estão no [ADR 0003](docs/adr/0003-retencao-preserva-snapshots-de-diagnostico.md). Quando o teto de lotes de uma execução é atingido, a saída avisa que ainda existe telemetria expirada e basta executar o comando novamente.

## Comparação entre serviços

O diagnóstico compara serviços em vez de analisar um por vez. A partir do
serviço informado, o Faultmap descobre quem participou dos mesmos traces durante
o incidente e ranqueia todos juntos:

```bash
go run ./cmd/faultmap diagnose incident \
  --config ./faultmap-local/faultmap.yaml \
  --service checkout-service \
  --since 30m --baseline 60m
```

A saída declara de onde veio o escopo, para que a comparação possa ser julgada:

```text
Escopo da investigação:
  2 serviço(s) comparado(s): checkout-service, payment-service
  Origem: expansão pelos traces do serviço de entrada
  Traces que sustentaram a expansão: 20
```

Use `--no-expand` para investigar apenas o serviço informado, `--all` para
comparar todos os serviços com telemetria na janela, ou uma lista separada por
vírgula em `--service`. `--max-services` limita o tamanho do escopo.

### Saltos de trace

Dentro de um mesmo trace a cadeia inteira já é alcançada no primeiro nível: se a
requisição passa por `checkout → payment → banco`, todos entram com o padrão. Os
saltos de `--depth` servem para o caso diferente — um serviço que **nunca**
aparece nos traces do serviço de entrada, mas divide traces com um vizinho:

```text
salto 1:  checkout → payment    (trace do usuário)
salto 2:  payment  → ledger     (trace de uma rotina interna)
```

Com `--depth 2`, o `ledger` entra na comparação mesmo sem nunca ter participado
de um trace do checkout. Cada salto adicional traz serviços mais distantes do
incidente e aumenta o risco de falso positivo, por isso o padrão é um salto e o
máximo são cinco. A expansão para sozinha quando um nível não descobre ninguém
novo.

Quando dois serviços empatam em score — o caso comum de um falhar e o outro
falhar junto por consequência — vem primeiro quem está mais fundo na cadeia da
requisição. É um desempate heurístico sobre a topologia observada, descrito no
[ADR 0009](docs/adr/0009-investigacao-compara-servicos-por-escopo-de-traces.md),
e nunca altera a ordem de suspeitos com pontuações diferentes.

## Compatibilidade com instrumentação real

Os detectores reconhecem as duas convenções do OpenTelemetry para cada atributo
— a estável e a anterior — porque quem escolhe o nome é a biblioteca de
instrumentação, não a aplicação. Reconhecer apenas uma delas deixava o Faultmap
cego para aplicações inteiras, sem erro e sem aviso.

O reconhecimento de banco não cita nenhum motor: qualquer sistema declarado pela
instrumentação é aceito, de PostgreSQL a SQLite e DuckDB.

As fixtures em [`fixtures/otel/real/`](fixtures/otel/real/) são capturas de
instrumentação de terceiros e sustentam os testes que impedem essa classe de
regressão. As fixtures em `fixtures/otel/` são escritas por nós e provam apenas
que o produto funciona contra si mesmo.

### Gravar os artefatos em arquivo

Em vez de redirecionar cada formato na mão, o comando abaixo grava de uma vez os
cinco artefatos previstos:

```bash
go run ./cmd/faultmap export artifacts \
  --config ./faultmap-local/faultmap.yaml \
  --incident inc_001
```

```text
faultmap-out/
├── report.md
├── ranking.json
├── evidence-graph.mmd
├── incident-summary.json
└── timeline.json
```

O diretório precisa existir — ele é criado pelo `init`. Use `--output` para
gravar em outro lugar.

### Explicar um suspeito

```bash
go run ./cmd/faultmap explain suspect payment-service \
  --config ./faultmap-local/faultmap.yaml \
  --incident inc_001
```

A saída detalha cada parcela do score, as evidências que a sustentam e a
proveniência resumida. Uma contribuição sem evidência gravada aparece declarada
como tal, em vez de ser omitida. Como `incident show`, o comando lê o snapshot e
não reexecuta detectores nem ranking.

## Privacidade

O Faultmap descarta atributos sensíveis **entre a normalização e a persistência**, então o que é bloqueado nunca chega ao disco. Por padrão saem de circulação o corpo de requisição (`http.request.body`), o SQL executado (`db.statement` e `db.query.text`, os dois nomes que a convenção já teve) e o caminho absoluto do arquivo de origem que o SDK de logs anexa a cada registro (`code.file.path` e `code.filepath`). `code.function.name` e `code.line.number` continuam permitidos: ajudam a investigar sem expor a estrutura de diretórios.

A lista de `privacy.blocked_attributes` no YAML **soma** aos padrões em vez de substituí-los ([ADR 0012](docs/adr/0012-bloqueios-de-privacidade-somam-em-vez-de-substituir.md)):

```yaml
privacy:
  blocked_attributes:
    - user.email
    - user.document
```

A configuração acima bloqueia esses dois atributos **e** os cinco padrões. Declarar o vocabulário sensível do próprio negócio é acrescentar proteção, nunca abrir mão dela.

Logs entram sem o texto da mensagem. `privacy.store_raw_logs` permanece no arquivo por compatibilidade e não tem efeito.

## Mudanças de schema

Uma migração é uma das explicações mais frequentes para um incidente, e era a única que o Faultmap citava sem observar. A coleta compara duas leituras do catálogo:

```bash
export FAULTMAP_PG_DSN="postgres://usuario:senha@host:5432/payments?sslmode=disable"
faultmap ingest schema --database payments
```

A primeira execução estabelece a linha de base e não acusa nada. A partir da segunda, cada coleta registra o que mudou desde a anterior: tabela, coluna, índice ou restrição adicionada, removida ou alterada.

Coletas e mudanças recebem identificadores de 26 caracteres com o instante à frente, então ordenar por ID é ordenar cronologicamente. Eles são derivados do conteúdo, e não sorteados: recoletar o mesmo catálogo produz os mesmos identificadores, que é o que mantém a coleta idempotente.

Objetos são identificados com o schema à frente (`public.pedidos.valor`), porque um banco PostgreSQL quase nunca tem um schema só. Uma coleta que volte sem coluna nenhuma é **recusada** em vez de comparada: é a assinatura de um usuário que perdeu `SELECT` no catálogo, e não de uma migração.

A DSN vem do ambiente e **não** é gravada no `faultmap.yaml`, que o `init` promete criar sem tokens nem credenciais. A coleta é somente leitura: nenhum slot de replicação, nenhuma extensão, nenhum privilégio além de `SELECT` no catálogo.

O detector `schema_change_proximity` acusa um serviço quando uma mudança recente atingiu uma **tabela ou base que ele de fato consulta** — o vínculo vem dos spans de banco da janela do incidente, e não de configuração declarada. A tabela vem primeiro porque a instrumentação real quase sempre emite `db.collection.name` e quase nunca `db.namespace`, e porque ela é uma ligação mais estreita que a base. A janela de busca é de 24 horas, mais larga que a do deployment porque uma migração raramente quebra no instante em que roda.

A mudança de schema é **evidência de apoio**: ela só aparece quando o serviço já tem algum sintoma observado na janela. Uma migração sem efeito observável não é evidência de nada, e um ranking que sempre acha um culpado é indistinguível de um que adivinha.

Duas limitações vão declaradas em todo finding produzido:

- o instante da mudança é um **intervalo entre duas coletas**, não um instante — coletas mais frequentes estreitam o intervalo;
- proximidade temporal não prova causalidade, como em todo o resto do produto.

Sobre o que é guardado, ver [ADR 0014](docs/adr/0014-schema-guarda-identificadores-nao-expressoes.md): entram nome de tabela, coluna, índice, restrição e tipo de dado; **não** entram as expressões de `DEFAULT` e de `CHECK`, que carregam valor e regra de negócio. Uma mudança de expressão é registrada como "a expressão associada mudou", sem os dois valores.

## Servidor MCP

`faultmap mcp` expõe os diagnósticos já registrados a clientes MCP, por stdin/stdout:

```bash
faultmap mcp --config ./faultmap-local/faultmap.yaml
```

São três ferramentas, todas de leitura:

| Ferramenta | O que devolve |
| --- | --- |
| `list_incidents` | os diagnósticos registrados, do mais recente ao mais antigo |
| `get_incident` | o diagnóstico completo: janelas, hipóteses com evidência e proveniência, suspeitos e limitações |
| `explain_suspect` | cada parcela do score de um suspeito, o que a sustenta e as hipóteses que não pontuaram |

**Não existe ferramenta que dispare investigação, ingira dados ou apague nada.** É uma decisão, não uma etapa pendente: o motor do Faultmap é determinístico, e o papel de um LLM é consumir e explicar o resultado estruturado. Isso também zera a superfície de privacidade — tudo que sai já passou pela política aplicada na ingestão, então o servidor não tem como revelar o que a ingestão barrou.

Como stdout é o transporte do protocolo, a auditoria de chamadas sai por stderr. Ela registra instante, ferramenta e desfecho, sem os argumentos.

## Decisões arquiteturais

As decisões cujo motivo não é dedutível do código estão registradas em [`docs/adr/`](docs/adr/).

## Especificação

A especificação é modular e sua leitura completa é obrigatória antes de implementar ou revisar o projeto. Comece por [FAULTMAP_MVP.md](FAULTMAP_MVP.md), que direciona para todos os documentos normativos em [`docs/mvp/`](docs/mvp/).

## Estado atual

O núcleo funcional do MVP está implementado. A CLI inicializa o workspace, recebe traces OTLP HTTP em JSON/protobuf e logs OTLP em JSON (incluindo gzip), importa traces OTLP de arquivo, coleta commits/deployments do GitHub e o catálogo de bases PostgreSQL, consulta a telemetria, diagnostica e persiste incidentes, recupera o histórico de snapshots, exporta relatórios JSON/Markdown, gera a cronologia `timeline.json`, aplica a política de retenção, reconstrói o grafo de um trace e o exporta em Mermaid, e expõe os diagnósticos registrados por MCP.

São treze detectores: aumento de erros HTTP, aumento de latência, timeout PostgreSQL, erro de banco fora de timeout, aumento de latência de banco, correlação desses impactos pelo mesmo `trace_id`, repetição anormal da mesma operação por trace, proximidade de deployment com correspondência de versão, regressão entre versões que convivem na mesma janela, falha de dependência downstream, quebra de propagação de trace que surgiu no incidente, logs de erro correlacionados a requisições que falharam, e mudança de schema recente numa base que o serviço consulta. Os doze primeiros comparam duas janelas: nenhum reporta estado absoluto, porque um serviço que sempre erra descreve o próprio sistema e não o incidente. O ranking agrega essas evidências com pesos configuráveis, contribuições auditáveis e teto por classe de peso.

Cada evidência declara também o que aquele padrão de sinais **costuma** significar — lock, saturação de pool, consulta sem índice — sempre com mais de uma possibilidade, porque o produto não afirma causalidade ([ADR 0013](docs/adr/0013-evidencia-diz-o-que-o-padrao-costuma-significar.md)).

A `demo-shop` instrumentada reproduz seis falhas controladas, e a matriz E2E automatizada cobre os seis cenários com bancos isolados, telemetria OTLP real e expectativas determinísticas; o cenário de timeout também importa commit/deployment de um mock GitHub local e comprova a correspondência com `service.version`.

O [piloto cego](examples/pilot/) levou o produto a uma aplicação que não é nossa: top-1 em três de três cenários de ranking, incluindo o caso em que culpado e vítima ficaram lentos quase igual e o desempate veio da evidência de banco. Dois dos treze detectores dispararam em telemetria real; os outros onze seguem exercitados apenas por cenários que nós desenhamos — o de mudança de schema entre eles, verificado contra um PostgreSQL real mas ainda não contra um incidente que ninguém planejou. O [resultado](examples/pilot/resultado.md) registra isso em vez de contar o silêncio deles como cobertura.

O que ainda **não** foi validado: nenhum incidente **inesperado**, que ninguém tenha planejado, passou pelo produto. E não sabemos ainda se a frase de causas comuns muda a hipótese que uma pessoa formula — é o que o próximo piloto mede.

Cada cenário da matriz também mede as metas do MVP: top-1 e top-3 do serviço esperado, tempo de diagnóstico abaixo de 10 segundos, estabilidade do ranking entre execuções idênticas, 100% das evidências com proveniência e a geração válida de `report.json`, `report.md`, `timeline.json` e do grafo Mermaid.
