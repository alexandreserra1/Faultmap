# ADR 0011 — Logs entram no Faultmap sem o texto da mensagem

- Status: aceito
- Data: 2026-08-13

## Contexto

Até aqui o Faultmap recebia apenas traces. O peso `log_correlation`, de `0.10`,
estava configurado desde o início e não era usado por regra nenhuma — os pesos
efetivamente aplicados somavam `0.90`.

Logs do OpenTelemetry carregam `trace_id`, e é isso que os torna úteis a um
diagnóstico: ligado ao trace de uma requisição que falhou, um log de erro
mostra que os dois sinais descrevem o mesmo evento. Sem essa ligação, ele é
apenas uma contagem que pode vir de rotina de fundo ou tarefa agendada.

A mensagem, porém, é ao mesmo tempo a parte mais útil e a mais perigosa da
telemetria de logs: é onde aparecem endereço de e-mail, documento, token e corpo
de requisição. A configuração já previa `store_raw_logs: false`, mas o campo
nunca chegou a significar nada porque logs não eram ingeridos.

## Decisão

**O texto da mensagem nunca é armazenado.** O Faultmap guarda severidade,
instante, correlação de trace e atributos permitidos. Quem precisa ler a
mensagem vai ao sistema de logs de origem, que já existe e é feito para isso.

O corpo é usado em um único ponto: compor o identificador do registro, que
precisa distinguir dois logs emitidos no mesmo instante para que reenvios não
dupliquem sinais. Ele entra em um resumo criptográfico e é descartado em
seguida, sem alcançar a persistência.

**O detector exige correlação, não volume.** `log_correlation` só produz
finding quando os logs de erro que cresceram compartilham trace com requisições
que falharam. Crescimento sem correlação pode ser migração, rotina noturna ou
tarefa agendada, e acusá-lo seria repetir o falso positivo que este projeto
gastou várias rodadas combatendo.

**Apenas o mapeamento JSON é aceito.** Acrescentar o protobuf sem nunca ter
visto uma instrumentação real exportando por ele repetiria o erro que já custou
três releases publicadas cegas.

## Consequências

- O peso `log_correlation` passa a ser usado e os pesos configurados somam
  `1.00`.
- O Faultmap não se torna um repositório de logs, e não herda o risco de
  guardar dado pessoal. Em troca, ele não substitui a leitura da mensagem: o
  diagnóstico aponta que houve erro correlacionado e remete à origem.
- A captura de telemetria real revelou um vazamento que o teste sintético não
  tinha: o SDK anexa `code.file.path` a cada registro, com o caminho absoluto do
  arquivo de origem. É a mesma classe de informação do stacktrace, já
  descartado, e passa a ser bloqueada por padrão. `code.function.name` e
  `code.line.number` seguem permitidos: ajudam a investigar sem expor a
  estrutura de diretórios.
- Logs têm volume muito maior que traces. A retenção, que hoje é um comando
  explícito, passa a merecer execução regular em instalações que os ingiram.
- Habilitar `store_raw_logs: true` não tem efeito. O campo permanece na
  configuração por compatibilidade, e uma evolução que decida armazenar
  mensagens precisará de mascaramento próprio antes de honrá-lo.
