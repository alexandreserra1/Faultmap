# Changelog

## Não publicado

### Adicionado

- **Cauda de latência de banco como regra própria (`database_latency_tail`).**
  Um piloto cego mostrou que o produto ficava calado diante de um
  `ACCESS EXCLUSIVE` real: das 120 operações da janela, cinco esperaram dois
  segundos, nenhuma falhou — logo todas entraram na conta —, e o detector de p95
  não se moveu, porque a cauda inteira vivia acima do percentil, por uma
  observação. O p50 era 0,5 ms, o p95 21,11 ms e o p97 1.999 ms.

  A regra nova pergunta "alguém esperou muito além do normal?" em vez de "a
  janela inteira ficou mais lenta?". Acusa quando ao menos três operações passam
  de `max(p95 da baseline × 20, 100 ms)` e a proporção do incidente é mais que o
  dobro da da baseline. Divide a classe de peso `database_evidence` com as outras
  regras de banco, então uma degradação uniforme dispara as duas sem que o mesmo
  fato seja pago duas vezes.

  Não é calibragem de piso: um lock bloqueia quem colide com ele enquanto é
  mantido, o que numa janela curta é sempre uma minoria — qualquer percentil
  estável o bastante deixaria passar. Veja a
  [ADR 0017](docs/adr/0017-cauda-de-banco-e-pergunta-propria-nao-outro-percentil.md).


- **Mudança de schema como sinal de incidente.** O produto já dizia, em duas
  regras, que migração de schema é uma explicação frequente — o texto de causas
  comuns de `deployment_proximity` cita "migração de schema que acompanhou o
  deploy", e o de `database_error` cita "migração de schema incompatível com o
  código em execução" — e não observava nenhuma. Quem investigava recebia a
  hipótese e tinha que sair do Faultmap para verificá-la.

  `faultmap ingest schema --database <base>` lê o catálogo PostgreSQL, compara
  com a leitura anterior e registra o que mudou. A nova regra
  `schema_change_proximity` acusa o serviço quando uma mudança recente atingiu
  uma base que ele de fato consulta — o vínculo vem dos spans de banco da janela,
  não de configuração declarada. Sem esse vínculo seria coincidência temporal:
  qualquer migração em qualquer base acusaria qualquer serviço com problema no
  mesmo horário.

  A janela de busca é de 24 horas, contra uma hora do deployment. Um deploy que
  quebra costuma quebrar de imediato; uma migração quebra quando o código que a
  pressupõe encontra o schema, ou quando o volume cresce o bastante para que a
  falta do índice apareça.

  A regra divide a classe de peso com `deployment_proximity` em vez de ter peso
  próprio, porque na prática costumam ser o mesmo evento. Com pesos separados, um
  único deploy com migração somaria duas vezes e passaria à frente de um serviço
  que está de fato falhando; o teto por classe da ADR 0010 impede isso.

  Custo aceito: o instante da mudança é um **intervalo entre duas coletas**, não
  um instante, e isso vai declarado como limitação em todo finding. Uma mudança
  feita e revertida entre duas coletas é invisível — o produto prefere não ver a
  inventar um instante que não mediu. A replicação lógica daria precisão maior e
  não serve: ela não decodifica DDL, e capturá-lo por ali exigiria instalar event
  triggers na base de quem usa o produto. Ver ADR 0014.

- **Servidor MCP somente leitura.** `faultmap mcp` expõe os diagnósticos já
  registrados a clientes MCP por três ferramentas — `list_incidents`,
  `get_incident` e `explain_suspect`. Não há ferramenta que dispare investigação,
  ingira dados ou apague nada, e isso é posição do produto: o motor é
  determinístico, e o papel de um LLM é consumir e explicar o resultado
  estruturado, não participar da análise.

  Ser somente leitura também zera a superfície de privacidade: tudo que sai já
  passou pela política aplicada na ingestão, então o servidor não tem como
  revelar o que a ingestão barrou. As evidências viajam acompanhadas da frase de
  causas comuns da ADR 0013, pelo mesmo motivo que a apresentação ao humano:
  uma medida sozinha não evoca a lista que alguém experiente teria de imediato.

  O protocolo é escrito à mão, sem SDK: MCP sobre stdio é pequeno, o `go.mod`
  segue com as mesmas 12 dependências diretas e a sessão inteira é exercitada em
  memória, sem subir processo. O custo é acompanhar a evolução da especificação
  manualmente.

