# Sorteio cego na demo-shop

Registro da primeira aplicação do `scripts/sortear-incidente.sh`, em 18/09/2026.

## O que este formato resolve

O [protocolo do piloto](README.md) exige duas pessoas: quem injeta a falha não
pode investigar, porque já sabe a resposta. Isso travou o piloto sempre que só
havia uma pessoa disponível.

O sorteio tira a escolha das mãos de qualquer um. A verdade vai selada em base64
para um arquivo que ninguém abre até a hipótese estar escrita, então investigador
e operador podem ser a mesma pessoa.

**O que ele não substitui.** Aqui a aplicação é a demo-shop e a carga é gerada.
O piloto do README roda contra staging, com tráfego real e uma aplicação que
ninguém desenhou para o Faultmap. Este formato mede se o produto ajuda quem não
sabe a resposta; não mede se ele aguenta um sistema que não é nosso.

## Resultado: três rodadas inválidas antes de uma válida

As três primeiras não mediram o produto. Mediram defeitos do harness, todos da
mesma forma — **erro suprimido fazendo uma checagem passar por vacuidade**:

1. **Falha expirada.** O script subia o cenário e só depois gerava carga. O
   `table-lock` segura a tabela por oito segundos; a carga rodou quinze minutos
   depois. A janela do incidente mediu um sistema saudável.
2. **Injeção não aplicada.** Um `|| true` engoliu o erro da injeção, e a rodada
   seguiu com os serviços na versão base.
3. **Guarda que aprovava o vazio.** A verificação escrita para pegar o item 2
   usava `|| true` no próprio `exec`: quando a leitura falhava, ela comparava
   string vazia com `"1.0.0"`, dava falso, e aprovava. A guarda tinha o defeito
   que existia para impedir.

A causa raiz das três: **o gerador de carga revertia a injeção.** `docker compose
run` sobe as dependências do serviço pedido e as reconcilia com os arquivos que
recebeu — chamar o gerador só com o compose base, depois de aplicar o cenário,
fazia o compose recriar o `checkout-service` conforme o base. A falha era
aplicada e desfeita pela carga seguinte.

Com a carga passando pelo mesmo conjunto de arquivos, a injeção sobrevive e o
produto detecta: no `table-lock`, latência p95 de 5 ms para 97 ms, HTTP 504 e
falha propagada entre serviços.

## O achado sobre o produto

Na primeira rodada, com a falha já expirada e o sistema saudável, o produto
acusou `database_latency_delta` com **confiança alta**: p95 de banco de 1,64 ms
para 5,20 ms.

É a quarta vez que esse padrão aparece nesta linha de trabalho. As outras três
foram 4 → 12 ms, 3,9 → 12,3 ms e 6,66 → 17,04 ms, todas em sistema saudável, e
todas atribuídas a aquecimento de processo.

Os pisos da regra são dois milissegundos absolutos e o dobro em proporção. Num
banco local, que trabalha na casa do milissegundo, o aquecimento passa pelos dois
com folga. O comentário do detector justifica o piso baixo — uma requisição
dispara várias consultas, e o acréscimo se acumula — e o raciocínio continua
válido; o que a evidência sugere é que ele não se sustenta abaixo de dez
milissegundos.

Não está corrigido. Quatro observações são um padrão, não uma medição: mudar o
piso sem medir trocaria um número arbitrário por outro.

## Resultado de oito rodadas cegas

Rodadas válidas, cada uma com a hipótese escrita antes de abrir o envelope.

| # | Hipótese registrada | Sorteado | |
|---|---|---|---|
| 1 | retry-storm | retry-storm | ✅ |
| 2 | payment-500 | payment-500 | ✅ |
| 3 | database-slow | database-slow | ✅ |
| 4 | database-slow | database-slow | ✅ |
| 5 | sem-culpado | sem-culpado | ✅ |
| 6 | small-pool | **table-lock** | ❌ |
| 7 | database-slow | database-slow | ✅ |
| 8 | database-slow | database-slow | ✅ |

Sete de oito. O `sem-culpado` foi identificado como tal, que é o resultado que o
modo difícil existe para medir.

**`small-pool` não saiu nenhuma vez em oito.** O cenário continua sem passar
pelo piloto; a tabela acima não diz nada sobre ele.

Não foi azar improvável: com sorteio uniforme, a chance de um cenário específico
não sair em oito rodadas é 23%, e a de *algum* dos seis não sair é 80%. Cobrir os
seis custa 13 rodadas medianas e 27 no p95 — medido por simulação, não estimado —
e cada rodada leva minutos.

### Por que o sorteio passou a ser ponderado

Sortear **sem reposição** resolveria a cobertura e destruiria o piloto: na sexta
rodada a resposta estaria determinada, e quem acompanhou as cinco anteriores
acertaria por eliminação, sem investigar nada.

