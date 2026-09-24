#!/usr/bin/env bash
# Testes do sorteio. O sorteio é a única coisa que torna o piloto cego possível
# com uma pessoa; se ele enviesar, o piloto inteiro mede outra coisa.
set -uo pipefail

RAIZ="$(cd -- "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SORTEIO="${RAIZ}/examples/pilot/scripts/sortear-incidente.sh"
falhas=0

# Trava de segurança: sem o modo de sorteio puro, as chamadas abaixo cairiam no
# caminho real e subiriam contêineres 400 vezes. Melhor falhar em um segundo do
# que descobrir isso com a máquina ocupada.
if ! HISTORICO=/dev/null "${SORTEIO}" --sortear-apenas >/dev/null 2>&1; then
  echo "FALHA: ${SORTEIO} não suporta --sortear-apenas (sorteio puro, sem subir nada)" >&2
  exit 1
fi

reportar() {
  if [[ "$1" == "ok" ]]; then printf '  ok   %s\n' "$2"; else printf '  FALHA %s\n' "$2"; falhas=$((falhas + 1)); fi
}

# O sorteio precisa ser invocável sem subir contêiner nenhum, senão cada
# verificação custaria minutos e ninguém a rodaria.
sortear_n() {
  local quantas="$1" historico="$2"
  local _
  for _ in $(seq 1 "${quantas}"); do HISTORICO="${historico}" "${SORTEIO}" --sortear-apenas; done
}

echo "== todo cenario da urna é alcançável =="
historico=$(mktemp)
saida=$(sortear_n 400 "${historico}")
faltando=""
for cenario in database-slow small-pool payment-500 retry-storm table-lock sem-culpado; do
  grep -qx "${cenario}" <<<"${saida}" || faltando="${faltando} ${cenario}"
done
[[ -z "${faltando}" ]] && reportar ok "os seis cenarios saíram em 400 sorteios" \
  || reportar falha "cenario(s) inalcançável(is):${faltando}"

echo "== nenhum cenario é excluído por já ter saído =="
# Esta é a propriedade que separa 'acelerar a cobertura' de 'destruir a
# cegueira': sem reposição, na sexta rodada a resposta estaria determinada e
# quem acompanhou as cinco anteriores acertaria sem investigar. O sorteio
# favorece o que ainda não saiu, mas nunca zera a chance de nada.
historico=$(mktemp)
for _ in $(seq 1 25); do echo "database-slow"; done > "${historico}"
saida=$(sortear_n 400 "${historico}")
repetido=$(grep -cx "database-slow" <<<"${saida}" || true)
[[ "${repetido}" -gt 0 ]] \
  && reportar ok "cenario sorteado 25 vezes ainda saiu ${repetido}x em 400 (não foi excluído)" \
  || reportar falha "cenario exausto virou impossível: a cegueira quebra por eliminação"

echo "== o que ainda não saiu é favorecido =="
historico=$(mktemp)
for cenario in database-slow payment-500 retry-storm table-lock sem-culpado; do
  for _ in $(seq 1 10); do echo "${cenario}"; done >> "${historico}"
done
saida=$(sortear_n 300 "${historico}")
novos=$(grep -cx "small-pool" <<<"${saida}" || true)
# Com peso uniforme seriam ~50 de 300; favorecido, muito mais.
[[ "${novos}" -gt 120 ]] \
  && reportar ok "o único cenario nunca sorteado saiu ${novos}x em 300 (uniforme daria ~50)" \
  || reportar falha "o cenario nunca sorteado saiu só ${novos}x: a cobertura não acelera"

echo "== a guarda exige o cenario sorteado, nao apenas 'algo diferente da base' =="
# Uma rodada foi invalidada exatamente aqui: o envelope dizia payment-500 e a
# telemetria era de retry-storm — 504 e 3,92 tentativas por trace, quando
# payment-500 produz 500 e 502 sem retentativa. A guarda aprovou porque só
# perguntava se a versão era diferente de 1.0.0.
guarda() { "${SORTEIO}" --conferir-injecao "$1" "$2" "$3"; }

casos_aprovar=(
  "table-lock|1.0.0|1.1.0-table-lock"
  "retry-storm|1.1.0-retry-storm|1.1.0-retry-storm"
  "small-pool|1.0.0|1.1.0-small-pool"
)
casos_recusar=(
  "payment-500|1.1.0-retry-storm|1.1.0-retry-storm"
  "payment-500|1.0.0|1.0.0"
  "table-lock||"
  "database-slow|1.0.0|"
  "|1.0.0|1.0.0"
  "|1.1.0-retry-storm|1.1.0-retry-storm"
)

for caso in "${casos_aprovar[@]}"; do
  IFS='|' read -r c v1 v2 <<< "${caso}"
  if guarda "${c}" "${v1}" "${v2}"; then
    reportar ok "aprova ${c} com versoes ${v1:-<vazio>}/${v2:-<vazio>}"
  else
    reportar falha "recusou injecao correta de ${c} (${v1:-<vazio>}/${v2:-<vazio>})"
  fi
done

for caso in "${casos_recusar[@]}"; do
  IFS='|' read -r c v1 v2 <<< "${caso}"
  if guarda "${c}" "${v1}" "${v2}"; then
    reportar falha "aprovou ${c} com versoes ${v1:-<vazio>}/${v2:-<vazio>}: o piloto mediria outro sistema"
  else
    reportar ok "recusa ${c} com versoes ${v1:-<vazio>}/${v2:-<vazio>}"
  fi
done

echo
if [[ "${falhas}" -eq 0 ]]; then echo "sorteio: PASS"; else echo "sorteio: ${falhas} FALHA(S)"; exit 1; fi