### Corrigido

- **Proximidade de deploy acusava um commit em sistema saudável.** O cenário
  `deploy-inofensivo` encontrou isto assim que passou a ser executado de verdade:
  com o sistema sem sintoma algum, o **commit** aparecia em primeiro lugar no
  ranking — acima de um serviço com regressão de latência medida.

  A causa foi uma isenção que eu mesmo abri: o teto relativo pulava commits,
  pelo raciocínio de que um commit só tem evidência de mudança por construção e
  capá-lo contra a própria evidência o zeraria sempre. O mecanismo estava certo e
  a conclusão errada. O teto do commit não é o dele, é o do **serviço onde ele
  foi implantado** — a acusação do commit deriva da do serviço.

  E as duas regras de proximidade passaram a ter a mesma corroboração. Só a de
  schema exigia sintoma; a de deploy seguia emitindo o finding, e o relatório
  saía com "Deployment próximo ao incidente, confiança alta" e **nenhum
  suspeito** — quem lê conclui que algo aconteceu.

- **O modo difícil passou a exercitar as duas regras de proximidade.** Nenhum dos
  cinco cenários originais coletava schema ou ingeria deployments, então
  `schema_change_proximity` e `deployment_proximity` atravessavam a suíte inteira
  sem serem executados uma única vez. O `sem-culpado` se declara "a rede de
  proteção de todo detector novo" e não protegia nenhuma das duas.

  `migracao-inofensiva` fechou a metade do schema; `deploy-inofensivo` fecha a do
  deploy. Ele é o `timeout-after-deploy` sem o defeito injetado: mesma
  proximidade temporal, mesmo commit correspondendo à `service.version`
  observada, sistema saudável. Os dois recusam passar por vacuidade — se a coleta
  ou a ingestão não registrar nada, o cenário falha, porque aí o silêncio não
  provaria coisa alguma.

  As duas regras também entraram na lista de proibidas do `sem-culpado`.

- **Migrations concorrentes contra o mesmo PostgreSQL abortavam.** `Migrate`
  fazia verificar-e-aplicar sem serialização, e os comandos da CLI migram ao
  iniciar. Num banco compartilhado — que é exatamente o cenário da ADR 0016 —
  dois processos subindo juntos passavam ambos pela verificação e executavam o
  mesmo DDL; um abortava com "relation already exists". Um lock consultivo em
  torno do ciclo resolve. Sem ele, o teste de seis processos simultâneos falha
  nas três tentativas.

- **A guarda de linha de base liberada existia só no SQLite.** O backend
  PostgreSQL caía num `unexpected end of JSON input` em vez da mensagem que
  explica o que houve. A bateria de conformidade não pegou porque nenhum caso
  recoletava depois de podar — a lacuna virou caso, e ele fica vermelho quando a
  guarda é removida.

- **Concorrência entre os backends passou a ser coberta.** A ADR 0016 registrava
  que o pool é de uma conexão no SQLite e de oito no PostgreSQL, e que a bateria
  não cobria isso. O produto ingere telemetria por HTTP concorrente no `serve`, e
  dois lotes chegando juntos são o caso normal. A bateria passou a exigir o mesmo
  resultado observável nos dois — nenhum sinal perdido, nenhum duplicado, e a
  idempotência valendo quando o mesmo lote chega por conexões simultâneas.