O sorteio agora favorece o que ainda não saiu, com peso `1/(1 + vezes sorteado)`.
A propriedade que importa é preservada: **nenhum cenário é jamais impossível.**
Um já sorteado três vezes mantém 4,8% de chance, então nada pode ser descartado
por eliminação. A cobertura cai para 9 rodadas medianas e 14 no p95.

Três testes fixam isso (`sortear-incidente-test.sh`, dentro do `make verify`):
todo cenário é alcançável, nenhum é excluído por já ter saído, e o que nunca saiu
é favorecido. Mutação confirma que mordem — voltar ao uniforme, remover a
reposição ou fixar o primeiro da urna são todos pegos.

## small-pool, o cenário que faltava

Rodado explicitamente (com `FAULTMAP_CENARIO`, portanto **verificação e não
piloto**), já que oito rodadas cegas não o sortearam. O produto acertou:
`payment-service` em primeiro, score 0,33.

E a medição derrubou uma suposição minha. Na rodada 4 eu havia escrito que
"small-pool inflaria o HTTP sem inflar o span de banco", porque a espera por
conexão acontece *antes* da consulta. Está errado: a instrumentação inclui a
aquisição da conexão dentro do span de banco, e o p95 foi de 6,27 ms para
1.832,60 ms.

Isso explica a outra metade do erro da rodada 6. Eu atribuí o engano só à
cegueira do p95, mas havia uma segunda causa: **meu modelo da assinatura de
`small-pool` estava errado**, então mesmo com o sinal correto na tela eu teria
escolhido mal. Uma correção de produto não teria salvado aquela rodada sozinha.

A rodada também exercitou, em telemetria real, a cláusula condicional da regra
de cauda: com "96 de 96 operações", o relatório omitiu "as demais seguiram
normais", que é o que o teste exige e não havia como verificar fora de produção.

## O que a rodada 6 mediu

A hipótese errada não foi um palpite infeliz. Foi a leitura que o produto
induz, e ela expõe um defeito.

Sob `table-lock`, o produto **não emitiu `database_latency_delta`** — mostrou
apenas 504 em 4,17% e latência HTTP de 4 ms para 21 ms. Li a ausência do sinal
de banco como "o banco respondeu bem, logo a espera é antes dele", que é a
assinatura de `small-pool`.

Os spans crus da janela dizem outra coisa:

| | |
|---|---|
| operações de banco | 120 |
| marcadas como erro | **0** |
| p50 | 0,5 ms |
| p90 | 1,51 ms |
| **p95** | **21,11 ms** |
| p97 | 1.999,35 ms |
| máximo | 1.999,60 ms |

Cinco operações de 120 (4,2%) bateram no lock e esperaram dois segundos. Nenhuma
falhou, então todas entraram na conta do detector. **O lock inteiro vive acima do
p95** — por uma observação.

O detector lê p95. Um lock bloqueia quem colide com ele enquanto ele é mantido,
o que numa janela curta é sempre uma minoria: é a forma que essa falha tem por
natureza. A rodada válida anterior pegou `table-lock` (p95 de 5 ms para 97 ms)
porque ali a carga colidiu mais com o lock. **Detectar ou não vira uma questão de
quanto da carga esbarrou no lock, não de o lock existir.**

Isto não é calibragem de piso. O piso não foi alcançado porque a estatística
escolhida não enxerga essa forma de falha. O comentário em
`internal/detection/database_latency_delta.go` justifica descartar operações que
falharam dizendo que `database_timeout` e `database_error` as explicam — mas aqui
nada falhou, e nenhum dos dois tinha o que explicar.

### Relação com o falso positivo registrado acima

São defeitos distintos no mesmo detector, e não devem ser tratados como um só:

- acima, p95 **dispara** com sistema saudável em valores baixos (1,64 → 5,20 ms);
- aqui, p95 **se cala** diante de uma cauda real de dois segundos.

**O segundo está corrigido.** A regra `database_latency_tail` passou a fazer a
pergunta que o p95 não faz, e a mesma condição reproduzida contra o sistema real
agora devolve `payment-service` em primeiro: "5 de 120 operações PostgreSQL
levaram 1737 ms ou mais, contra um pior caso normal de 2,62 ms; as demais
seguiram normais." Antes o topo era 0,09, só com latência HTTP — que foi o que
induziu a hipótese errada. Veja a
[ADR 0017](../../docs/adr/0017-cauda-de-banco-e-pergunta-propria-nao-outro-percentil.md).

**O primeiro foi medido, e a medição não sustenta mexer no limiar.**

Sete janelas saudáveis — seis rodadas de `sem-culpado` e uma de
`deploy-inofensivo` — foram coletadas com
`examples/pilot/scripts/medir-ruido-do-banco.sh`. Nenhuma disparou:

