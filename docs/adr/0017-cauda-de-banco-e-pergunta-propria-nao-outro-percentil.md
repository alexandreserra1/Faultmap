# ADR 0017 — A cauda do banco é uma pergunta própria, não outro percentil

- Status: aceito
- Data: 2026-09-19

## Contexto

O detector `database_latency_delta` compara a duração p95 das operações de banco
entre as duas janelas. Ele nasceu de um caso real — um banco que ficou catorze
vezes mais lento sem falhar nenhuma vez — e resolve bem a degradação que atinge
a janela inteira.

Um piloto cego mostrou que ele não enxerga outra forma de falha, e a omissão foi
medida, não suposta. Contra um `ACCESS EXCLUSIVE` real em `payments`:

| | |
|---|---|
| operações de banco na janela | 120 |
| marcadas como erro | 0 |
| p50 | 0,5 ms |
| p90 | 1,51 ms |
| **p95** | **21,11 ms** |
| p97 | 1.999,35 ms |
| máximo | 1.999,60 ms |

Cinco operações esperaram dois segundos. Nenhuma falhou, logo todas entraram na
conta do detector. Ele ficou calado mesmo assim: a cauda inteira vive acima do
p95, por uma observação.

O investigador — neste caso o próprio modelo conduzindo o piloto — leu a ausência
do sinal de banco como "o banco respondeu bem, logo a espera é antes dele", e
acusou saturação de pool. A leitura errada foi induzida pelo produto: ele não
disse que o banco estava bem, disse que o p95 não se moveu. São coisas
diferentes, e só uma delas estava na tela.

## Decisão

Uma regra nova, `database_latency_tail`, que pergunta **"alguém esperou muito
além do normal?"** em vez de "a janela inteira ficou mais lenta?".

Uma operação entra na cauda quando passa de `max(p95 da baseline × 20, 100 ms)`.
A regra acusa quando ao menos três operações entram na cauda e a proporção do
incidente é mais que o dobro da proporção da baseline.

Os três números têm motivo, e não são o resultado de ajustar até passar:

- **20×** porque a regra irmã aceita o dobro para a janela inteira, e aqui a
  afirmação se apoia num punhado de observações: o critério precisa ser mais
  severo na mesma proporção em que a amostra é menor.
- **100 ms** porque a razão sozinha não protege em durações pequenas. Um banco
  local trabalha em frações de milissegundo, onde vinte vezes ainda são dez
  milissegundos — e este projeto registrou quatro falsos positivos do detector de
  p95 exatamente nessa escala, todos com valor de incidente abaixo de vinte
  milissegundos.
- **três operações** porque uma é anedota. É o mesmo cuidado que
  `minimumSampleSize` já aplica no pacote.

A comparação é por proporção, e não por contagem, porque as duas janelas podem
ter volumes diferentes e contar cru faria a janela maior parecer pior por ser
maior. E a exigência de que a cauda do incidente seja desproporcional à da
baseline mantém a regra comparativa como todas as outras: uma cauda que já
existia descreve o sistema, não o incidente.

A regra divide a classe de peso `database_evidence` com as outras três de banco.

## Por que não mudar o percentil

Seria a mudança menor, e não resolve. Um lock bloqueia quem colide com ele
enquanto é mantido, o que numa janela curta é sempre uma minoria — essa é a forma
que a falha tem por natureza, não uma coincidência da rodada. Qualquer percentil
alto o bastante para ser estável vai deixá-la passar, e qualquer percentil baixo
o bastante para pegá-la passa a ser dominado por ruído. O p99 sobre amostra
pequena é o pior dos dois: instável e ainda cego a caudas de menos de 1%.

Trocar o p95 também apagaria o que ele faz bem. A degradação uniforme é real e
frequente, e o p95 a descreve melhor do que qualquer contagem de cauda.

## O que se aceita em troca

**As duas regras disparam juntas quando a degradação é uniforme.** Não é defeito:
as duas afirmações são verdadeiras sobre o mesmo banco, e suprimir uma verdade
para reduzir linhas no relatório seria a troca errada. O teto por classe da
[ADR 0010](0010-teto-por-classe-de-peso.md) impede que o mesmo fato seja pago
duas vezes — verificado: o relatório exportado dos fixtures passou de cinco para
seis findings com o score do suspeito inalterado em 0,5436.

**A regra ainda depende de a coleta enxergar a espera.** Se o driver não emite
span para a operação bloqueada, ou se a operação é abortada antes de virar
telemetria, não há o que medir aqui — `database_timeout` e `database_error` são
quem responde nesse caso.

**Amostras pequenas continuam cegas a caudas.** Com oito operações na janela, o
p95 *é* o máximo, e três operações acima do limiar já são quase 40% da amostra. A
matriz E2E roda nessa escala e, nela, quem pega o `table-lock` é o p95 — a regra
nova se cala, corretamente, porque uma ou duas operações não sustentam hipótese.
A cauda só se separa do p95 quando o volume cresce, que é justamente a condição
de produção.
