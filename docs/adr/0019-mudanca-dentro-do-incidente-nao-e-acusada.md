# ADR 0019 — Mudança de catálogo dentro do incidente não é acusada

- Status: aceito
- Data: 2026-09-22

## Contexto

`DetectSchemaChangeProximity` cortava assim:

```go
age := incidentStart.Sub(change.ObservedBefore)
if age < 0 || age > SchemaChangeLookback {
    continue
}
```

`ObservedBefore` é a coleta que revelou a mudança, e não o instante dela. Pela
[ADR 0014](0014-schema-guarda-identificadores-nao-expressoes.md), a mudança
ocorreu em algum ponto entre `ObservedAfter` e `ObservedBefore` — o produto
compara duas fotos do catálogo e não sabe qual instante do intervalo é o certo.

O corte por `ObservedBefore` juntava dois casos que não são o mesmo:

1. **O intervalo atravessa o começo do incidente** (`ObservedAfter` antes,
   `ObservedBefore` depois). A mudança pode ter ocorrido antes do incidente.
2. **O intervalo inteiro começa depois do começo do incidente.** Todo instante
   possível da mudança é posterior ao início dele.

O primeiro caso era descartado contra a regra escrita no próprio detector — "a
inclusão usa a ponta otimista: basta que a mudança POSSA ter ocorrido dentro da
janela" — e contra a consequência declarada na ADR 0014, de que a inclusão sai
da ponta otimista e a pontuação da pessimista.

Não é um caso de borda. Quanto mais espaçada a coleta, mais provável que a
coleta seguinte caia já com o incidente em curso: com coleta diária, a migração
das 9h de um incidente das 14h só é vista na coleta do dia seguinte, e o
intervalo inteiro atravessa o começo do incidente. A ADR 0014 prometia que a
coleta rala custaria score — "com coleta diária o score vai a zero (…) e a
mudança aparece no relatório sem pontuar". Ela não aparecia: sumia.

O corte era o mesmo de `deployment_proximity`, que também descarta `age < 0`.
Nas duas regras a linha é idêntica e o significado não: um deployment tem
instante próprio (`DeployedAt`), e ali `age < 0` diz de fato "isto aconteceu
depois que o incidente começou". Uma mudança de catálogo não tem instante, e
`age < 0` diz apenas "a foto que a revelou foi tirada depois". Copiar a regra de
quem mede um instante para quem mede um intervalo foi o erro.

O segundo caso é outra coisa, e é onde está a pergunta difícil. Uma migração
aplicada no meio de um incidente pode ser a causa de ele ter piorado, e pode ser
a tentativa de contê-lo — alguém recriando um índice, revertendo uma coluna,
soltando uma constraint. As duas leituras são igualmente compatíveis com o que o
produto observou.

## Decisão

**O intervalo que atravessa o começo do incidente é apresentado, com a
ambiguidade escrita no finding.** O critério de inclusão passa a ser
`ObservedAfter` anterior ao início do incidente: se algum instante possível da
mudança precede o incidente, ela entra. A pontuação continua saindo da ponta
pessimista, que nesses casos é justamente a anterior ao incidente — nada é
pontuado a partir de instante que caia dentro dele.

O preço vai declarado, não escondido:

- a confiança cai para baixa, sempre;
- a limitação diz que a mudança pode ter ocorrido durante o incidente e que
  **uma migração aplicada como resposta ao incidente apareceria ali do mesmo
  jeito**;
- o resumo situa cada ponta do lado a que ela pertence: "entre 6h antes do
  início do incidente e 20m depois dele", em vez de "entre 20m e 6h antes", que
  seria falso para metade do intervalo;
- entre duas mudanças ambíguas vence a que invadiu menos o incidente, e uma
  mudança que certamente precede o incidente vence qualquer ambígua. Sem essa
  ordem bastaria existir uma mudança ambígua para rebaixar a confiança de um
  finding que hoje sai alto sobre uma migração comprovadamente anterior.

