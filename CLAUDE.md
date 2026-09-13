# Faultmap

## Antes de mudar código, consulte o grafo

Este repositório tem um grafo de conhecimento em `graphify-out/`: 1.770 nós e
5.100 arestas em duas camadas ligadas entre si — o código Go extraído por AST, e
a razão por trás dele, vinda das 14 ADRs, do README, do CHANGELOG e da spec do
MVP.

**Consulte-o antes de alterar assinatura ou comportamento de qualquer função ou
tipo, e antes de escrever o teste vermelho de um ciclo TDD.** A skill
`consultar-grafo` (em `.claude/skills/consultar-grafo/`) explica como e o que
perguntar.

```bash
graphify affected "Rank()" --depth 2   # quem quebra se eu mudar isto
graphify query "quem chama DetectSchemaChangeProximity?"
graphify god-nodes --top 10            # abstrações centrais
graphify update                        # depois de mexer no código
```

Isto existe porque três defeitos desta base nasceram de mudar uma coisa sem
enxergar quem dependia dela — inclusive a lista de convenções do OpenTelemetry,
que ficou duplicada entre detectores e renderizadores duas vezes seguidas.
Grep encontra o nome; o grafo encontra quem depende dele.

As ADRs estão no grafo e alcançam o código que governam: `graphify path "ADR
0014..." "CheckCollection()"` responde em um salto. **Leia a ADR antes de mudar
o código que ela decide** — mudar sem ler é desfazer uma decisão sem saber que
ela existiu.

As arestas de código vêm de AST e são fatos. As que ligam documento a símbolo
vêm de leitura: `EXTRACTED` quando a ADR cita o símbolo pelo nome, `INFERRED`
quando é interpretação. Confira as segundas antes de tratá-las como contrato.

## Verificação

`make verify` antes de qualquer commit: encadeia formatação, `go vet`, a suíte e
o detector de corrida. Nunca filtre a saída de `go test` com `grep` — isso
descarta o código de saída e inverte o resultado.

Para mudanças em detecção ou ranking, rode também:

- `make demo-test-e2e` — verifica se o produto **acusa** o serviço certo;
- `make demo-test-hard` — verifica se ele **se cala** quando não há regressão;
- `make test-integration` — sobe um PostgreSQL descartável para o que mock não prova.