- **PostgreSQL como backend alternativo de armazenamento.** `storage.driver:
  postgres` no YAML, com a DSN vindo de `FAULTMAP_STORAGE_DSN` e nunca gravada em
  arquivo. **SQLite continua o padrão** — o produto se vende como binário único e
  local-first, e exigir um banco antes de rodar mataria isso.

  A peça central não é o adaptador, é a **bateria de conformidade**
  (`internal/storage/storagetest`): 45 casos que rodam idênticos contra os dois
  backends, cobrindo idempotência por `ON CONFLICT DO NOTHING`, janelas
  semiabertas, desempate estável de ID, leituras em lote, normalização para UTC e
  o alcance da retenção. Dois armazenamentos que divergem em silêncio seriam
  piores que um só.

  Duas armadilhas de dialeto que mock nenhum pegaria, ambas encontradas contra
  PostgreSQL real: `COUNT(*)` sobre subconsulta exige alias, e **`REAL` tem 8
  bytes no SQLite e 4 no PostgreSQL** — traduzir o nome ao pé da letra truncaria
  todo score de finding e reordenaria os suspeitos. As colunas de score usam
  `DOUBLE PRECISION`.

  Os 15 comandos da CLI escolhem o backend pela configuração. A religação foi
  mecânica de propósito: o alias do import passou a apontar para o seletor, e o
  driver entrou como segundo argumento de cada abertura — `Migrate` e cada
  `NewXRepository` mantiveram a forma exata que já tinham, que é o motivo de o
  seletor existir. Ver ADR 0016.

  As mensagens de erro deixaram de citar SQLite por nome: "fechar banco SQLite"
  mentiria para quem está rodando com PostgreSQL.

- **Sessão efêmera com `faultmap init --ephemeral`.** Cria o workspace no
  diretório temporário do sistema, para experimentar o produto sem deixar
  `faultmap.yaml`, `faultmap.db` e `faultmap-out/` no diretório de trabalho. O
  comando imprime o caminho e o `--config` a usar em seguida.

  O banco continua sendo um arquivo. Um banco em memória parecia a solução óbvia
  e não funciona aqui: cada comando é um processo separado e eles compartilham
  estado através do arquivo, então `ingest` gravaria numa base que o `diagnose`
  seguinte abriria vazia — respondendo "nenhuma anomalia encontrada" sem erro
  algum, que é a pior falha possível em um produto de diagnóstico.

  A limpeza é declarada como manual, e não prometida: o `init` termina antes de o
  workspace ser usado, então não existe momento em que ele pudesse apagar.

- **Retenção para os catálogos de schema.** Cada coleta grava o catálogo inteiro
  como JSON, e a pontuação pessimista da proximidade passou a cobrar coleta
  frequente — o incentivo que o produto criou encheria o disco de quem o segue.
  Medido: 200 coletas de uma base com 2.000 objetos, cinco dias de coleta
  horária, ocupavam **52,7 MB**.

  `faultmap retention apply` passa a liberar o conteúdo dos catálogos expirados.
  A mesma medição cai para **0,5 MB**, uma redução de 99,1%, com as 199 mudanças
  derivadas intactas.

  A limpeza esvazia `objects_json` em vez de remover a linha: a chave estrangeira
  de `schema_changes` tem `ON DELETE CASCADE`, e remover a coleta levaria junto a
  evidência que diagnósticos já gravados citam. E a coleta mais recente de cada
  base nunca é esvaziada, qualquer que seja a idade — ela é a linha de base da
  próxima comparação, e esvaziá-la faria o diff seguinte reportar todo objeto da
  base como recém-criado. Ver ADR 0015.

- **A frequência de coleta passou a se cobrar sozinha.** O score usava a ponta
  otimista do intervalo — o instante da coleta que revelou a mudança —, então
  coletar uma vez por dia rendia a mesma pontuação que coletar a cada minuto. Uma
  migração que rodou 22 horas antes do incidente era apresentada como "até 1 hora
  antes", com confiança alta, porque foi só na coleta seguinte que ela foi vista.

  A pontuação passa a sair da ponta pessimista e a inclusão continua na otimista:
  inclui com generosidade, pontua com cautela. Uma migração 1h antes vale 0,95
  com coleta de 5 em 5 minutos e 0,00 com coleta diária.

  A confiança também trocou de limiar. Ela caía quando o intervalo passava de 24
  horas, o que nunca acontecia na prática; agora cai quando o intervalo é maior
  que a proximidade afirmada — que é o que de fato torna a afirmação imprecisa.

  O resumo declara as duas pontas: "entre 1h e 7h antes do incidente", em vez de
  escolher a mais favorável.

