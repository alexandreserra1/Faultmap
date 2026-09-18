#!/usr/bin/env bash
set -euo pipefail

# Sorteia um incidente na demo-shop sem revelar qual, para uma investigação cega.
#
# O protocolo do piloto cego (../README.md) exige duas pessoas: quem injeta a
# falha não pode investigar, porque já sabe a resposta. Isso trava o piloto
# sempre que só há uma pessoa disponível — e travou o nosso.
#
# Aqui a escolha é do sorteio, não de uma pessoa. A verdade vai selada para um
# arquivo que ninguém lê até a hipótese estar registrada, então investigador e
# operador podem ser a mesma pessoa — ou a pessoa e o assistente, juntos.
#
# O que isto NÃO é: o piloto do README, que roda contra uma aplicação de staging
# com tráfego real. Aqui a aplicação é a demo-shop e a carga é gerada. O que se
# mede é se o produto ajuda quem não sabe a resposta; o que não se mede é se ele
# aguenta um sistema que ninguém desenhou para ele.
#
# Limite honesto de cegueira: o sorteio esconde a escolha da saída, mas os
# contêineres em execução carregam a marca do cenário nas variáveis de ambiente.
# Quem inspecionar `docker inspect` durante a investigação quebra o próprio
# experimento. A cegueira aqui é disciplina, não cofre.

DIRETORIO_DO_SCRIPT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
RAIZ="$(cd -- "${DIRETORIO_DO_SCRIPT}/../../.." && pwd)"
DEMO="${RAIZ}/examples/demo-shop"
ENVELOPE="${ENVELOPE:-${RAIZ}/piloto-cego-envelope.txt}"
# As janelas não são segredo: sem elas a investigação não tem o que consultar.
JANELAS="${JANELAS:-${RAIZ}/piloto-cego-janelas.txt}"

# "sem-culpado" está na urna de propósito. Sem ele o investigador sabe que
# sempre há algo quebrado, e passa a procurar um culpado em vez de avaliar a
# evidência — que é justamente o viés que o modo difícil existe para medir.
URNA=(database-slow small-pool payment-500 retry-storm table-lock sem-culpado)

revelar() {
  if [[ ! -f "${ENVELOPE}" ]]; then
    printf 'Nenhum envelope em %s. Rode o sorteio primeiro.\n' "${ENVELOPE}" >&2
    exit 1
  fi
  printf '\n=== Envelope ===\n'
  base64 --decode < "${ENVELOPE}"
  printf '\n'
}

diagnosticar() {
  if [[ ! -f "${JANELAS}" ]]; then
    printf 'Nenhuma janela em %s. Rode o sorteio primeiro.\n' "${JANELAS}" >&2
    exit 1
  fi
  # shellcheck disable=SC1090
  source "${JANELAS}"

  local incidente_s baseline_s
  incidente_s=$(( $(date -u -j -f %Y-%m-%dT%H:%M:%SZ "${FIM}" +%s 2>/dev/null \
    || date -u -d "${FIM}" +%s) - $(date -u -j -f %Y-%m-%dT%H:%M:%SZ "${INICIO_INCIDENTE}" +%s 2>/dev/null \
    || date -u -d "${INICIO_INCIDENTE}" +%s) ))
  baseline_s=$(( $(date -u -j -f %Y-%m-%dT%H:%M:%SZ "${INICIO_INCIDENTE}" +%s 2>/dev/null \
    || date -u -d "${INICIO_INCIDENTE}" +%s) - $(date -u -j -f %Y-%m-%dT%H:%M:%SZ "${INICIO_BASELINE}" +%s 2>/dev/null \
    || date -u -d "${INICIO_BASELINE}" +%s) ))

  printf 'Aguardando a telemetria da janela do incidente...\n' >&2
  local tentativa
  for tentativa in $(seq 1 60); do
    if base exec -T faultmap faultmap diagnose incident \
      --config /etc/faultmap/faultmap.yaml --service checkout-service --environment demo \
      --since "${incidente_s}s" --baseline "${baseline_s}s" --until "${FIM}" 2>&1 \
      | grep -q "Incidente: [1-9]"; then
      break
    fi
    sleep 5
  done

  base exec -T faultmap faultmap diagnose incident \
    --config /etc/faultmap/faultmap.yaml --service checkout-service --environment demo \
    --since "${incidente_s}s" --baseline "${baseline_s}s" --until "${FIM}"
}

