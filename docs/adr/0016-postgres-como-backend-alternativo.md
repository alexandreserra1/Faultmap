# ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada

- Status: aceito
- Data: 2026-09-13

## Contexto

O Faultmap persiste tudo em SQLite: telemetria, snapshots de diagnóstico,
commits, deployments e coletas de catálogo. A escolha é boa para o produto que
está sendo vendido — binário único, banco local, nada para instalar — e o
[ADR 0003](0003-retencao-preserva-snapshots-de-diagnostico.md) já se apoia nela
ao limitar a retenção a `signals`.

Ela tem um limite conhecido: o SQLite admite um escritor por vez. Enquanto o
Faultmap roda na máquina de quem investiga, isso não aparece. Quando um time
passa a compartilhar a mesma instância — ingestão OTLP contínua de vários
serviços, várias pessoas investigando — o escritor único vira o gargalo, e a
telemetria deixa de caber confortavelmente em um arquivo.

A alternativa óbvia seria trocar o SQLite pelo PostgreSQL. Isso mataria o
produto: a promessa de instalar um binário e diagnosticar um incidente em
seguida não sobrevive a "primeiro suba um servidor de banco".

O risco real de ter dois backends não é o custo de escrever o segundo. São 2.074
linhas de SQL, e a tradução do dialeto é mecânica: 117 placeholders, 13 tipos de
data, três PRAGMAs que somem. O risco é os dois **divergirem em silêncio**. Uma
janela que seleciona um sinal a mais em um banco, uma ordenação que desempata
diferente, um instante que volta em outro fuso — e a mesma investigação passa a
acusar serviços diferentes conforme onde o banco está. O produto se vende como
diagnóstico determinístico; determinismo que depende do backend não é
determinismo.

## Decisão

**O SQLite continua sendo o padrão.** `storage.driver` aceita `postgres` como
alternativa explícita. Um `storage.driver` ausente é lido como `sqlite`, para
que uma configuração escrita antes deste campo continue abrindo o banco local.

**A equivalência é cobrada por uma bateria compartilhada**, em
`internal/storage/storagetest`. Ela recebe uma fábrica de repositórios e roda os
mesmos 36 casos contra os dois backends: idempotência do `ON CONFLICT DO
NOTHING`, janelas semiabertas, desempate estável por ID, leituras em lote sem
N+1, normalização em UTC, preservação de snapshots pela retenção e as recusas de
entrada. Não é documentação da equivalência; é o mecanismo que a mantém.

**A DSN do PostgreSQL vem da variável `FAULTMAP_STORAGE_DSN` e nunca do
`faultmap.yaml`.** O `init` promete um arquivo de configuração sem credenciais,
e um arquivo que ganha usuário e senha tende a ir para o controle de versão sem
que ninguém perceba. É o mesmo raciocínio que o
[ADR 0014](0014-schema-guarda-identificadores-nao-expressoes.md) aplica à coleta
de catálogo, onde a conexão com a base observada também não é gravada.

**As migrations usam os mesmos dez números de versão do SQLite**, ainda que o
caminho de algumas seja diferente. A migration 4 reescreve duas tabelas no
SQLite porque aquele banco não sabe adicionar uma foreign key a uma tabela
existente; no PostgreSQL um `ALTER TABLE ADD CONSTRAINT` basta. Numerar igual
permite comparar o `schema_migrations` de dois bancos ao investigar uma
divergência.

## Consequências

- Duas traduções de dialeto não são mecânicas e estão comentadas no código: o
  `COUNT(*)` sobre subconsulta exige apelido no PostgreSQL e o dispensa no
  SQLite, e `REAL` significa 8 bytes em um e 4 no outro — traduzir o nome ao pé
  da letra truncaria o score de cada finding e faria os dois backends ordenarem
  os mesmos suspeitos de forma diferente. As colunas de score usam
  `DOUBLE PRECISION`.
- O `TIMESTAMPTZ` volta no fuso da sessão, que é o do servidor. Cada leitura
  normaliza para UTC antes de devolver ao domínio. Sem isso, a mesma janela de
  incidente selecionaria conjuntos diferentes conforme onde o banco está
  hospedado — a bateria cobra o fuso, não só o instante.
- A bateria só roda contra PostgreSQL quando `FAULTMAP_TEST_PG_DSN` está
  definida, e se ignora sem ela. A suíte continua passando em uma máquina sem
  Docker, ao custo de que uma regressão exclusiva do PostgreSQL só aparece em
  `make test-integration`.
- `retention apply` usa a mesma consulta nos dois backends, sem
  `FOR UPDATE SKIP LOCKED`. É um comando explícito de operação, não uma rotina
  concorrente; duas execuções simultâneas no PostgreSQL podem se bloquear. Se a
  retenção passar a rodar automaticamente, isso precisa ser revisto.
- A escolha do backend vive em `internal/storage/bootstrap`, e não espalhada
  pelos comandos. Sem ele, cada um dos quinze comandos da CLI teria a mesma
  ramificação em volta da abertura do pool e de cada repositório.

## Adendo — a divergência de concorrência passou a ser coberta

Esta decisão registrava que o pool é de uma conexão no SQLite e de oito no
PostgreSQL, e que a bateria não cobria concorrência. Não coberta é onde defeito
mora: o produto ingere telemetria por HTTP concorrente no `serve`, e dois lotes
chegando juntos são o caso normal, não o excepcional.

A bateria passou a exigir que o **resultado observável** seja o mesmo nos dois —
nenhum sinal perdido, nenhum duplicado, e a idempotência do `ON CONFLICT DO
NOTHING` valendo também quando o mesmo lote chega por conexões simultâneas, que
é o retry de ingestão. Como cada banco serializa a escrita por baixo continua
diferente, e continua sendo detalhe de implementação.

A retenção segue sem `FOR UPDATE SKIP LOCKED`: duas execuções simultâneas de
`retention apply` podem se bloquear no PostgreSQL. Continua deliberado — SKIP
LOCKED faria os dois backends escolherem lotes diferentes — e continua a ser
revisitado se a retenção virar automática.
