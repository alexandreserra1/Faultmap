---
name: consultar-grafo
description: Use ANTES de alterar qualquer código deste repositório e antes de escrever um teste em TDD — consulta o grafo de conhecimento em graphify-out/ para descobrir quem chama a função, quem a implementa e que decisão arquitetural a governa, evitando mudança que quebra chamador invisível ou que contraria uma ADR.
---

# Consultar o grafo antes de mudar

O grafo em `graphify-out/graph.json` tem 1.565 nós e 4.665 arestas extraídas por
AST dos arquivos Go: funções, tipos, arquivos e as ligações entre eles —
chamadas, referências de tipo, implementações de interface.

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
4. **Existe ADR sobre isso?** O grafo cobre código, não os 14 ADRs em
   `docs/adr/`. Se o nó estiver em ranking, privacidade, retenção ou detecção,
   leia a ADR correspondente antes — ela registra por que a decisão é o que é.

## Limites que você precisa saber

- **Só código Go.** Os 47 arquivos de documentação, incluindo as ADRs, não estão
  no grafo. Esta é a lacuna que o item 4 acima cobre à mão.
- **1.144 arestas de ponta solta** (19%): referências a `time.Duration`, cobra,
  otel e stdlib, que não viram nó por estarem fora do corpus. A estrutura interna
  está íntegra; ausência de aresta para fora do projeto não significa ausência de
  dependência.
- **Grafo não dirigido.** "A se liga a B" não diz quem chama quem; confirme o
  sentido lendo o `source_location` que a consulta devolve.
- **Uma foto, não um espelho.** Se o último `graphify update` foi antes das suas
  mudanças, o grafo mente com confiança. Atualize depois de mexer.