| rodada | p95 baseline | p95 incidente | Δ | razão | dispara? |
|---|---|---|---|---|---|
| 1 | 4,54 | 5,84 | +1,30 | 0,29 | não |
| 2 | 4,35 | 6,92 | +2,57 | 0,59 | não |
| 3 | 5,94 | 1,75 | −4,19 | — | não |
| 4 | 3,32 | 1,95 | −1,38 | — | não |
| 5 | 3,81 | 7,00 | +3,19 | 0,84 | não |
| 6 | 3,62 | 4,76 | +1,14 | 0,31 | não |
| deploy inofensivo | 5,65 | 3,56 | −2,09 | — | não |

O critério exige o dobro; a razão nunca passou de 0,84. **Zero falsos positivos
em sete.** Mudar o piso com base nas quatro observações avulsas teria sido
trocar um número arbitrário por outro — exatamente o que este registro dizia
para não fazer.

### O mecanismo existe, mas não foi confirmado como a causa

A mesma coleta mostra o aquecimento de processo, em baldes de 8 s desde o
primeiro span:

| janela | operações | p95 |
|---|---|---|
| 0–8 s | 60 | **17,93 ms** |
| 8–16 s | 120 | 4,54 ms |
| 16–24 s | 120 | 5,84 ms |

Os quatro falsos positivos históricos ficaram entre 5,20 e 17,04 ms — dentro
dessa faixa. Isso sugere que o aquecimento vira acusação quando calha de cair na
janela do incidente em vez da baseline.

A previsão foi testada: um cenário que **reinicia** o `payment-service` sem tocar
no banco deveria reproduzir o falso positivo. **Falhou.** No
`deploy-inofensivo`, com a troca de versão confirmada na telemetria, o p95 do
incidente *caiu* de 5,65 para 3,56 ms — a carga só começa depois do health
check, e o aquecimento fica fora da janela.

Então: o mecanismo é real e medido, a correlação de magnitude é boa, e a causa
continua **não demonstrada**. As quatro observações permanecem sem explicação
reproduzível, e o limiar fica como está até que alguém as reproduza.

## A rodada inválida que fechou o buraco da guarda

Numa bateria posterior, uma rodada registrou hipótese `retry-storm` e o envelope
disse `payment-500`. Antes de contar como erro, medi a assinatura crua de
`payment-500` com carga controlada:

| serviço | status | n |
|---|---|---|
| payment-service | 500 | 120 |
| checkout-service | 500 / 502 | 120 / 120 |
| load-generator | 502 | 120 |

**Zero 504, zero retentativa** — uma requisição por serviço. A rodada mostrava 504
e 3,92 tentativas por trace. Não era `payment-500`: a rodada foi inválida, a
quarta desta história, sempre por estado que não correspondia ao envelope.

A guarda de injeção existia e aprovou. Ela perguntava se a versão era diferente
de `1.0.0` — ou seja, "algo foi injetado?" em vez de "foi injetado **isto**?". É
aprovação por vacuidade uma camada acima da que já havia invalidado três rodadas.
Agora ela confronta o carimbo `1.1.0-<cenário>` com o cenário sorteado, e sete
casos de teste fixam isso.

## A otimização que quase virou um gerador de falso positivo

Reaproveitar a pilha entre rodadas derrubou o custo de 164 s para 48–84 s. A
primeira versão estava errada de um jeito que só a medição pegou.

Reaproveitar mantém o volume do PostgreSQL. A carga insere 300 linhas por rodada,
então após três rodadas a tabela `payments` tinha 900 — e, **dentro de cada
rodada**, a janela de incidente passou a consultar uma tabela maior que a da
baseline. Uma rodada de `sem-culpado` acusou o banco com p95 de 1,86 ms para
7,29 ms, razão de 2,92× — acima de qualquer coisa nas sete janelas saudáveis
medidas, cujo máximo foi 0,84×.

O piloto teria medido o próprio harness, e o resultado pareceria a quinta
observação daquele falso positivo do p95 — quando a causa era minha.

Corrigido com `TRUNCATE` no início de cada rodada reaproveitada. A contagem
voltou a 300 por rodada e duas rodadas saudáveis seguidas voltaram ao silêncio.
A reversão da injeção é igualmente explícita e **conferida**: se sobrou carimbo
de cenário anterior, o script sobe a pilha do zero em vez de confiar na rodada
que passou.

## Como repetir

```bash
examples/pilot/scripts/sortear-incidente.sh
# investigue, escreva a hipótese, e só então:
examples/pilot/scripts/sortear-incidente.sh --revelar
```

A urna inclui `sem-culpado`. Sem ele o investigador sabe que sempre há algo
quebrado e passa a procurar um culpado em vez de avaliar a evidência — que é o
viés que o modo difícil existe para medir.

**A cegueira é disciplina, não cofre.** Os contêineres carregam a marca do
cenário nas variáveis de ambiente; quem rodar `docker inspect` durante a
investigação quebra o próprio experimento.
