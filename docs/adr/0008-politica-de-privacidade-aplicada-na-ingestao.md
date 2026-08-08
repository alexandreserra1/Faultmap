# ADR 0008 — Política de privacidade aplicada na ingestão

- Status: aceito
- Data: 2026-08-08

## Contexto

A configuração declara `privacy.blocked_attributes` e `privacy.max_attribute_length`
desde o início, e a especificação exige não armazenar SQL bruto por padrão.
Ambos eram validados no carregamento do YAML e **nenhum ponto do código os
consultava**. Uma busca por `BlockedAttributes` fora do pacote de configuração
não retornava nada.

O efeito só ficou visível com telemetria de uma aplicação real: ao ingerir spans
de DuckDB capturados de uma aplicação de terceiros, 41 de 126 sinais foram
gravados com o texto completo das consultas, incluindo
`SELECT user_id, name, password_hash FROM auth_users WHERE email = ?`.

Nenhum teste podia revelar isso, porque a telemetria escrita por nós nunca
incluía atributos que a política deveria bloquear.

## Decisão

O pacote `internal/telemetry/privacy` aplica a política entre a normalização e a
persistência. `IngestTelemetry` recebe a política e a aplica antes de chamar o
repositório, de modo que **os dois caminhos de ingestão** — arquivo e receiver
OTLP — ficam cobertos pelo mesmo ponto.

A comparação de nomes ignora caixa e espaços: uma variação de escrita no YAML
não pode deixar passar um atributo sensível.

A política é construída uma vez no bootstrap e reutilizada por todas as
requisições; o receiver permanece stateless.

## Consequências

- SQL bruto deixa de ser persistido por padrão, sob qualquer uma das duas
  convenções (`db.statement` e `db.query.text`).
- Atributos acima do limite são truncados em vez de descartados: o valor
  continua sustentando evidência sem custo de cardinalidade.
- Bancos criados antes desta mudança podem conter atributos que a política
  bloquearia. A limpeza retroativa não é feita automaticamente — quem tiver essa
  preocupação deve recriar o workspace ou aplicar a retenção.
- A política atua sobre atributos já normalizados. Um dado sensível que a
  instrumentação coloque em um atributo não previsto na lista continuará sendo
  gravado; a lista é uma allowlist invertida e depende de manutenção.
