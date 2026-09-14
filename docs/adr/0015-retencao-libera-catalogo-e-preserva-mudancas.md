# ADR 0015 — A retenção libera o catálogo e preserva as mudanças

- Status: aceito
- Data: 2026-09-13

## Contexto

A [ADR 0003](0003-retencao-preserva-snapshots-de-diagnostico.md) limitou a
retenção à tabela `signals` e registrou a condição para revisitar a decisão:

> snapshots crescem devagar e ainda não têm política própria. Se isso mudar, uma
> retenção separada para snapshots deve ser discutida em um novo ADR.

Isso mudou, e não por causa dos snapshots de diagnóstico que a 0003 tratava. A
coleta de catálogo da [ADR 0014](0014-schema-guarda-identificadores-nao-expressoes.md)
criou uma tabela que ela nunca considerou, e a pontuação pessimista da
proximidade passou a **cobrar coleta frequente**: quanto mais espaçadas as
coletas, menor o score da evidência.

O incentivo produziu o problema. Cada coleta grava o catálogo inteiro como JSON:

| Base | Coleta horária | Coleta de 5 em 5 minutos |
| --- | --- | --- |
| 500 objetos | 0,5 GB/ano | 6,4 GB/ano |
| 5.000 objetos | 5,3 GB/ano | 64 GB/ano |

Num produto que se distribui como SQLite local, 64 GB por ano é o produto
enchendo o disco de quem segue a orientação que ele mesmo dá.

## Decisão

**A limpeza esvazia `objects_json`; não remove a linha da coleta.** A chave
estrangeira de `schema_changes` aponta para a coleta com `ON DELETE CASCADE`, e
remover a linha levaria junto as mudanças derivadas dela — que são a evidência
citada por diagnósticos já gravados. Apagá-las tiraria do relatório aquilo que
ele afirma. Esvaziar recupera praticamente todo o espaço, porque o peso está no
JSON e não na linha, e preserva o registro de que a coleta aconteceu.

**As mudanças nunca são tocadas.** Elas são minúsculas — nome de objeto, tipo de
mudança e o intervalo observado — e é sobre elas que o detector trabalha. A
assimetria é a mesma da ADR 0003: o volume mora no dado bruto, a evidência mora
no derivado.

**A coleta mais recente de cada base nunca é esvaziada, qualquer que seja a
idade dela.** Ela é a linha de base da próxima comparação. Esvaziá-la faria o
diff seguinte enxergar um catálogo vazio e reportar **todo objeto da base como
recém-criado** — uma migração inventada em cada tabela, no próximo incidente.

**Comparar contra uma linha de base liberada é erro, não catálogo vazio.** A
regra acima impede o caso; se ele acontecer mesmo assim — banco editado à mão,
defeito futuro —, a coleta falha dizendo o que houve. Tratar como catálogo vazio
inventaria a migração descrita acima; tratar como ausência de linha de base
gravaria uma foto nova sem comparar, escondendo a migração real.

## Consequências

- O crescimento passa a ser dominado pelas mudanças, que são proporcionais ao
  que de fato muda no schema, e não à frequência da coleta. Coletar de 5 em 5
  minutos deixa de custar disco a quem não migra nada.
- Um catálogo liberado não pode ser reexaminado. Quem quiser auditar como o
  schema estava em uma data antiga precisa de outra fonte — o Faultmap guarda o
  que mudou, não o retrato de cada instante.
- A limpeza avança nos mesmos lotes da telemetria e é idempotente: um catálogo já
  liberado não entra no lote seguinte, então repetir o comando relata zero em vez
  de contar de novo o mesmo trabalho.
- A política é opcional na camada de aplicação. Quem nunca coletou catálogo não
  tem o que liberar, e um podador ausente não pode derrubar a limpeza de
  telemetria.
