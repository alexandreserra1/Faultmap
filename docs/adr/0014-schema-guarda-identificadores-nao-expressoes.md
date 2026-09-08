# ADR 0014 — A coleta de schema guarda identificadores, não expressões

- Status: aceito
- Data: 2026-09-03

## Contexto

O texto de causas comuns de `deployment_proximity` já dizia que aquele padrão
"costuma vir da mudança de código em si, mas também de migração de schema que
acompanhou o deploy". O de `database_error` dizia "migração de schema
incompatível com o código em execução". O produto afirmava, em duas regras, que
mudança de schema é uma explicação frequente — e não observava nenhuma. Quem
investigava recebia a hipótese e tinha que sair do Faultmap para verificá-la.

A forma óbvia de capturar DDL seria a replicação lógica do PostgreSQL, que é o
caminho usado por ferramentas de CDC. Ela não serve aqui por uma razão que não é
de custo: **a replicação lógica não decodifica DDL**. Capturar `ALTER TABLE` por
esse caminho exigiria instalar event triggers no banco de quem usa o produto, ou
seja, escrever no banco alheio para poder lê-lo.

Restou comparar duas leituras do catálogo. E aí aparece a questão de
privacidade, porque o catálogo não contém só nomes.

## Decisão

**A coleta é somente leitura e compara snapshots.** Cada execução de
`faultmap ingest schema` lê o catálogo, compara com a leitura anterior da mesma
base e grava as diferenças. Nenhuma escrita no banco observado, nenhum slot de
replicação, nenhuma extensão, nenhum privilégio além de `SELECT` no catálogo.

**Identificador e tipo entram; expressão não.** Nome de tabela, nome de coluna,
nome de índice, nome de constraint e tipo de dado são metadados, da mesma
categoria dos atributos que a [ADR 0011](0011-logs-guardam-apenas-metadados.md)
já permite guardar: descrevem a forma do sistema, não o que passou por ele.

`DEFAULT` e as expressões de `CHECK` são outra coisa. `DEFAULT 'acme-corp'`,
`CHECK (cpf ~ '^[0-9]{11}$')` e `DEFAULT 'https://interno.empresa/callback'`
carregam valor, regra de negócio e endereço interno — exatamente o que a
[ADR 0008](0008-politica-de-privacidade-aplicada-na-ingestao.md) impede de
entrar por telemetria. Entrar por outra porta seria contornar a própria política.

O Faultmap registra **que** a expressão mudou, e não qual ela é. A evidência diz
"o valor padrão da coluna `clientes.plano` mudou" e não mostra os dois valores.
Quem investiga tem o ponteiro exato para olhar no banco, que é onde a informação
já está.

**A mudança é evidência de apoio, não acusação isolada.** O finding só é
apresentado quando o serviço já tem algum sintoma observado na janela. Uma
migração sem efeito observável não é evidência de nada: sem esta regra, o
produto apontava com confiança alta um serviço em que 200 requisições passaram
sem uma única falha, porque uma coluna não usada havia sido adicionada.

Nenhum caso real se perde. Uma migração que quebrou alguma coisa acende também
erro, latência ou falha de banco — o índice removido aparece como
`database_latency_delta`, a coluna incompatível como `error_rate_delta`. O que
deixa de aparecer é a migração que não fez nada, que é ruído.

**A tabela é guardada junto do objeto.** O vínculo entre uma migração e um
serviço vem da telemetria, e a telemetria real frequentemente não nomeia a base:
medindo 199 spans de banco de uma aplicação instrumentada, todos traziam
`db.collection.name` e nenhum trazia `db.namespace`. Exigir o nome da base
deixaria a regra permanentemente calada ali.

A tabela é também o vínculo mais estreito: uma migração em `payments` e um
serviço que consulta `payments` é uma ligação mais forte do que "os dois usam o
mesmo PostgreSQL". O nome da base entra quando a telemetria o traz; nenhum dos
dois observados significa silêncio.

**O nome do objeto carrega o schema.** Um banco PostgreSQL quase nunca tem um
schema só: há `public` mais os da aplicação, e instalações multi-inquilino usam
um schema por cliente. Objetos são identificados como `schema.tabela.coluna` e
`schema.índice`. Sem isso, `public.pedidos.valor` e `tenant_a.pedidos.valor`
seriam a mesma chave na comparação, e uma mudança em um schema mascararia a do
outro — dropar uma tabela que existe com o mesmo nome em outro schema apareceria
como "nada mudou".

**Uma coleta sem colunas é recusada, não comparada.** Esta é a única falha da
coleta que o PostgreSQL não reporta como erro: `information_schema` filtra por
privilégio, então um usuário que perdeu `SELECT` nas tabelas recebe zero linhas
com sucesso. Medindo contra um servidor real, a coleta de um usuário sem
privilégio não chega vazia — `information_schema.columns` devolve nada enquanto
`pg_indexes` continua devolvendo os índices. Ela chega **sem colunas**.

O critério é a ausência total de colunas porque é onde ele não tem falso
positivo: tabela não existe sem coluna, e índice não existe sem tabela. Um
catálogo que tinha colunas e passa a não ter nenhuma, ainda exibindo outros
objetos, descreve um estado que não existe em banco nenhum — só em uma leitura
incompleta. As outras classes ficam de fora de propósito: dropar o único índice
de uma base é migração legítima, e é justamente o incidente que esta
funcionalidade existe para explicar.

**A credencial não é persistida.** A DSN vem por flag ou variável de ambiente e
nunca é gravada no `faultmap.yaml`, que o `init` promete criar sem tokens ou
credenciais. O nome da base é gravado; a forma de conectar nela, não.

## Consequências

- O instante de uma mudança é um **intervalo entre duas coletas**, não um
  instante. Isso é pior que o dado que a replicação lógica daria e é declarado
  como limitação em todo finding produzido, em vez de escondido. Coletas mais
  frequentes estreitam o intervalo; a decisão sobre a frequência é de quem opera.
- Uma mudança feita e revertida entre duas coletas é invisível. É a troca
  aceita: o produto prefere não ver a inventar um instante que não mediu.
- A comparação vê o efeito, não o comando. `ALTER TABLE ... RENAME COLUMN`
  aparece como uma coluna removida e outra adicionada, porque é isso que duas
  fotos do catálogo mostram. A evidência descreve o que foi observado, e não
  supõe o DDL que produziu aquilo.
- A regra entra na mesma classe de peso de `deployment_proximity`, e não em uma
  classe nova. O teto por classe da [ADR 0010](0010-teto-por-classe-de-peso-no-ranking.md)
  passa então a impedir que deploy e migração somem duas vezes quando são o mesmo
  evento — que é o caso comum, já que a migração normalmente acompanha o deploy.
- Perder privilégio no meio de uma operação faz o comando falhar com uma
  mensagem que aponta a causa provável, em vez de gravar "todas as colunas foram
  removidas" e envenenar todo diagnóstico seguinte. O custo é que um `DROP` de
  todas as tabelas também é recusado; é raro e visível por outros meios.
- O detector só acusa serviço que fala com aquela base, verificado pelos spans de
  banco da janela. Sem esse vínculo, seria correlação puramente temporal:
  qualquer migração em qualquer base acusaria qualquer serviço.
