# ADR 0012 — A lista de bloqueios do YAML soma aos padrões

- Status: aceito
- Data: 2026-08-16

## Contexto

A configuração é decodificada sobre uma struct já preenchida com os valores
padrão. Isso funciona campo a campo — omitir `server.read_timeout` preserva o
default — mas **uma sequência YAML substitui o slice inteiro**.

`privacy.blocked_attributes` é a única lista da configuração, e é justamente a
que carrega proteções de privacidade. Declarar os atributos sensíveis do próprio
negócio removia em silêncio `db.query.text`, `db.statement`, `code.file.path` e
`code.filepath`.

O efeito era o oposto da intenção: quem escrevia a seção estava tentando
proteger **mais**, e acabava protegendo menos. E era invisível, porque a
configuração carregava sem erro e o produto seguia funcionando.

Isto foi encontrado ao conferir, contra o código, o protocolo de piloto — cuja
configuração de exemplo listava `user.email` e `user.document` e teria reaberto
exatamente o vazamento de caminho de arquivo que a v0.4.0 fechou. O piloto
registraria "dados sensíveis encontrados" para uma falha que o produto não tem
por padrão.

## Decisão

**A lista informada no arquivo é somada aos bloqueios padrão, nunca os
substitui.** A união é deduplicada e ordenada, mantendo determinística a
configuração carregada.

Ninguém tem motivo legítimo para desbloquear SQL bruto ou o caminho absoluto do
arquivo de origem: são dados que não sustentam nenhuma decisão do diagnóstico e
que o produto se compromete a não guardar.

## Consequências

- Perde-se a capacidade de **encolher** a lista padrão. É uma troca deliberada:
  o modo de falha que ela abria é silencioso e vaza dado pessoal, enquanto a
  limitação nova é visível e não tem caso de uso conhecido.
- A verificação passou a ser feita no disco, não na struct: um teste ingere uma
  captura real de psycopg2 — que anexa o SQL em `db.statement` — com uma lista
  própria de bloqueios, e conta as linhas gravadas. Com a união desligada, 49
  sinais gravam SQL bruto; com ela, zero.
- Se um dia houver necessidade real de reduzir a lista, isso exigirá um campo
  próprio e explícito, algo como `unblocked_attributes`, para que a remoção seja
  uma declaração consciente e não o efeito colateral de escrever uma lista.
