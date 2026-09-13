---
name: consultar-grafo
description: Use ANTES de alterar qualquer código deste repositório e antes de escrever um teste em TDD — consulta o grafo de conhecimento em graphify-out/ para descobrir quem chama a função, quem a implementa e que decisão arquitetural a governa, evitando mudança que quebra chamador invisível ou que contraria uma ADR.
---

# Consultar o grafo antes de mudar

O grafo em `graphify-out/graph.json` tem 1.770 nós e 5.100 arestas. São duas
camadas ligadas entre si:

- **código** (1.565 nós), extraído por AST dos arquivos Go: funções, tipos,
  arquivos, chamadas, referências de tipo, implementações de interface;
- **razão** (205 nós), extraída das 14 ADRs, do README, do CHANGELOG, da spec do
  MVP, dos cenários da demo e do diagrama de arquitetura — ligada ao código por
  cerca de 90 arestas `rationale_for` e `references`.

A segunda camada é a que responde *por que* o código é assim. A ADR 0014 alcança
`CheckCollection()` em um salto; `exceedsSamplingNoise()` mostra seus chamadores
Go e a decisão de ranking que o cita.

## Por que isso existe

Três defeitos desta base de código nasceram de mudar uma coisa sem enxergar
quem dependia dela:

- a lista de convenções do OpenTelemetry ficou duplicada entre detectores e
  renderizadores, **duas vezes**, e a tela passou a discordar da análise sobre o
  mesmo span;
- a regra de schema exigia `db.namespace` e teria ficado permanentemente calada
  contra instrumentação real, porque ninguém olhou o que a telemetria de fato
  emite;
- dois testes afirmavam comportamento que outra camada já havia tornado errado.

Grep encontra o nome. O grafo encontra **quem depende dele**.

## Quando usar

Antes de:

- alterar assinatura, comportamento ou remover qualquer função ou tipo;
- escrever o teste vermelho de um ciclo TDD — para saber o que o comportamento
  atual sustenta antes de decidir o que ele deveria fazer;
- mover código entre pacotes ou extrair uma abstração.

Não precisa para: corrigir texto, comentário, documentação ou formatação.

## Como usar

O grafo já está construído. **Não reconstrua** para responder uma pergunta.

O comando que responde a pergunta que importa é `affected`: travessia reversa,
devolvendo quem quebra se você mexer, com arquivo e linha.

```bash
graphify affected "Rank()" --depth 2        # QUEM QUEBRA se eu mudar isto
graphify query "quem chama DetectSchemaChangeProximity?"
graphify path "SchemaChange" "Rank"         # caminho mais curto entre dois conceitos
graphify explain "PersistedDiagnosis"       # o que é e a que se liga
graphify god-nodes --top 10                 # abstrações centrais; mexer nelas custa caro
```

Depois de mudanças no código, atualize incrementalmente:

```bash
graphify update
```

## O que perguntar, em ordem

1. **Quem quebra se eu mudar isto?** `graphify affected "X"`. Cada resultado é um
   contrato que a mudança pode romper — inclusive os testes, que aparecem na
   lista e dizem quais precisam ser revistos antes de escrever o vermelho novo.
2. **Quem implementa a interface?** Mudar o método atinge todos os dublês de
   teste também.
3. **A que comunidade pertence?** As 18 comunidades nomeadas dizem qual parte do
   produto está sendo tocada — se a mudança atravessa fronteira de comunidade,
   ela é maior do que parece.
4. **Que decisão governa isto?** `graphify path "<nó>" "<ADR>"`, ou procure na
   saída do `affected` os nós de razão. Quatorze ADRs estão no grafo, cada uma
   carregando o porquê, a troca aceita e o custo assumido. Mudar código sem ler a
   ADR que o governa é desfazer uma decisão sem saber que ela existiu.

## Limites que você precisa saber

- **Camada de razão é inferida, não estrutural.** As arestas de código vêm de
  AST e são fatos; as que ligam documento a símbolo foram extraídas por leitura e
  vêm marcadas `EXTRACTED` só quando a ADR cita o símbolo pelo nome. As
  `INFERRED` são interpretação — confira antes de tratá-las como contrato.
- **1.144 arestas de ponta solta** (19%): referências a `time.Duration`, cobra,
  otel e stdlib, que não viram nó por estarem fora do corpus. A estrutura interna
  está íntegra; ausência de aresta para fora do projeto não significa ausência de
  dependência.
- **Grafo não dirigido.** "A se liga a B" não diz quem chama quem; confirme o
  sentido lendo o `source_location` que a consulta devolve.
- **Uma foto, não um espelho.** Se o último `graphify update` foi antes das suas
  mudanças, o grafo mente com confiança. Atualize depois de mexer.
