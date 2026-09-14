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

## Confronto com um catálogo de falhas alheio

As frases de causas comuns foram escritas por quem conhece o produto, o que é
exatamente o problema: elas poderiam descrever um vocabulário que só serve aos
nossos cenários. O teste foi confrontá-las com as **15 falhas injetáveis do
OpenTelemetry Demo** — um sistema de mais de vinte serviços poliglotas, com uma
taxonomia de falhas que não escrevemos.

O detector mais exigido, `latency_delta`, orientava sobre **três das sete**
classes de falha que produzem latência naquele catálogo. Faltavam:

| Falha real | Estava na frase? |
| --- | --- |
| `adManualGc` — pausa de coleta de lixo | não |
| `recommendationCacheFailure` — cache parou de servir | não |
| `kafkaQueueProblems` — acúmulo em fila com consumidor atrasado | não |
| `failedReadinessProbe` — instância fora de rotação | não, como perda de capacidade |

A frase foi reescrita e um teste passou a exigir que cada classe do catálogo
apareça na regra que dispararia para ela.

**A primeira versão da minha verificação passou por acidente**, e vale registrar
como: ela procurava as palavras no texto de todas as regras juntas, e encontrava
"cache" em `version_regression` — falando de aquecimento após deploy — e "fila"
em `trace_break` — falando de perda de contexto por proxy ou fila no caminho. A
palavra existia no lugar errado, e o relatório dizia 9 de 11 cobertas quando a
resposta honesta era 3 de 7 na regra que importava.

### Uma lacuna que texto não resolve

`kafkaQueueProblems` expõe algo além da frase: **não existe detector de atraso de
consumidor**. O `semconv` lê `messaging.operation.name` e
`messaging.destination.name`, e o `retry_storm` os usa para distinguir operações
repetidas — mas nada mede acúmulo de fila ou defasagem de consumo.

Num sistema com fila, esse é um modo de falha de primeira classe: o produtor
segue saudável, o consumidor atrasa, e o sintoma aparece minutos depois em outro
lugar. Hoje o Faultmap veria o efeito e não a origem.

Fica registrado como limitação conhecida, não como trabalho a antecipar: um
detector novo só se justifica depois de ver telemetria real de fila.

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