- **Identificadores de catálogo curtos e ordenáveis por tempo.** Os IDs de
  coleta e de mudança eram concatenações de até 90 caracteres começando pelo
  nome do objeto, então ordenar por ID ordenava por tabela e nunca por tempo.

  O novo formato tem 26 caracteres: 48 bits de instante em milissegundos, base32
  de Crockford, seguidos de 80 bits de resumo do conteúdo. A ordenação
  lexicográfica passa a ser a cronológica, e o mesmo índice do SQLite serve às
  duas.

  Os 80 bits finais são resumo, e não entropia aleatória como no ULID e no
  UUIDv7, porque a persistência é idempotente por `ON CONFLICT(id) DO NOTHING` e
  o produto promete que a mesma entrada produz a mesma saída. Com entropia,
  recoletar o mesmo catálogo gravaria tudo de novo como se fossem mudanças
  inéditas. O que se perde é a imprevisibilidade — quem conhece o conteúdo
  recalcula o ID —, e isso não pesa aqui porque nenhum identificador do Faultmap
  é segredo nem serve de credencial.

  O alfabeto exclui I, L, O e U, então não há confusão entre 1 e I ou 0 e O para
  quem lê um ID em um relatório.

  Coletas feitas antes desta mudança mantêm os identificadores antigos e
  continuam sendo lidas normalmente — o ID é opaco e nunca é interpretado. Só a
  ordenação por ID mistura os dois formatos, com os antigos no fim. Como a
  funcionalidade nunca foi publicada, nenhuma instalação é afetada; um workspace
  local de desenvolvimento pode ser recriado se a ordenação incomodar.

- **Evidência de apoio pesava mais que a evidência que ela apoia.** Medindo na
  demo-shop, uma latência que regrediu de 4 ms para 12 ms valia 0.07 e a
  proximidade de migração valia 0.20 sozinha: o apoio superava em três vezes
  tudo o que deveria apenas sustentar.

  A causa está na janela. `1 - idade/24h` dá score máximo a qualquer mudança
  recente, e "houve migração há pouco" carrega muito menos informação em 24
  horas do que na janela de uma hora do deployment.

  A contribuição da classe de mudança passa a ser limitada à soma das evidências
  medidas do mesmo suspeito. O teto se ajusta sozinho — sintoma forte, o apoio
  pesa; sintoma marginal, o apoio fica marginal junto — e não inventa constante
  nova para ninguém defender depois.

  Sem sintoma algum o apoio vira zero e o serviço deixa de ser suspeito, o que
  fecha estruturalmente a exposição que `deployment_proximity` sempre teve:
  disparar sozinho em sistema saudável quando houve deploy na última hora.

  O commit fica de fora do teto. Ele só tem evidência de mudança por construção,
  então aplicá-lo ali zeraria todo commit e apagaria a acusação que é a
  informação mais acionável de um incidente causado por deploy.

  Custo assumido: uma migração que foi a causa única, com sintoma pequeno mas
  real, fica presa ao tamanho do sintoma. É a mesma troca conservadora de
  `exceedsSamplingNoise`.

