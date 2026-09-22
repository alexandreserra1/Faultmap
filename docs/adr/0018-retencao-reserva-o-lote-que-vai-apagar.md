# ADR 0018 — A retenção reserva o lote que vai apagar

- Status: aceito
- Data: 2026-09-22

## Contexto

A [ADR 0016](0016-postgres-como-backend-alternativo.md) registrou, duas vezes,
que a retenção ficaria sem `FOR UPDATE SKIP LOCKED`:

> `retention apply` usa a mesma consulta nos dois backends, sem
> `FOR UPDATE SKIP LOCKED`. É um comando explícito de operação, não uma rotina
> concorrente; duas execuções simultâneas no PostgreSQL **podem se bloquear**.

A consequência prevista era bloqueio: uma execução esperaria a outra. Medida,
ela é outra, e pior. Não há bloqueio — há desistência silenciosa.

O caso de uso avança em lotes e tem um único sinal de parada: um lote que volta
com menos linhas que o limite prova que a telemetria expirada acabou
(`internal/application/apply_retention.go`). No PostgreSQL esse sinal é
ambíguo. Duas execuções simultâneas escolhem o mesmo lote — a subconsulta de
cada uma enxerga as linhas que a outra ainda não confirmou. A primeira a
confirmar apaga tudo; a segunda encontra as linhas já apagadas, não espera por
nada, e devolve um lote curto. O lote curto é lido como "acabou". Ela encerra
relatando sucesso, sem truncamento, com o banco cheio de telemetria que a
política mandava apagar.

Medido contra PostgreSQL 16, dez execuções simultâneas de `retention apply` com
lotes de tamanhos diferentes sobre 1.200 sinais expirados, doze repetições:

| | |
|---|---|
| repetições que deixaram telemetria expirada para trás | 10 de 12 |
| sinais expirados restantes, típico | 1.746 de 2.000 (87%) |
| execuções que pararam no primeiro lote | 9 de 10 |
| erros relatados | 0 |
| execuções relatando truncamento | 0 |

O resultado varia porque depende da corrida: quando uma execução sobrevive aos
primeiros rounds, ela termina o trabalho sozinha e o banco fica correto. É a
forma mais perigosa de defeito — funciona na maioria das vezes que alguém
testa, e falha calado no resto.

A poda de catálogo da [ADR 0015](0015-retencao-libera-catalogo-e-preserva-mudancas.md)
erra pelo lado oposto e não perde dado nenhum. Ali a perdedora não encontra a
linha apagada, e sim esvaziada: o `UPDATE` é reaplicado sobre a linha já vazia,
conta de novo o trabalho alheio e segue. O estado final fica certo; o número
relatado, não. Quatro execuções simultâneas relataram 3.520 catálogos liberados
para 1.160 liberações reais — três vezes o trabalho feito. A ADR 0015 promete o
contrário, em letra:

> um catálogo já liberado não entra no lote seguinte, então repetir o comando
> relata zero em vez de contar de novo o mesmo trabalho.

A promessa valia só em sequência.

**O SQLite não tem nenhum dos dois problemas**, e é isso que torna a divergência
um assunto da bateria da ADR 0016 e não uma otimização do backend novo. Aquele
banco admite um escritor por vez: a segunda execução só começa a ler depois que
a primeira confirmou, enxerga a telemetria já removida e escolhe o lote
seguinte. Um lote curto continua significando "acabou". A mesma bateria, contra
os dois backends, passa em um e falha no outro.

## Decisão

**As duas consultas reservam o lote com `FOR UPDATE SKIP LOCKED`.** Cada
execução trava as linhas que vai tocar e pula as que outra já travou, de modo
que dois lotes simultâneos são sempre disjuntos e sempre cheios.

Com a reserva, o sinal de parada volta a ser verdadeiro: quando um lote volta
curto, toda linha expirada ou já foi apagada por esta execução, ou está travada
por outra que a apagará. Não sobra linha sem dono.

**Isto restaura a equivalência entre os backends; não a rompe.** O argumento que
a ADR 0016 usou para recusar o SKIP LOCKED — "faria os dois backends escolherem
lotes diferentes" — não se sustenta na medida. `SKIP LOCKED` só muda o
comportamento diante de linha travada por outra transação, e a bateria em
sequência nunca encontra uma: sem contenção a consulta escolhe exatamente as
mesmas linhas, na mesma ordem, que o SQLite escolheria. A divergência que
existia era a de antes, e ela aparecia no resultado observável — o nível em que
o adendo da própria ADR 0016 já havia decidido cobrar concorrência.

**Reservar pulando, e não esperando.** `FOR UPDATE` sem `SKIP LOCKED` também
corrige o resultado: a perdedora espera, e quando a vencedora confirma, a
varredura segue para as linhas vivas e completa o lote. Foi medido e funciona —
e leva treze vezes mais tempo (21,7 s contra 1,6 s no mesmo cenário), porque
serializa o que não precisa ser serializado. A diferença não aparece em nenhuma
asserção de resultado, só no relógio, e por isso está fixada por um caso que não
usa relógio: uma transação segura o lote mais antigo e não o solta, e a limpeza
precisa devolver um lote cheio das linhas seguintes dentro de um prazo curto.
Quem espera estoura o prazo.

## Consequências

- Duas execuções de `retention apply` no mesmo PostgreSQL passam a se dividir o
  trabalho em vez de uma anular a outra. A retenção deixa de depender de ninguém
  rodar o comando duas vezes ao mesmo tempo — inclusive de um cron cruzando com
  uma execução manual, que é o arranjo que a instância compartilhada torna
  provável.
- O número que o comando imprime volta a ser o trabalho que ele fez, nas duas
  frentes. A soma do que as execuções simultâneas relatam é igual ao que
  desapareceu do banco.
- A condição que a ADR 0016 deixou para revisitar — "se a retenção passar a
  rodar automaticamente, isso precisa ser revisto" — deixa de ser uma condição.
  A correção não depende de a retenção ser manual ou automática, e transformá-la
  em rotina agendada não exige mais nada deste lado.
- A reserva vale enquanto a transação do lote viver, que é o tempo de um
  `DELETE` limitado. Uma execução que morra no meio solta a reserva ao ser
  desfeita, e as linhas voltam para o lote de quem vier depois — que é o
  comportamento que a ADR 0003 já descrevia ao dizer que basta executar o
  comando de novo.
- Duas execuções muito desiguais em lote continuam terminando em momentos
  diferentes, e nada garante que o trabalho se divida por igual. O que se garante
  é que nenhuma delas encerre antes de o trabalho acabar.
- O custo é uma trava de linha por linha do lote, dentro de uma transação que já
  as escrevia de qualquer modo. Não há lock novo em tabela nem em página.
