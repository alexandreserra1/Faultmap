# ADR 0007 — Telemetria de instrumentação real como base de teste

- Status: aceito
- Data: 2026-08-08

## Contexto

A v0.1.1 corrigiu a cegueira do Faultmap para a convenção HTTP anterior. O
mesmo defeito existia, intacto, no banco de dados — e a razão de não termos
encontrado os dois juntos é estrutural, não um descuido.

Toda a telemetria de teste do projeto era escrita por nós, usando os mesmos
nomes de atributo que o código sob teste esperava. Onze cenários automatizados
passavam enquanto o produto estava cego para aplicações reais. A demo prova que
o Faultmap funciona contra si mesmo; ela não pode provar compatibilidade, porque
quem escolhe os nomes dos atributos é a **biblioteca de instrumentação**, não a
aplicação.

Captura feita com as bibliotecas oficiais revelou três desencontros no caminho
de banco de dados:

1. `db.system`, e não `db.system.name`. Todo span de banco era lido como zero
   sinais: `database_timeout` e `database_http_trace_correlation` nunca
   disparavam.
2. Nenhum atributo de operação é emitido pela instrumentação DBAPI — só
   `db.statement`.
3. Falhas são sinalizadas por status do span e por evento `exception`, não pelo
   atributo `error.type` que a nossa demo escreve à mão.

## Decisão

**Convenções em um lugar só.** `attributeConventions`, em
`internal/detection/detectors.go`, é a única lista de precedência do projeto:
convenção estável primeiro, legada depois. Os renderizadores usam a mesma ordem.
Foi a divergência entre tela e detector que manteve a cegueira de HTTP
invisível — a tela mostrava "HTTP 200" enquanto o detector não via nada.

**Falha reconhecida pelo status do span.** `databaseFailures` considera falha
qualquer operação com severidade de erro; `databaseTimeouts` restringe às que
têm evidência textual de timeout, para que a evidência apresentada continue
descrevendo o que foi observado.

**Eventos de exceção preservados.** O normalizador promove `exception.type` e
`exception.message` a atributos do sinal, nos caminhos JSON e protobuf.
`exception.stacktrace` é descartado no próprio normalizador — carrega caminhos
absolutos e trechos de código, tem cardinalidade alta e não sustenta nenhuma
decisão. A supressão fica antes da persistência, não apenas na configuração.

**Privacidade.** `db.query.text`, o nome atual do SQL bruto, entra na lista
bloqueada por padrão ao lado de `db.statement`. Bloquear só um deixaria SQL
bruto ser gravado conforme a instrumentação da aplicação.

**Fixtures reais.** `fixtures/otel/real/` guarda capturas de instrumentação de
terceiros, e `internal/detection/real_telemetry_test.go` as carrega pelo mesmo
normalizador da ingestão. Um teste que afirma "os sinais são contados, nunca
zero" é o que teria impedido a release cega.

## Consequências

- O Faultmap passa a funcionar com **qualquer** banco sem código por motor:
  `filterDatabaseSignals` sempre aceitou qualquer valor de sistema; o que
  travava era o nome do atributo. Postgres, MySQL, SQLite, DuckDB e outros
  passam juntos.
- Verificado no nível do comando: sobre a mesma captura real de PostgreSQL e a
  mesma janela, a v0.1.1 responde "nenhuma anomalia" e esta versão relata
  "12 de 24 operações PostgreSQL tiveram timeout".
- Ler eventos aumenta o volume de atributos persistidos. O ganho — saber a causa
  da falha — justifica o custo, e o stacktrace, que é o campo volumoso, não é
  guardado.
- Quando um span traz as duas convenções com valores divergentes, vence a
  estável. É uma escolha nossa, não uma regra do OpenTelemetry.
- **Lacuna que permanece:** não capturamos spans de DuckDB da aplicação real. Os
  endpoints exercitados servem dados estáticos ou exigem autenticação, e criar
  um usuário no banco dela seria invasivo. A instrumentação DBAPI que produziria
  esses spans é a mesma já capturada de PostgreSQL e SQLite, mas isso é
  inferência, não medição.
- A cobertura continua limitada às instrumentações Python capturadas. Go, Java e
  Node podem emitir combinações que ainda não vimos.

## Como recapturar

1. Subir um OpenTelemetry Collector com receiver OTLP e exporter `file`,
   apontando para um diretório com permissão de escrita.
2. Executar a aplicação instrumentada pelas bibliotecas oficiais, exportando
   para esse coletor.
3. Converter a saída em JSON Lines para um único documento OTLP JSON por
   serviço, truncando apenas `exception.stacktrace`.

O procedimento completo está registrado em `fixtures/otel/real/README.md`.
