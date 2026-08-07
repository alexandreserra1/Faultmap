# ADR 0006 — Detectores aceitam as duas convenções HTTP e ignoram spans internos

- Status: aceito
- Data: 2026-08-07

## Contexto

O Faultmap foi ligado a uma aplicação FastAPI real, instrumentada com
`opentelemetry-instrument`, sem qualquer alteração no código dela. O resultado
foi silêncio total: nenhum detector produzia finding, mesmo com regressão
evidente de latência.

A causa eram dois desencontros entre a demo e a instrumentação do mundo real:

1. **Nome do atributo.** A convenção estável do OpenTelemetry renomeou
   `http.status_code` para `http.response.status_code`. A demo, escrita por nós,
   já usava o nome novo; a instrumentação automática do Python emite o antigo.
   `filterHTTPSignals` reconhecia apenas o nome novo, então os 324 spans da
   aplicação eram lidos como **zero sinais HTTP**. Os detectores de erro e
   latência ficavam completamente cegos, sem nenhum aviso ao usuário.

2. **Spans internos.** A instrumentação ASGI emite um span `http send` por
   requisição, que repete o código de resposta do span principal. Contá-lo como
   requisição dobra o denominador: uma falha de 100% seria reportada como 50%.

O primeiro defeito escondia o segundo. Os renderizadores de terminal já aceitavam
os dois nomes, então a tela mostrava "HTTP 200" normalmente e nada parecia errado.

Nenhum dos onze cenários automatizados poderia ter encontrado isso. Todos usam a
telemetria da própria demo, no mesmo dialeto do código que estavam verificando.

## Decisão

`httpStatusCode` passa a reconhecer as duas convenções, na mesma ordem de
precedência já usada pelos renderizadores. `filterHTTPSignals` descarta spans
`SPAN_KIND_INTERNAL`, mantendo servidor e cliente como as únicas formas de
requisição observada.

Os testes reproduzem a forma exata dos spans emitidos pela aplicação real, e não
uma versão idealizada.

## Consequências

- O Faultmap deixa de ficar cego para aplicações instrumentadas
  automaticamente — provavelmente a maioria das aplicações reais.
- Verificado contra a aplicação real: no mesmo banco e na mesma janela, o
  binário v0.1.0 responde "nenhuma anomalia" e o corrigido identifica o aumento
  de p95 de 1 ms para 15 ms sob concorrência.
- A demo continua sendo insuficiente como garantia de compatibilidade. Ela prova
  que o produto funciona contra si mesmo. Só telemetria de aplicações que não
  escrevemos revela desencontros de convenção, e essa lacuna permanece aberta
  para atributos de banco de dados, que ainda não foram exercitados fora da demo.
- Outras convenções renomeadas pelo OpenTelemetry (por exemplo em atributos de
  rede e de banco) não foram auditadas. Este ADR corrige o caso comprovado, não
  a classe inteira do problema.
