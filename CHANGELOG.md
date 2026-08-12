# Changelog

## Não publicado

### Adicionado

- **O commit implantado passa a ser suspeito por direito próprio no ranking.**
  Antes ele existia apenas dentro do texto da evidência do serviço: quem lia via
  "o checkout está suspeito" e precisava caçar, na explicação, qual mudança
  havia entrado. Num incidente causado por deploy, essa é a informação mais
  acionável que existe.

  ```text
  1. checkout-service                                      0.54
  2. payment-service                                       0.24
  3. commit 01234567 — Versão E2E com regressão de timeout  0.20  (commit)
  ```

  O ranking deixou de agrupar por serviço e passou a agrupar por **sujeito**,
  que pode ser um serviço ou um commit. Detectores que não declaram sujeito
  continuam acusando o próprio serviço, sem alteração alguma.

  Só commits **implantados** viram suspeitos: um commit que não chegou a rodar
  não tem relação observável com o incidente. A mensagem que torna o rótulo
  legível é buscada em lote, uma consulta para todo o escopo.

### Removido

- O caminho de diagnóstico de serviço único (`DiagnoseIncident` e
  `DiagnoseIncidentWithDeployments`) ficou órfão quando a investigação passou a
  comparar serviços, e só era exercitado pelos próprios testes. A cobertura de
  validação e de correlação de deployment foi portada para o caminho vivo antes
  da remoção.

### Corrigido

- **Retry storm confundindo operações diferentes.** Ao aceitar spans de banco
  sem atributo de operação, todas as chamadas de um mesmo sistema colapsavam em
  uma assinatura só: uma aplicação que passasse a fazer mais consultas por
  requisição — mudança legítima de padrão — apareceria como tempestade de retry.
  O nome do span passa a ser o discriminador quando o atributo não vem. Ele é o
  que a telemetria realmente traz, e usá-lo evita inventar uma operação
  inexistente. Em Go o nome é genérico (`sql.conn.query`), então distinguimos
  leitura de escrita, mas não uma consulta de outra.
- **Retry storm invisível em banco sem atributo de operação.** A assinatura de
  uma chamada de banco exigia `db.operation`, e desistia do span quando ele não
  vinha. A captura da instrumentação oficial do Node mostrou que os 68 spans de
  banco não trazem esse atributo — a biblioteca coloca a operação no nome do
  span, como `pg.query:SELECT captura`. Uma tempestade de retry no banco era
  invisível em qualquer aplicação instrumentada assim. O sistema de banco já
  identifica a repetição; a operação apenas refina o rótulo quando existe.

### Adicionado

- Telemetria capturada de mais três instrumentações oficiais:
  `nodejs-pg.json`, `golang-otelsql.json` e `java-agent-jdbc.json`. Com as de
  FastAPI, psycopg2, SQLite e DuckDB, os detectores passam a ser exercitados
  contra **sete** instrumentações de terceiros, em quatro linguagens.

  As capturas confirmaram que nenhuma biblioteca segue uma convenção por
  inteiro: o `otelsql` do Go emite `db.system` (anterior) junto de
  `db.query.text` (atual), e o agente do Java usa a convenção anterior em quase
  tudo enquanto o Node usa a atual. Aceitar as duas não é preciosismo.

  Verificação de privacidade feita com medição, não suposição: o agente do Java
  emite `db.connection_string`, e um teste com usuário real e senha embutida na
  URL de conexão confirmou que ele sanitiza o valor antes de exportar.

### Verificação

- Java: cinco detectores funcionaram sem nenhuma correção. Go: três, cobrindo
  erro, timeout e latência de banco. Node: cinco. Nenhuma cegueira nova apareceu
  em quatro linguagens.
- Cinco detectores funcionaram contra o Node sem nenhuma correção:
  `error_rate_delta`, `latency_delta`, `database_timeout`, `database_error` e
  `database_http_trace_correlation`. A instrumentação usa a convenção estável,
  que o produto já reconhecia — não houve a terceira cegueira que se temia.
- Matriz E2E: 6 de 6. Modo difícil: 5 de 5, incluindo o cenário de fan-out
  legítimo, que era o risco direto de alargar a assinatura do retry.

## v0.3.1 — 2026-08-10

### Adicionado