if [[ "${1:-}" == "--revelar" ]]; then
  revelar
  exit 0
fi

if [[ "${1:-}" == "--diagnosticar" ]]; then
  sorteado=""   # o diagnóstico não precisa saber, e não deve
  base() { docker compose --project-name faultmap-piloto-cego -f "${DEMO}/compose.yaml" "$@"; }
  diagnosticar
  exit 0
fi

if [[ -f "${ENVELOPE}" ]]; then
  printf 'Já existe um envelope em %s.\n' "${ENVELOPE}" >&2
  printf 'Abra com --revelar, ou apague para sortear de novo.\n' >&2
  exit 1
fi

sorteado="${URNA[$((RANDOM % ${#URNA[@]}))]}"

# A verdade é escrita em base64 para que um `cat` distraído, um `grep` no
# diretório ou a rolagem do terminal não a entreguem antes da hora.
{
  printf 'Cenário sorteado: %s\n' "${sorteado}"
  printf 'Sorteado em: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  if [[ "${sorteado}" == "sem-culpado" ]]; then
    printf 'Nada foi injetado. A resposta certa é "nenhuma anomalia".\n'
  else
    printf 'Descrição: %s\n' "$(sed -n '3p' "${DEMO}/scenarios/${sorteado}/README.md" 2>/dev/null || echo '(sem README)')"
  fi
} | base64 > "${ENVELOPE}"

# base sobe a demo sem defeito nenhum; falha aplica o cenário sorteado.
#
# A separação existe por um defeito que a primeira execução deste script
# expôs: ele subia o cenário inteiro e só depois gerava carga. Para uma falha
# com duração — o table-lock segura a tabela por oito segundos — isso a matava
# antes da janela do incidente, e a investigação media um sistema saudável
# achando que media um quebrado. O harness E2E não erra nisso porque aplica a
# falha imediatamente antes do tráfego, e é o que fazemos aqui.
base() {
  docker compose --project-name faultmap-piloto-cego -f "${DEMO}/compose.yaml" "$@"
}

falha() {
  if [[ "${sorteado}" == "sem-culpado" ]]; then
    return 0
  fi
  docker compose --project-name faultmap-piloto-cego \
    -f "${DEMO}/compose.yaml" -f "${DEMO}/scenarios/${sorteado}/compose.yaml" "$@"
}

printf 'Preparando o ambiente (a saída não revela o sorteio)...\n'
base down --volumes --remove-orphans >/dev/null 2>&1 || true
base up --build -d --wait >/dev/null 2>&1

# carga roda o gerador pelo MESMO conjunto de arquivos compose do estado atual.
#
# Isto não é detalhe. `docker compose run` sobe as dependências do serviço pedido,
# e ao fazê-lo as reconcilia com os arquivos que recebeu: chamar o gerador só com
# o compose base, depois de aplicar o cenário, faz o compose RECRIAR o
# checkout-service conforme o base e desfazer a injeção. A falha era aplicada e
# revertida pela carga seguinte, e o piloto media um sistema saudável achando que
# media um quebrado.
#
# A saída é descartada de propósito — contagem de falhas entregaria o sorteio.
# O código de retorno não é: esconder o erro junto foi o que fez a primeira
# versão deste script morrer em silêncio.
carga() {
  local compose="$1" quantidade="$2"
  if ! "${compose}" --profile tools run --rm \
    -e REQUESTS="${quantidade}" -e CONCURRENCY=5 load-generator >/dev/null 2>&1; then
    printf 'A geração de carga falhou. O piloto não tem telemetria para investigar.\n' >&2
    exit 1
  fi
}