**A mudança cujo intervalo inteiro começa depois do início do incidente
continua não sendo apresentada.** Isto é uma recusa, não uma pendência:

- **Não há proximidade anterior a afirmar.** Todo instante possível é posterior
  ao começo do incidente. A regra se chama proximidade porque mede distância até
  o início do incidente; ali não existe distância a medir, só sobreposição.
- **O score premiaria a ambiguidade.** A fórmula é
  `1 - oldestAge/SchemaChangeLookback`. Com `oldestAge` negativo ela passa de 1 e
  o `clamp` devolve o máximo: o caso mais ambíguo receberia a pontuação mais
  alta, e quanto mais tarde a migração, mais forte a acusação.
- **A corroboração da ADR 0014 é inerte aqui.** A proteção contra a migração
  inofensiva é exigir que o serviço tenha sintoma observado na janela. Durante o
  incidente o serviço está sintomático por construção, então essa exigência passa
  sempre. A rede que pega o falso positivo conhecido não pega este.
- **O erro é assimétrico.** Apresentar uma migração de resposta é apontar quem
  correu para consertar o incidente, com a pontuação máxima e sobre um serviço
  que já está aceso. É o falso positivo mais caro que este produto pode cometer,
  e o modo difícil existe para proibir a família inteira: um ranking que sempre
  acha um culpado é indistinguível de um que adivinha.
- **Nenhum caso real se perde**, pelo mesmo argumento da ADR 0014: uma migração
  que piorou o incidente acende erro, latência ou falha de banco, e aparece como
  `database_latency_delta`, `database_latency_tail` ou `error_rate_delta` — por
  medida, e não por coincidência de horário.

## O que reabriria a questão

Distinguir causa de resposta exige ordenar no tempo duas coisas: o instante da
mudança e o instante do primeiro sintoma. Uma migração posterior ao primeiro
sintoma não pode ter causado o início do incidente.

O produto não tem nenhum dos dois instantes:

- a mudança é um **intervalo entre duas coletas**, pela ADR 0014, e o caminho que
  daria o instante exato — replicação lógica com event triggers — foi recusado
  ali por exigir escrita no banco de quem usa o produto;
- os findings **não têm instante próprio**, pela
  [ADR 0004](0004-timeline-ancora-findings-na-janela-do-incidente.md): são
  ancorados no início da janela do incidente, com `time_source` declarando que o
  instante é derivado.

Reabrir esta decisão exige resolver os dois, e não argumentar sobre o caso de
uso. Uma fonte de instante para a migração — o runner de migração da casa, o log
do servidor, um webhook do pipeline — mais um instante medido para o primeiro
sintoma, e a pergunta passa a ser decidível. O campo `time_source` da ADR 0004
já foi desenhado para essa migração acontecer sem quebrar contrato.

## Consequências

- Com coleta espaçada o produto deixa de ficar mudo no caso mais comum. Era um
  silêncio sem aviso: nenhum erro, nenhuma limitação, só "nenhuma anomalia".
- Toda evidência que atravessa o começo do incidente sai com confiança baixa,
  mesmo com amostra grande e coleta frequente. É intencional: a largura do
  intervalo deixa de ser o único motivo para duvidar.
- O relatório passa a ter dois textos de resumo para a mesma regra, um para cada
  lado. Duplica o texto e evita a frase falsa; a alternativa era um "entre X e Y
  antes do incidente" em que Y está depois do incidente.
- A regra segue na classe de peso de `deployment_proximity`
  ([ADR 0010](0010-teto-por-classe-de-peso-no-ranking.md)), então uma migração
  ambígua não soma com o deploy que a acompanhou.
- Fica um caso conhecido e não coberto: a migração que roda inteira dentro do
  incidente e de fato o piora. O produto a enxerga pelos detectores de medida,
  não por esta regra, e não nomeia o objeto migrado. É a troca aceita.
