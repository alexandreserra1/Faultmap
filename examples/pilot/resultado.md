# Resultado do piloto cego

Executado contra o StriderEdge, uma aplicação FastAPI + DuckDB que não é nossa,
instrumentada por biblioteca de terceiros. Os cenários de ranking usaram o
`pilot-gateway` como porta de entrada, para que existisse mais de um serviço no
trace.

O valor deste documento está nas observações escritas, não na taxa de acerto:
quatro incidentes são amostra pequena demais para estatística.

## Incidente de explicação — `pilot-01`

Um serviço só, sem gateway.

```text
Causa real: 120 ms de atraso em cada consulta ao DuckDB
Top-1 retornado: strideredge-api (único serviço possível)
Diagnóstico: database_latency_delta, p95 de 3,54 ms para 150,73 ms
Tempo do diagnóstico: < 1 s
Proveniência: 100%
Dados sensíveis gravados: nenhum
```

**O Faultmap acertou a medida.** Os 150 ms observados correspondem aos 120 ms
injetados mais o tempo real da consulta.

**O investigador humano errou a causa**, e este é o achado mais valioso do
piloto. Diante de "p95 das operações duckdb aumentou de 3,54 ms para 150,73 ms",
a hipótese registrada foi *"usuário incompleto ou SQL injection"* — duas
hipóteses sobre os **dados** estarem errados, quando a causa era sobre o
**tempo**.

O relatório informa uma medida correta e não sugere o que costuma produzir aquela
medida. Um número de latência de banco, sozinho, não evoca "pool de conexões
esgotado", "lock na tabela", "índice ausente" ou "disco lento" — que é a lista
que alguém experiente teria automaticamente.

A cautela é deliberada e está certa: o produto nunca afirma causalidade. Mas
passou do ponto e virou termômetro — mostra a febre sem sugerir o que investigar.
Nenhum teste automatizado poderia ter encontrado isto, porque todos verificam se
o número está certo, e o número estava certo.

## Cenários de ranking — dois serviços

```text
curl ──► strideredge-gateway ──► strideredge-api ──► DuckDB
```

| Cenário | Resposta correta | Retornado | |
| --- | --- | --- | --- |
| Backend lento | `strideredge-api` | `strideredge-api` 0,28; gateway 2º com 0,09 | acertou |
| Gateway lento | `strideredge-gateway` | `strideredge-gateway`; backend ausente | acertou |
| Nenhuma falha | silêncio | "nenhuma anomalia determinística" | acertou |

**Top-1: 3 de 3.** Critérios do protocolo (top-1 ≥ 60%, top-3 ≥ 80%) atendidos.

O primeiro cenário era genuinamente difícil: os dois serviços ficaram lentos
quase igual — 326 ms no backend e 329 ms no gateway — porque o gateway espera a
resposta. Um produto que olhasse apenas latência estaria escolhendo por sorte, e
poderia acusar o gateway, que é quem o usuário vê sofrendo. O desempate veio da
evidência de banco, que só o backend tem.

O segundo cenário é a contraprova, e existe porque um produto que sempre
apontasse para o serviço mais profundo passaria no primeiro sem diagnosticar
nada. Ele inverteu a resposta quando a realidade inverteu.

O terceiro mostra silêncio **com escopo declarado**: o relatório diz que comparou
os dois serviços e que 72 traces sustentaram a análise. Silêncio com escopo é
diferente de silêncio por não ter olhado.

## Cobertura dos detectores

| Detector | Disparou | Foi útil |
| --- | --- | --- |
| `error_rate_delta` | não | — |
| `latency_delta` | sim | sim, mas insuficiente sozinho para desempatar |
| `database_timeout` | não | — |
| `database_error` | não | — |
| `database_latency_delta` | sim | foi ele que separou culpado de vítima |
| `database_http_trace_correlation` | não | — |
| `retry_storm` | não | — |
| `deployment_proximity` | não | — |
| `version_regression` | não | — |
| `dependency_failure` | não | — |
| `trace_break` | não | — |
| `log_correlation` | não | — |

**Dois de doze detectores dispararam em telemetria real.** Os outros dez seguem
exercitados apenas por cenários que nós desenhamos. Isso não os torna errados,
mas é honesto registrar que não foram confrontados com a realidade.

## O que este piloto NÃO valida

```text
Correlação com GitHub: o StriderEdge não registra deployments na API, então
deployment_proximity e version_regression não foram exercitados.

Logs: log_correlation exige um Collector no caminho, porque nenhum SDK de
aplicação exporta OTLP em JSON. O piloto rodou sem Collector.

Topologia: dois serviços é o mínimo para a pergunta "quem é o culpado" existir.
Um sistema real tem dez, com dependências cruzadas e incidentes concorrentes.

Ambiente: máquina local não reproduz latência de rede nem volume de produção.

Amostra: quatro incidentes desenhados por quem conhece o produto. Um incidente
inesperado vale mais que os quatro.
```

## Defeitos encontrados na montagem

Nenhum destes veio de teste unitário. Todos vieram de tentar usar o produto.

**No produto:**

- `privacy.blocked_attributes` do YAML substituía os bloqueios padrão em vez de
  somar. Uma lista própria removia em silêncio a proteção contra SQL bruto e
  caminho de arquivo: 49 sinais gravaram SQL em disco no teste que comprovou o
  defeito. Corrigido, com [ADR 0012](../../docs/adr/0012-bloqueios-de-privacidade-somam-em-vez-de-substituir.md).
- Logs em protobuf eram recusados sem que ninguém pudesse descobrir por quê: a
  resposta OTLP não pode conter detalhes internos e o processo não registrava
  nada. Agora a causa vai ao terminal do operador, uma vez por motivo distinto.
- O Collector é **obrigatório** para logs, não recomendado. O SDK Python recusa
  `http/json` na inicialização. Não estava documentado.
- O README afirmava que logs não eram recebidos e usava `otlp_http` como nome do
  exportador do Collector, que não existe — o correto é `otlphttp`, confirmado
  rodando `validate` na imagem oficial.

**No método, o mais instrutivo:** na primeira execução do cenário difícil, o
Faultmap acusou o gateway e pareceu um defeito do produto. Não era. O tráfego da
baseline havia batido enquanto o backend ainda subia, então o backend mal
aparecia na janela comparada — e, com aqueles dados, o gateway realmente era o
único que havia piorado. A resposta estava certa para a pergunta errada.

Aceitar aquele resultado teria produzido um relatório de defeito falso e,
possivelmente, o "conserto" de um produto correto. O roteiro passou a abortar se
qualquer serviço tiver menos de 30 sinais na baseline, porque **serviço ausente da
janela vira "não piorou" em vez de "não foi medido"**, e a olho nu as duas coisas
são idênticas.

## Conclusão

```text
O ranking ajudou um humano que não sabia a resposta: parcialmente.

Acertou quem — em três cenários de três, incluindo o caso em que culpado e
vítima ficaram lentos quase igual, e a contraprova em que a culpa estava na
frente.

Não ajudou no porquê. O investigador leu a medida certa e formulou uma hipótese
de natureza errada.
```

**O que mudar no produto por causa deste piloto:** dar a cada evidência uma
frase sobre o que costuma causar aquele padrão, sem afirmar causalidade. A
distância entre "o p95 do banco subiu 40 vezes" e "isto costuma ser lock,
saturação de pool ou consulta sem índice" é a distância entre um termômetro e um
diagnóstico.

**O que medir no próximo:** se essa frase muda a hipótese que a pessoa formula.
Mesmo protocolo, mesmos cenários, e a pergunta *"o que você acha que quebrou"*
comparada antes e depois.