- **O modo difícil não exercitava regra de proximidade de mudança nenhuma.** Os
  cinco cenários nunca coletam schema nem ingerem deployments, então tanto
  `schema_change_proximity` quanto `deployment_proximity` passavam por eles sem
  serem executados uma única vez. Uma regra que o modo difícil não consegue
  exercitar não está protegida por ele — foi por essa fresta que o falso positivo
  abaixo entrou.

  O cenário `migracao-inofensiva` fecha isso: aplica uma migração real antes da
  janela, mantém o sistema saudável e exige silêncio. Ele recusa passar por
  vacuidade — se a coleta não registrar mudança alguma, falha, porque aí o
  silêncio não provaria nada. `schema_change_proximity` também entrou na lista de
  regras proibidas do `sem-culpado`.

  O cenário inclui uma rodada de aquecimento descartada antes da baseline. Sem
  ela a primeira janela mede processo frio e a segunda mede processo quente, e a
  diferença aparece como regressão de latência real — o cenário passaria a medir
  o quanto a máquina está ocupada em vez do que se propõe a medir.

- **Uma migração inofensiva acusava um sistema saudável.** A regra de schema
  disparava sozinha: bastava existir uma mudança recente numa tabela que o
  serviço consulta. Contra a `demo-shop`, com 200 requisições e zero falhas, o
  produto apontava o `payment-service` com **confiança alta** porque alguém havia
  adicionado uma coluna que ninguém usa.

  É o falso positivo que o cenário `sem-culpado` do modo difícil existe para
  proibir — ele exige "Nenhuma anomalia determinística" em sistema saudável — e a
  regra passava por ele apenas porque nenhum cenário do modo difícil coleta
  schema.

  A mudança de schema passa a ser **evidência de apoio**, apresentada só quando o
  serviço já tem algum sintoma observado. Nenhum caso real se perde: uma migração
  que quebrou alguma coisa acende também erro, latência ou falha de banco. O que
  deixa de aparecer é exatamente a migração que não fez nada.

- **O detector de schema nunca dispararia contra instrumentação real.** Ele
  exigia o nome da base (`db.namespace` ou `db.name`) para ligar uma migração ao
  serviço. Medindo 199 spans de banco de uma aplicação instrumentada, todos
  traziam `db.collection.name` e **nenhum** trazia o nome da base: a regra ficaria
  permanentemente calada ali, sem erro e sem aviso, exatamente o defeito que as
  ADRs 0006 e 0011 registram.

  O vínculo passa a aceitar a tabela, que é uma ligação mais estreita que a base:
  uma migração em `payments` e um serviço que consulta `payments` diz mais do que
  "os dois usam o mesmo PostgreSQL". Sem nenhum dos dois observados o detector
  continua calado — só "ambos falam PostgreSQL" nunca liga nada a nada.

  Encontrado rodando contra a `demo-shop`; todos os testes passavam porque a
  telemetria deles fui eu quem escreveu, com `db.namespace` dentro.

- **"observado até 0s antes do incidente".** O arredondamento para minuto
  transformava uma migração de vinte e poucos segundos em "0s", que não informa
  nada. Apareceu na saída real da demo-shop.

- **Uma requisição grande demais encerrava a sessão MCP inteira.** O
  enquadramento por linha usava `bufio.Scanner`, que trata linha acima do teto
  como falha de leitura — e não como uma mensagem ruim. Um cliente defeituoso
  derrubava a investigação de quem estava do outro lado, exatamente a propriedade
  que o servidor prometia ter. Agora responde erro, descarta o excedente até a
  quebra de linha e segue atendendo.

- **Perder privilégio no banco viraria "o schema inteiro foi removido".**
  `information_schema` filtra por privilégio: um usuário sem `SELECT` nas tabelas
  recebe zero linhas **com sucesso**, sem erro nenhum. A comparação leria isso
  como remoção em massa, e no incidente seguinte todo serviço que fala com a base
  apareceria acusado por uma mudança produzida por uma permissão revogada.

  Medindo contra um PostgreSQL real, a coleta cega não chega vazia:
  `information_schema.columns` devolve nada enquanto `pg_indexes` continua
  devolvendo os índices. Uma guarda que olhasse só o total deixaria passar
  justamente esse caso. O critério passou a ser a ausência total de colunas, que
  é onde ele não tem falso positivo — tabela não existe sem coluna.

