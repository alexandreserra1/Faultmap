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

**O sorteio é com reposição, e `small-pool` não saiu nenhuma vez em oito.** O
cenário continua sem passar pelo piloto; a tabela acima não diz nada sobre ele.

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

**O primeiro continua aberto.** O p95 disparando com o sistema saudável em
valores baixos é outro defeito, e corrigi-lo exige medir o ruído em vez de mexer
no piso pelas quatro observações que já tenho.

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