# compose_do_incidente devolve qual conjunto usar depois da injeção.
compose_do_incidente() {
  if [[ "${sorteado}" == "sem-culpado" ]]; then printf 'base'; else printf 'falha'; fi
}

# Uma rodada descartada antes da baseline: sem ela a primeira janela mede
# processo frio e a segunda mede processo quente, e a diferença aparece como
# regressão de latência que ninguém injetou.
printf 'Aquecendo...\n'; carga base 60
sleep 8
inicio_baseline="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf 'Janela de baseline...\n'; carga base 120
sleep 8
# A falha entra agora, entre as duas janelas, para viver dentro do incidente.
#
# O estado é CONFERIDO depois, e não deduzido do código de retorno. Duas
# execuções deste piloto foram invalidadas por injeções que falharam em
# silêncio — uma porque o `|| true` engoliu o erro, outra sem que eu soubesse
# qual das chamadas tinha falhado. Um piloto que mede o sistema errado é pior
# que um piloto que não roda, porque produz um resultado no qual alguém acredita.
if [[ "${sorteado}" != "sem-culpado" ]]; then
  falha up -d --force-recreate --wait payment-service checkout-service >/dev/null 2>&1 || true
  # lock-holder só existe no cenário que o usa; a ausência não é erro.
  falha up -d lock-holder >/dev/null 2>&1 || true
  sleep 2

  # Falha fechada: se o exec não devolver nada, a verificação não sabe o estado
  # e não pode aprovar. A versão anterior usava `|| true` aqui e aprovava o vazio
  # — a guarda escrita para impedir aprovação por vacuidade tinha exatamente esse
  # defeito.
  versao_aplicada="$(base exec -T checkout-service printenv SERVICE_VERSION 2>/dev/null)" || versao_aplicada=""
  versao_pagamento="$(base exec -T payment-service printenv SERVICE_VERSION 2>/dev/null)" || versao_pagamento=""
  if [[ -z "${versao_aplicada}" || -z "${versao_pagamento}" ]]; then
    printf 'Não consegui ler a versão dos serviços; o estado da injeção é desconhecido.\n' >&2
    exit 1
  fi
  if [[ "${versao_aplicada}" == "1.0.0" && "${versao_pagamento}" == "1.0.0" ]]; then
    printf 'A injeção da falha não pegou: os serviços seguem na versão base.\n' >&2
    printf 'O piloto mediria um sistema saudável achando que mede um quebrado.\n' >&2
    printf 'Envelope preservado em %s para diagnóstico.\n' "${ENVELOPE}" >&2
    exit 1
  fi
fi
sleep 1
inicio_incidente="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf 'Janela de incidente...\n'; carga "$(compose_do_incidente)" 120
sleep 8
fim="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

{
  printf 'INICIO_BASELINE=%s\n' "${inicio_baseline}"
  printf 'INICIO_INCIDENTE=%s\n' "${inicio_incidente}"
  printf 'FIM=%s\n' "${fim}"
} > "${JANELAS}"

cat <<RESUMO

=== Investigação cega pronta ===

  Baseline começou em:  ${inicio_baseline}
  Incidente começou em: ${inicio_incidente}
  Fim:                  ${fim}

Investigue com:

  $0 --diagnosticar

As janelas saem de ${JANELAS}, calculadas a partir dos carimbos acima. Digitá-las
à mão foi a origem de várias investigações que consultaram a janela errada.

Registre a hipótese ANTES de abrir o envelope. Depois:

  $0 --revelar

Para encerrar:

  docker compose --project-name faultmap-piloto-cego -f ${DEMO}/compose.yaml down --volumes

RESUMO
