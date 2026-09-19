#!/usr/bin/env bash
# Mede o ruído do p95 de latência de banco em janelas sabidamente saudáveis.
#
# Existe porque "o detector dispara com o sistema saudável" é uma afirmação que
# só vale medida. Quatro observações avulsas sugeriram um falso positivo em
# `database_latency_delta`; este script existe para decidir se o limiar precisa
# mudar em vez de trocar um número arbitrário por outro.
#
#   examples/pilot/scripts/medir-ruido-do-banco.sh 6
set -uo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
DEMO="${RAIZ}/examples/demo-shop"
PROJETO="faultmap-medicao-ruido"
SAIDA="${SAIDA:-$(mktemp -d)}"
RODADAS="${1:-6}"

base() { docker compose --project-name "${PROJETO}" -f "${DEMO}/compose.yaml" "$@"; }

echo "Medindo ${RODADAS} rodada(s) saudáveis em ${SAIDA}"
for rodada in $(seq 1 "${RODADAS}"); do
  rm -f "${RAIZ}/piloto-cego-envelope.txt" "${RAIZ}/piloto-cego-janelas.txt"
  # sem-culpado não injeta nada: qualquer sinal aqui é ruído por construção.
  if ! FAULTMAP_CENARIO=sem-culpado "${RAIZ}/examples/pilot/scripts/sortear-incidente.sh" \
       >"${SAIDA}/rodada-${rodada}.log" 2>&1; then
    echo "rodada ${rodada}: falhou ao montar; veja ${SAIDA}/rodada-${rodada}.log" >&2
    continue
  fi
  cp "${RAIZ}/piloto-cego-janelas.txt" "${SAIDA}/janelas-${rodada}.txt"

  contêiner=$(base ps -q faultmap)
  docker cp "${contêiner}:/var/lib/faultmap/faultmap.db"     "${SAIDA}/db-${rodada}.db"     >/dev/null
  # O WAL precisa ir junto: sem ele o arquivo copiado está atrasado, e a medição
  # sairia sobre dados que não são os da janela.
  docker cp "${contêiner}:/var/lib/faultmap/faultmap.db-wal" "${SAIDA}/db-${rodada}.db-wal" >/dev/null 2>&1 || true
  sqlite3 "${SAIDA}/db-${rodada}.db" "PRAGMA wal_checkpoint(TRUNCATE);" >/dev/null 2>&1 || true
  echo "rodada ${rodada}: coletada"
done

base down -v --remove-orphans >/dev/null 2>&1
rm -f "${RAIZ}/piloto-cego-envelope.txt" "${RAIZ}/piloto-cego-janelas.txt"

python3 "${RAIZ}/examples/pilot/scripts/medir-ruido-do-banco.py" "${SAIDA}"
