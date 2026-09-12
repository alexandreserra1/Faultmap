# Faultmap

## Antes de mudar código, consulte o grafo

Este repositório tem um grafo de conhecimento em `graphify-out/`: 1.565 nós e
4.665 arestas extraídas por AST dos arquivos Go — funções, tipos, arquivos e as
ligações entre eles.

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

O grafo cobre **apenas código Go**. As 14 ADRs em `docs/adr/` ficam de fora e
precisam ser lidas à mão quando a mudança toca ranking, privacidade, retenção ou
detecção — elas registram por que cada decisão é o que é.

## Verificação

`make verify` antes de qualquer commit: encadeia formatação, `go vet`, a suíte e
o detector de corrida. Nunca filtre a saída de `go test` com `grep` — isso
descarta o código de saída e inverte o resultado.

Para mudanças em detecção ou ranking, rode também:

- `make demo-test-e2e` — verifica se o produto **acusa** o serviço certo;
- `make demo-test-hard` — verifica se ele **se cala** quando não há regressão;
- `make test-integration` — sobe um PostgreSQL descartável para o que mock não prova.