- **Objetos de schemas diferentes colidiam.** O nome era `tabela.coluna`, sem o
  schema, então `public.pedidos.valor` e `tenant_a.pedidos.valor` viravam a mesma
  chave: uma mudança mascarava a outra. Encontrado pela suíte de integração,
  quando dois testes criaram tabelas homônimas em schemas diferentes na mesma
  base — a situação normal de qualquer instalação multi-inquilino.

- **Restrições implícitas de NOT NULL apareceriam como migração fantasma.** O
  PostgreSQL as nomeia com OIDs — `2200_16385_1_not_null` —, e o OID muda quando
  a tabela é recriada: recriar uma tabela sem alterar nada acusaria uma restrição
  removida e outra adicionada durante um incidente. Descoberto rodando a coleta
  contra um PostgreSQL real, e não contra o mock.

- **Índice das ADRs estava sem a 0012 e a 0013.** Os arquivos existiam e não
  apareciam na tabela de `docs/adr/README.md`.

## v0.5.0 — 2026-08-20

A primeira release moldada por um **piloto cego**: o produto foi levado a uma
aplicação que não é nossa, com uma pessoa investigando sem saber qual falha havia
sido injetada. Tudo aqui saiu de tentar usar a ferramenta, não de percorrer a
lista de tarefas.

### Corrigido

- **`privacy.blocked_attributes` do YAML apagava as proteções padrão.** A
  configuração é decodificada sobre os valores padrão, e isso funciona campo a
  campo — mas uma sequência YAML substitui a lista inteira. Declarar o
  vocabulário sensível do próprio negócio removia em silêncio o bloqueio de SQL
  bruto (`db.statement`, `db.query.text`) e de caminho de arquivo de origem
  (`code.file.path`, `code.filepath`).

  O efeito era o oposto da intenção: quem escrevia a seção estava protegendo
  mais, e acabava protegendo menos. A lista agora **soma** aos padrões. A prova é
  no disco, não na estrutura de dados: ingerir uma captura real de psycopg2 com
  uma lista própria gravava **49 sinais com SQL bruto**; com a união, zero. Ver
  ADR 0012.

  Consequência aceita: não é mais possível encolher a lista padrão.

- **Recusa de logs em protobuf era invisível dos dois lados.** O protocolo OTLP
  exige respostas estáveis e sem detalhes internos, e o processo não registrava
  nada — quem exportava tudo em protobuf via os traces entrarem e os logs
  desaparecerem, sem nada que ligasse os dois fatos. A causa passa a ir ao
  terminal do operador, uma vez por motivo distinto, sem alterar o que vai pela
  rede.

- **README desatualizado em três pontos.** Afirmava que logs não eram recebidos,
  usava `otlp_http` como nome do exportador do Collector — que não existe; o
  correto é `otlphttp`, confirmado rodando `validate` na imagem oficial — e não
  registrava que o **Collector é obrigatório para logs**: os SDKs de aplicação
  não exportam OTLP em JSON, e o SDK Python recusa `http/json` na inicialização.

### Adicionado

- **Cada evidência diz o que aquele padrão costuma significar.** Nasceu do
  resultado mais incômodo do piloto: diante de "a duração p95 das operações
  duckdb aumentou de 3,54 ms para 150,73 ms" — medida correta, cuja causa real
  era exatamente atraso nas consultas — a pessoa que investigava registrou como
  hipótese *"usuário incompleto ou SQL injection"*. Duas hipóteses sobre os dados
  estarem errados, quando a medida falava sobre tempo.

  Nenhum teste nosso poderia ter encontrado isso, porque todos verificam se o
  número está certo, e o número estava certo. O relatório era um termômetro:
  mostrava a febre sem sugerir onde procurar.

  Cada frase apresenta **mais de uma possibilidade**, e um teste rejeita regra com
  causa única: uma só leria como veredito, e o produto não afirma causalidade. A
  ordem de renderização mantém a ressalva com a última palavra. Um teste enumera
  as doze regras e falha quando alguma não tem frase. Ver ADR 0013.

