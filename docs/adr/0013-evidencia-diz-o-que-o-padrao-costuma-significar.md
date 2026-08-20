# ADR 0013 — Cada evidência diz o que aquele padrão costuma significar

- Status: aceito
- Data: 2026-08-19

## Contexto

O piloto cego produziu um resultado que nenhum teste automatizado poderia ter
produzido.

O diagnóstico estava correto: a evidência dizia que a duração p95 das operações
DuckDB havia aumentado de `3,54 ms` para `150,73 ms`, e a falha injetada era
exatamente um atraso de `120 ms` em cada consulta. Score, proveniência e ranking
estavam certos.

A pessoa que investigava, sem saber o que havia sido quebrado, registrou como
hipótese *"usuário incompleto ou SQL injection"*. Duas hipóteses sobre os
**dados** estarem errados, quando a medida falava sobre **tempo**.

O relatório informava uma medida correta e não oferecia a lista que alguém
experiente teria de imediato: lock retido, saturação de pool, consulta sem
índice, armazenamento lento. Um número de latência de banco, isolado, não evoca
nenhuma dessas coisas.

Nossos testes nunca poderiam ter encontrado isso, porque todos verificam se o
número está certo — e o número estava certo. O defeito estava na distância entre
uma medida verdadeira e uma investigação que começa.

## Decisão

**Cada regra declara o que aquele padrão de sinais costuma significar**, em uma
frase que acompanha a evidência no terminal, no `explain` e no relatório
Markdown.

O texto **oferece sempre mais de uma possibilidade**. Isso não é estilo: uma
causa única leria como veredito, e o produto não afirma causalidade. Duas ou mais
deixam explícito que são hipóteses a verificar. Um teste rejeita qualquer regra
cuja frase não apresente alternativas.

A ordem de renderização é evidência, depois proveniência, depois causas comuns,
depois limitações. A ressalva que nega causalidade continua tendo a última
palavra, em cada finding e no rodapé.

Um teste enumera as doze regras e falha quando alguma não tem frase, para que
acrescentar um detector obrigue a dizer o que ele significa.

## Consequências

- O relatório deixa de ser um termômetro. A distância entre "o p95 do banco
  subiu 40 vezes" e "isto costuma ser lock, saturação de pool ou consulta sem
  índice" é a distância entre mostrar a febre e sugerir onde procurar.
- O texto é conhecimento de domínio embutido em código, e envelhece: bancos,
  runtimes e padrões de falha mudam. Ele vive em um único mapa por regra,
  justamente para que revisá-lo seja uma leitura curta.
- Há risco de âncora: a pessoa pode se fixar na primeira causa listada e parar
  de pensar. É uma troca aceita — a alternativa observada foi formular uma
  hipótese de natureza completamente errada. A ordem dentro de cada frase vai do
  mais frequente ao menos, para que a âncora, quando ocorrer, seja a mais
  provável.
- Falta medir se a frase muda a hipótese que a pessoa formula. O próximo piloto
  repete o mesmo protocolo e compara a resposta à pergunta *"o que você acha que
  quebrou"* antes e depois desta mudança.
