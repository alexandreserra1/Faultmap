# ADR 0009 — A investigação compara serviços descobertos pelos traces

- Status: aceito
- Data: 2026-08-09

## Contexto

`diagnose incident --service X` lia sinais apenas de X, os detectores filtravam
para X e o ranking agrupava por serviço. O resultado é que **nunca havia mais de
um suspeito**: a lista tinha sempre um nome, e a meta de "top-3 ≥ 80%" era
verdadeira por vacuidade — posição 1 de 1, em todos os cenários.

Pior que a métrica vazia era o comportamento. O produto promete responder "por
onde começo a investigar", mas exigia que a pessoa já soubesse a resposta ao
escolher o serviço. Num incidente em que o `payment` quebra e o `checkout` fica
lento por consequência, investigar o `checkout` levava a um diagnóstico correto
sobre a vítima e silencioso sobre a origem.

## Decisão

O escopo passa a ser **descoberto a partir dos traces** que atravessaram o
serviço de entrada durante a janela do incidente: quem participou do mesmo
caminho da requisição entra na comparação. É a topologia observada, não um
palpite.

Modos disponíveis: expansão por traces (padrão), `--no-expand` para o modo
focado anterior, `--all` para varrer a janela inteira e lista explícita por
vírgula em `--service`.

**Leituras em lote.** Uma consulta de sinais por janela e uma de deployments
para todo o escopo. Consultar por serviço seria um N+1 que cresceria justamente
quando o incidente é mais amplo.

**Identidade estável.** O ID do incidente continua derivado do serviço de
entrada e das janelas, e não do escopo descoberto. O escopo varia conforme a
telemetria já recebida; incluí-lo no ID faria dois retries da mesma investigação
criarem incidentes diferentes.

**Desempate por profundidade.** Quando um serviço falha, quem o chamou tende a
falhar junto: ambos chegam a 100% de erro e empatam em score. O desempate
alfabético colocava a vítima em primeiro — foi o que a matriz E2E flagrou no
cenário `payment-500`, com o `checkout` à frente do `payment`. Suspeitos
empatados passam a ser ordenados pela profundidade de seus spans na cadeia da
requisição: o mais distante da borda vem primeiro.

## Consequências

- O ranking finalmente compara serviços, e o top-3 passa a medir algo.
- Verificado em telemetria real: no `payment-500`, `payment-service` e
  `checkout-service` empatam em 0,25 e a origem aparece primeiro.
- O desempate é uma **heurística sobre a topologia observada**, não prova de
  causalidade. Ele nunca altera a ordem de suspeitos com pontuações diferentes.
  Uma cadeia instrumentada parcialmente, sem `span.parent_id`, faz todos os
  serviços ficarem na profundidade zero e o desempate volta a ser alfabético.
- `--limit` passa a valer para o total de sinais da janela, e não por serviço.
  Um escopo grande divide o mesmo orçamento entre mais serviços; quem investigar
  um incidente amplo deve aumentar o limite.
- O escopo é limitado a 20 serviços por padrão e a 1.000 traces alimentando a
  expansão. Quando o corte acontece, a saída declara o truncamento.
- A saída passa a declarar quantos serviços foram comparados, de onde o escopo
  veio e quantos traces o sustentaram. Sem isso, o ranking apresentaria serviços
  sem explicar por que estão ali.
- A expansão vai a **um nível por padrão**, e `--depth` percorre saltos
  adicionais. Vale entender o que o salto significa: dentro de um mesmo trace a
  cadeia inteira já é alcançada no primeiro nível, porque todos os seus serviços
  compartilham aquele trace. Os saltos servem para o caso diferente — um serviço
  que nunca aparece nos traces do serviço de entrada mas divide traces com um
  vizinho, como uma rotina interna que usa a mesma dependência do fluxo do
  usuário.
- Cada salto adicional é mais uma consulta e traz serviços mais distantes do
  incidente, o que aumenta o risco de falso positivo. Por isso o padrão continua
  em um salto e o máximo são cinco; além disso a expansão viraria a varredura
  que `--all` já oferece. A busca encerra sozinha quando um nível não descobre
  ninguém novo.
