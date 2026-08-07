# ADR 0004 — `timeline.json` ancora findings no início da janela do incidente

- Status: aceito
- Data: 2026-08-06

## Contexto

O artefato `timeline.json` precisa apresentar o incidente em ordem cronológica.
O snapshot persistido, porém, guarda apenas as janelas da investigação
(baseline e incidente) — nenhum finding carrega um instante próprio, porque os
detectores comparam agregados de duas janelas em vez de apontar um evento único.

Fabricar timestamps a partir dos sinais citados exigiria reler telemetria, o que
contraria a regra de que os artefatos derivam do snapshot e não recalculam a
investigação — e que também deixaria de funcionar depois da retenção.

## Decisão

Eventos de janela (`baseline_window_start`, `baseline_window_end`,
`incident_window_start`, `incident_window_end`) usam os instantes reais do
snapshot. Findings são ancorados ao início da janela do incidente e trazem
`time_source: "incident_window_start"`, deixando explícito que o instante é
derivado, não observado.

O documento declara sempre a limitação correspondente, e a ordenação usa
desempate estável por regra e resumo para garantir saída determinística.

## Consequências

- A cronologia é honesta: nenhum instante é inventado e a origem de cada um é
  declarada no próprio artefato.
- Findings não podem ser ordenados entre si por tempo — apenas por regra. Quem
  precisar da sequência real dos sinais deve usar `blame trace`.
- Se o modelo passar a registrar o instante de cada finding, o campo
  `time_source` permite migrar sem quebrar o contrato: basta passar a emitir
  `"snapshot"` para os findings que tiverem instante próprio.