- **`database_latency_delta`** — banco que ficou mais lento sem falhar. Os dois
  detectores de banco existentes procuram falha: timeout e erro. Um banco que
  apenas degrada não era percebido por nenhum, e o diagnóstico mostrava só a
  latência HTTP que a degradação arrastava — quem investigava via "a API ficou
  lenta" sem ver "o banco ficou lento", que é a informação que aponta onde mexer.

  O caso apareceu na primeira carga gerada contra uma aplicação real: o banco
  ficou catorze vezes mais lento, sem uma única falha, e o diagnóstico não o
  mencionava.

  Este detector não consta da lista de dez do documento normativo. Ele foi
  pedido pela realidade, não pela especificação.

  Duas decisões de contrato: o limiar é próprio, mais baixo que o de latência
  HTTP, porque uma requisição dispara várias consultas e acréscimos pequenos se
  acumulam — exige 2 ms de aumento e que a duração ao menos dobre; e só mede
  operações que **concluíram**, já que uma operação que estourou o tempo é lenta
  por definição e reportá-la aqui repetiria o que `database_timeout` já explica.

### Verificação

- Contra a aplicação real que motivou o detector: p95 do banco de 0,40 ms para
  5,67 ms, sem nenhuma falha, agora reportado ao lado da latência HTTP.
- Matriz E2E: 6 de 6. O detector fala nos três cenários em que o banco degrada
  (760 ms, 1532 ms e 630 ms de p95 no incidente) e permanece calado nos outros
  três. Nenhum diagnóstico existente mudou de suspeito.
- Modo difícil: 5 de 5, incluindo silêncio nos cenários sem culpado e de ruído
  crônico.

## v0.3.0 — 2026-08-10

Completa a lista de detectores do documento normativo e fecha os dois últimos
itens que faltavam da especificação: o comando `explain suspect` e a gravação
dos artefatos em disco.

### Adicionado

- **Os quatro detectores restantes**, fechando os dez previstos:
  - `database_error` — falhas de banco que não são timeout, comparadas entre as
    janelas. Cancelamento provocado por quem chamou é deliberadamente ignorado:
    o banco não falhou, quem desistiu foi o serviço de cima, e contá-lo culparia
    o serviço de baixo por um problema que nasceu acima dele.
  - `version_regression` — compara duas versões do mesmo serviço convivendo na
    janela do incidente, como em um rollout parcial. Exige volume mínimo por
    versão, para que um canário de três requisições não vire conclusão.
  - `dependency_failure` — marca o serviço cuja falha aparece sob a falha de
    quem o chamou, no mesmo trace. O finding pertence ao serviço mais profundo,
    candidato à origem, e não a quem sofreu a consequência.
  - `trace_break` — aponta ligação entre serviços que existia na baseline e
    desapareceu no incidente. Uma lacuna presente nas duas janelas descreve a
    instrumentação, não o incidente, e é silenciada.

- **`faultmap export artifacts`** grava em disco os cinco artefatos previstos
  pela especificação: `report.md`, `ranking.json`, `evidence-graph.mmd`,
  `incident-summary.json` e `timeline.json`. O `init` criava o diretório
  `faultmap-out/` e nada nunca escrevia nele; as saídas existiam apenas em
  stdout. `ranking.json` e `incident-summary.json` não existiam.
- **`faultmap explain suspect <serviço> --incident <id>`** detalha as parcelas
  do score de um suspeito, as evidências que as sustentam, a proveniência e as
  limitações. Era o último dos onze comandos previstos que faltava. Como
  `incident show`, lê o snapshot e não reexecuta nada.

### Alterado

- **Teto por classe de peso no ranking.** Quatro regras passam a dividir
  `graph_proximity`, e somar livremente faria a evidência estrutural valer mais
  que o aumento de erros apenas por existirem mais regras daquele tipo. O total
  de cada classe passa a ser limitado ao peso configurado para ela; as
  contribuições individuais continuam todas visíveis. Serviços que disparam duas
  regras da mesma classe pontuam menos que antes. Ver ADR 0010, que revisita o
  ADR 0001 como ele mesmo previa.
- **Cenário `retry-storm` redesenhado.** Ele configurava o payment para falhar
  em 100% das chamadas; com o ranking comparando serviços, quem quebrava era o
  payment e o retry era reação. O defeito mudou de lugar: payment saudável,
  checkout com timeout curto demais e quatro tentativas.

### Corrigido

