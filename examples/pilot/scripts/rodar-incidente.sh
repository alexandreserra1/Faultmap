#!/usr/bin/env bash
#
# ATENÇÃO: os caminhos abaixo apontam para a máquina onde o piloto foi
# executado (StriderEdge, venv, diretório de trabalho). Ajuste-os antes de usar.
# Roteiro de um incidente do piloto cego.
#
# Uso: rodar-incidente.sh <nome> [rota]
#
# A aplicação sobe UMA vez e permanece de pé. A falha entra e sai escrevendo um
# arquivo, que o middleware consulta a cada requisição. Reiniciar o processo era
# a parte frágil da versão anterior: travava esperando o processo antigo morrer
# e abria um intervalo morto entre as duas janelas.
#
# A saída contém apenas as janelas de tempo. A falha sorteada vai para o
# diretório lacrado, nunca para a saída padrão nem para os argumentos.

set -euo pipefail

SCRATCH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STRIDE=/Users/user/Desktop/strideredge_os
PYTHON="$STRIDE/.venv/bin/python"
PORT=8099
ENDPOINT=http://127.0.0.1:4319

NOME="${1:?informe o nome do incidente}"

# A rota exercitada precisa consultar o banco, senão a falha de banco lento não
# tem efeito e o incidente sai vazio — foi o que anulou a primeira execução, com
# um endpoint de vocabulário estático que nunca consulta nada.
ROTA="${2:-/api/v1/injuries}"
URL="http://127.0.0.1:${PORT}${ROTA}"
# O login servia ao propósito de consultar o banco, mas tem limitador de taxa:
# sob carga as requisições viram 429 antes de chegar ao banco, e as duas janelas
# passam a medir o limitador. A rota de lesões consulta o banco, não é limitada,
# e exige um token — de um usuário de teste criado só para o piloto.
TOKEN="$(cat "$SCRATCH/pilot-token.txt")"
FALHA_ARQUIVO="$SCRATCH/falha-atual.txt"

# O sorteio acontece aqui dentro porque quem investiga enxerga os comandos
# executados. Como argumento, a falha deixaria de ser secreta na largada.
CARDAPIO=(
  "latency:${ROTA}:600"
  "error500:${ROTA}:0.45"
  "db_slow:${ROTA}:120"
)
FALHA="${CARDAPIO[$((RANDOM % ${#CARDAPIO[@]}))]}"

# A espera precisa superar o lote do SDK (5s) com folga; abaixo disso o
# diagnóstico corre antes de a telemetria chegar.
FLUSH="${OTEL_FLUSH_WAIT_SECONDS:-10}"

# trafego espera apenas os curls da rodada, nunca `wait` puro.
#
# A aplicação também é um filho deste script, e `wait` sem argumento espera por
# todos os filhos — inclusive o uvicorn, que só termina no fim do piloto. Era o
# que travava o roteiro logo depois da primeira rodada, com a aplicação
# respondendo normalmente e nenhum curl pendente: um travamento sem sintoma.
trafego() {
  local rodadas="$1" pids=() pid
  for _ in $(seq 1 "$rodadas"); do
    pids=()
    for _ in $(seq 1 12); do
      curl -s -o /dev/null -m 25 -H "Authorization: Bearer $TOKEN" "$URL" || true &
      pids+=($!)
    done
    for pid in "${pids[@]}"; do wait "$pid" || true; done
  done
}

rm -f "$FALHA_ARQUIVO"
pkill -f "uvicorn faultmap_app" 2>/dev/null || true
sleep 2

cd "$SCRATCH/bootstrap"
FAULTMAP_PILOT_FAULT_FILE="$FALHA_ARQUIVO" \
PYTHONPATH="$SCRATCH/otel-libs:$STRIDE" \
OTEL_SERVICE_NAME=strideredge-api \
OTEL_RESOURCE_ATTRIBUTES="service.version=piloto,deployment.environment.name=piloto" \
OTEL_EXPORTER_OTLP_ENDPOINT="$ENDPOINT" \
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
OTEL_TRACES_EXPORTER=otlp OTEL_LOGS_EXPORTER=none OTEL_METRICS_EXPORTER=none \
  nohup "$PYTHON" "$SCRATCH/otel-libs/bin/opentelemetry-instrument" \
  "$PYTHON" -m uvicorn faultmap_app:app --host 127.0.0.1 --port "$PORT" \
  > "$SCRATCH/app-${NOME}.log" 2>&1 &
until curl -s -m 2 -o /dev/null -H "Authorization: Bearer $TOKEN" "$URL"; do sleep 0.5; done

# Baseline: aplicação saudável. Metade do diagnóstico está aqui, porque todo
# detector compara janelas e nenhum reporta estado absoluto.
BASELINE_INICIO=$(date +%s)
trafego 6
BASELINE_FIM=$(date +%s)

sleep "$FLUSH"

# O protocolo manda parar se a telemetria não chegou. A razão apareceu na
# prática: sem o wrapper de instrumentação a aplicação respondia 200 e não
# exportava nada, e o roteiro entregava uma janela vazia com aparência boa.
SINAIS=$(python3 -c "
import sqlite3
print(sqlite3.connect('$SCRATCH/pilot/faultmap.db').execute('SELECT COUNT(*) FROM signals').fetchone()[0])
")
if [ "$SINAIS" -lt 20 ]; then
  echo "Baseline com apenas ${SINAIS} sinais; a instrumentação não está exportando." >&2
  exit 1
fi

printf '%s' "$FALHA" > "$FALHA_ARQUIVO"
INCIDENTE_INICIO=$(date +%s)
trafego 6
INCIDENTE_FIM=$(date +%s)
rm -f "$FALHA_ARQUIVO"

sleep "$FLUSH"
FIM=$(date +%s)

mkdir -p "$SCRATCH/pilot-verdade"
cat > "$SCRATCH/pilot-verdade/${NOME}.txt" <<EOF
Incidente: ${NOME}
Falha real: ${FALHA}
Rota alvo: ${ROTA}
Baseline: $(date -u -r "$BASELINE_INICIO" +'%Y-%m-%dT%H:%M:%SZ') .. $(date -u -r "$BASELINE_FIM" +'%Y-%m-%dT%H:%M:%SZ')
Incidente: $(date -u -r "$INCIDENTE_INICIO" +'%Y-%m-%dT%H:%M:%SZ') .. $(date -u -r "$INCIDENTE_FIM" +'%Y-%m-%dT%H:%M:%SZ')
EOF

cat <<EOF
Incidente ${NOME} pronto para investigação.

  --since    $((FIM - INCIDENTE_INICIO + 2))s
  --baseline $((INCIDENTE_INICIO - BASELINE_INICIO + 2))s
  --until    $(date -u -r "$FIM" +'%Y-%m-%dT%H:%M:%SZ')
EOF