- **Kit do piloto cego** em `examples/pilot/`: protocolo, configurações,
  formulário do investigador, resultado e os roteiros usados. Inclui o
  `pilot-gateway`, um proxy reverso instrumentado que existe para haver um
  segundo serviço no trace — com uma aplicação só, o ranking não tem entre quem
  escolher.

### Verificação

- `make verify`, matriz E2E 6 de 6 e modo difícil 5 de 5, sobre o código exato
  publicado.
- **Piloto cego contra aplicação de terceiro: top-1 em 3 de 3** cenários de
  ranking. No caso difícil, culpado e vítima ficaram lentos quase igual — 326 ms
  contra 329 ms — e o desempate veio da evidência de banco, que só o culpado
  podia ter. A contraprova, com a lentidão nascendo no proxy, inverteu a
  resposta.
- `collector.yaml` do kit validado pela imagem oficial do OpenTelemetry
  Collector.

### Limitações conhecidas

- **Dois dos doze detectores dispararam em telemetria real.** Os outros dez
  seguem exercitados apenas por cenários que nós mesmos desenhamos. Isso não os
  torna errados, e não é o mesmo que cobertura.
- Não sabemos se a frase de causas comuns muda a hipótese que uma pessoa formula.
  Sabemos que o relatório anterior levou a uma hipótese de natureza errada.
- Nenhum incidente **inesperado**, que ninguém tenha planejado, passou pelo
  produto.
- O modo difícil mede latência de processos reais e **exige a máquina em
  repouso**: sob contenção, `sem-culpado` acusou 9 ms para 64 ms e falhou;
  isolado, passou três vezes com silêncio completo.

## v0.4.0 — 2026-08-14

O diagnóstico passa a receber **logs** e a acusar o **commit implantado** como
suspeito próprio. É mudança de comportamento: o ranking agrupa por sujeito, e
um commit pode aparecer ao lado dos serviços.

### Adicionado

- **Ingestão de logs OTLP.** O receiver passa a atender `POST /v1/logs`, e o
  detector `log_correlation` liga o crescimento de logs de erro às requisições
  que falharam no mesmo trace. O peso `log_correlation` estava configurado desde
  o início sem ser usado por regra nenhuma; agora os pesos somam `1.00`.

  **O texto da mensagem nunca é armazenado.** O Faultmap guarda severidade,
  instante, correlação de trace e atributos permitidos — quem precisa ler a
  mensagem vai ao sistema de logs de origem. O corpo é usado apenas para compor
  o identificador do registro, o que impede que reenvios dupliquem sinais, e é
  descartado em seguida. Ver ADR 0011.

  O detector exige **correlação, não volume**: crescimento de logs de erro sem
  ligação com requisição que falhou pode ser migração ou rotina noturna.

  Apenas o mapeamento JSON é aceito. Aceitar protobuf sem ter visto uma
  instrumentação real exportando por ele repetiria o erro que já custou três
  releases publicadas cegas.

- **`code.file.path` bloqueado por padrão.** A captura de telemetria real
  mostrou que o SDK de logs anexa o caminho absoluto do arquivo de origem a cada
  registro — a mesma classe de informação do stacktrace, já descartado.
  `code.function.name` e `code.line.number` seguem permitidos, porque ajudam a
  investigar sem expor a estrutura de diretórios.


### Verificação

- Matriz E2E: 6 de 6. Modo difícil: 5 de 5, sobre o código exato publicado.
- Telemetria real do SDK oficial de logs ingerida ponta a ponta: severidade,
  instante e correlação de trace preservados; nenhum texto de mensagem em disco.
- O commit aparece no ranking em telemetria real, com deployment importado, sem
  deslocar o serviço que acumula mais evidência.
- Quatro linguagens exercitadas contra instrumentação de terceiros — Python,
  Node, Go e Java — sem nenhuma cegueira de convenção nova.

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