- **Janela de incidente chegando vazia de forma intermitente.** O SDK agrupa
  spans por 5 segundos e o coletor acrescenta o próprio lote, mas os runners
  esperavam 6 — menos de um segundo de margem. A espera sobe para 10 segundos.
  Defeito do arcabouço de teste, não do produto, mas que o tornava não confiável.
- **Grafo de evidências cego para a convenção anterior de HTTP.** Terceira
  aparição do mesmo defeito: a classificação de spans no grafo reconhecia apenas
  `http.response.status_code`, então uma aplicação instrumentada automaticamente
  tinha seus spans HTTP ignorados ali enquanto os detectores, já corrigidos, os
  enxergavam. As listas de convenções estavam copiadas em cinco lugares e três já
  haviam divergido — uma com a precedência invertida, outra sem a convenção
  anterior de operação de banco. Todas passam a viver em
  `internal/telemetry/semconv`.
- **Artefatos discordavam sobre a ordem dos suspeitos.** O `report.md`
  reordenava por score e ID, então em um empate ele podia eleger um primeiro
  suspeito diferente do `ranking.json` gravado no mesmo diretório. Um teste novo
  amarra os três artefatos e falha se qualquer um voltar a inventar a própria
  ordem.
- **Exportação contradizia o terminal.** Os renderizadores JSON reordenavam os
  suspeitos por score e ID, enquanto o terminal preservava a ordem gravada. Em
  um empate isso desfazia o desempate por profundidade na cadeia e colocava a
  vítima em primeiro lugar no `ranking.json` e no relatório JSON. A ordem
  persistida é o ranking e não é mais recalculada na exportação.

### Verificação

- Matriz E2E: 6 de 6. Modo difícil: 5 de 5, agora exigindo silêncio também das
  quatro regras novas no cenário sem culpado.
- Os quatro detectores foram submetidos à telemetria capturada de instrumentação
  de terceiros usando a mesma janela como baseline e incidente: nada mudou entre
  elas, então qualquer finding seria falso positivo por construção. Nenhum falou.

## v0.2.0 — 2026-08-09

O diagnóstico passa a **comparar serviços** em vez de analisar um por vez. É
mudança de comportamento, não correção: a saída de `diagnose incident` muda para
quem já usa o Faultmap.

### O defeito de projeto que isto corrige

`diagnose incident --service X` lia sinais apenas de X, os detectores filtravam
para X e o ranking agrupava por serviço. O resultado é que **nunca havia mais de
um suspeito**: a lista tinha sempre um nome, e a meta de "top-3 ≥ 80%" era
verdadeira por vacuidade — posição 1 de 1, em todos os cenários.

Pior que a métrica vazia era o comportamento. O produto promete responder "por
onde começo a investigar", mas exigia que a pessoa já soubesse a resposta ao
escolher o serviço. Num incidente em que o `payment` quebra e o `checkout` fica
lento por consequência, investigar o `checkout` produzia um diagnóstico correto
sobre a vítima e silencioso sobre a origem.

### Adicionado

- **Escopo descoberto pelos traces.** A partir do serviço informado, o Faultmap
  identifica quem participou dos mesmos traces durante o incidente e ranqueia
  todos juntos. É a topologia observada, não um palpite.
- **`--depth`** percorre saltos adicionais de trace. Dentro de um mesmo trace a
  cadeia inteira já é alcançada no primeiro nível; os saltos servem para
  serviços ligados por **outros** traces — uma rotina interna que usa a mesma
  dependência do fluxo do usuário, por exemplo. Padrão 1, máximo 5.
- **`--all`** compara todos os serviços com telemetria na janela, para quem não
  tem por onde começar. **`--no-expand`** preserva o modo focado anterior.
  `--service` aceita lista separada por vírgula. **`--max-services`** limita o
  escopo.
- **A saída declara o escopo**: quais serviços foram comparados, a quantos
  saltos cada um está, de onde o escopo veio e quantos traces o sustentaram.
  Sem isso o ranking apresentaria serviços sem explicar por que estão ali.

### Corrigido

- **Desempate premiava a vítima.** Quando um serviço falha, quem o chamou tende
  a falhar junto: ambos chegam a 100% de erro e empatam em score. O desempate
  alfabético colocava a vítima em primeiro — flagrado pela matriz E2E, com o
  `checkout-service` à frente do `payment-service`. Suspeitos empatados passam a
  ser ordenados pela profundidade na cadeia da requisição. É desempate, não
  score, e não prova causalidade.
- **Oscilação de latência virando evidência.** Em um sistema saudável nas duas
  janelas, o detector relatou "a duração p95 aumentou de 3 ms para 3 ms". O
  aumento agora precisa ser relevante nas duas escalas: ao menos 5 ms e ao menos
  20%. O custo é declarado — uma piora real e pequena em um serviço muito rápido
  passa despercebida.
- **Nome de span inútil.** A instrumentação de banco nomeia o span com a
  primeira palavra da consulta; consultas que começam com comentário SQL viravam
  spans chamados `--`. A apresentação passa a reconstruir o rótulo a partir da
  operação e do sistema de banco. O dado armazenado não é alterado.

### Mudança de comportamento

- `--service X` passa a comparar X com os serviços vizinhos. Use `--no-expand`
  para o comportamento anterior.
- **`--limit` agora vale para o total de sinais da janela, não por serviço.** Um
  escopo grande divide o mesmo orçamento entre mais serviços; investigações
  amplas devem aumentar o limite.

### Limitações conhecidas

- O desempate por profundidade depende de `span.parent_id`. Uma cadeia
  instrumentada parcialmente deixa todos na profundidade zero e o desempate
  volta a ser alfabético.
- Cada salto adicional traz serviços mais distantes do incidente e aumenta o
  risco de falso positivo. O padrão continua em um salto por isso.

## v0.1.2 — 2026-08-08

Release de correção. A v0.1.1 corrigiu a cegueira para HTTP mas continuava cega
para **banco de dados**, e nunca aplicava a política de privacidade configurada.
**Recomendamos atualizar.**

### Corrigido

- **Cegueira para banco de dados.** Os detectores exigiam `db.system.name`,
  enquanto as instrumentações oficiais emitem `db.system`. Contra qualquer
  aplicação instrumentada automaticamente, todo span de banco era lido como zero
  sinais e `database_timeout` e `database_http_trace_correlation` nunca
  disparavam. A correção é por convenção, não por motor: **PostgreSQL, MySQL,
  SQLite, DuckDB e outros passam a funcionar juntos**, sem código específico.
- **Falha de banco invisível.** Instrumentações reais sinalizam falha pelo status
  do span e por evento de exceção, não pelo atributo `error.type` que a nossa
  demo escreve à mão. O status passa a ser considerado.
- **Causa do erro descartada.** O normalizador ignorava os eventos do span, onde
  as instrumentações registram `exception.type` e `exception.message`. Agora eles
  são preservados; `exception.stacktrace` é descartado por carregar caminhos
  absolutos da máquina sem sustentar nenhuma decisão.
- **Política de privacidade nunca aplicada.** `privacy.blocked_attributes` e
  `privacy.max_attribute_length` eram validados no YAML e não consultados por
  nenhum ponto do código. Ao ingerir telemetria de uma aplicação real, o texto
  completo das consultas era gravado no banco local. A política passa a ser
  aplicada entre a normalização e a persistência, cobrindo tanto a ingestão de
  arquivo quanto o receiver OTLP. `db.query.text` entra na lista bloqueada ao
  lado de `db.statement`.

### Adicionado

- `fixtures/otel/real/` — telemetria capturada de instrumentação de terceiros
  (PostgreSQL via psycopg2, SQLite, DuckDB e FastAPI de uma aplicação real), com
  testes que falham se o Faultmap voltar a contar zero sinais. É a rede de
  proteção que faltava: as fixtures anteriores eram escritas por nós, no mesmo
  dialeto do código que deveriam verificar.

### Verificação

- Sobre a mesma captura real de PostgreSQL e a mesma janela, a v0.1.1 responde
  "nenhuma anomalia" e esta versão relata "12 de 24 operações PostgreSQL tiveram
  timeout".
- Matriz E2E: 6 de 6. Modo difícil: 5 de 5.

### Limitações conhecidas que permanecem

- Bancos criados antes desta versão podem conter atributos que a política agora
  bloquearia; a limpeza retroativa não é automática.
- A cobertura de convenções vem das instrumentações Python capturadas.
  Instrumentações de Go, Java e Node podem emitir combinações ainda não vistas.
- `diagnose --service X` analisa um serviço por vez, então o ranking nunca tem
  mais de um suspeito e a métrica de top-3 não mede nada.

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
