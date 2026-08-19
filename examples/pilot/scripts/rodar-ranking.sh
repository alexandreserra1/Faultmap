#!/usr/bin/env bash
#
# ATENÇÃO: os caminhos abaixo apontam para a máquina onde o piloto foi
# executado (StriderEdge, venv, diretório de trabalho). Ajuste-os antes de usar.
# Cenários de ranking do piloto: dois serviços, um culpado.
#
# Uso: rodar-ranking.sh <backend-lento|gateway-lento|sem-falha>
#
# O tráfego entra pelo gateway, que encaminha ao StriderEdge. Cada requisição
# vira um trace com os dois serviços dentro, ligados por parentesco real de span.
#
# O caso difícil é backend-lento: o gateway também fica lento, porque está
# esperando a resposta. Os dois pioram juntos e com números parecidos, e acusar
# o gateway seria culpar a vítima.
#
# gateway-lento existe como contraprova: um produto que sempre acusasse o
# serviço mais profundo passaria no primeiro cenário sem diagnosticar nada.

set -euo pipefail

SCRATCH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STRIDE=/Users/user/Desktop/strideredge_os
PYTHON="$STRIDE/.venv/bin/python"
REPO=/Users/user/Desktop/Faultmap
PORTA_APP=8099
PORTA_GATEWAY=8100
ENDPOINT=http://127.0.0.1:4319
ROTA=/api/v1/injuries

CENARIO="${1:?informe backend-lento, gateway-lento ou sem-falha}"
case "$CENARIO" in
  backend-lento|gateway-lento|sem-falha) ;;
  *) echo "cenário desconhecido: $CENARIO" >&2; exit 1 ;;
esac

FALHA_APP="$SCRATCH/falha-atual.txt"
FALHA_GATEWAY="$SCRATCH/atraso-gateway.txt"
TOKEN="$(cat "$SCRATCH/pilot-token.txt")"
URL="http://127.0.0.1:${PORTA_GATEWAY}${ROTA}"
FLUSH="${OTEL_FLUSH_WAIT_SECONDS:-10}"

limpar() {
  rm -f "$FALHA_APP" "$FALHA_GATEWAY"
  pkill -f "uvicorn faultmap_app" 2>/dev/null || true
  pkill -f "pilot-gateway" 2>/dev/null || true
}
trap limpar EXIT
limpar
sleep 2

# O StriderEdge, com a mesma injeção por arquivo já usada nos incidentes de um
# serviço só.
cd "$SCRATCH/bootstrap"
FAULTMAP_PILOT_FAULT_FILE="$FALHA_APP" \
PYTHONPATH="$SCRATCH/otel-libs:$STRIDE" \
OTEL_SERVICE_NAME=strideredge-api \
OTEL_RESOURCE_ATTRIBUTES="service.version=piloto,deployment.environment.name=piloto" \
OTEL_EXPORTER_OTLP_ENDPOINT="$ENDPOINT" \
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
OTEL_TRACES_EXPORTER=otlp OTEL_LOGS_EXPORTER=none OTEL_METRICS_EXPORTER=none \
  nohup "$PYTHON" "$SCRATCH/otel-libs/bin/opentelemetry-instrument" \
  "$PYTHON" -m uvicorn faultmap_app:app --host 127.0.0.1 --port "$PORTA_APP" \
  > "$SCRATCH/app-${CENARIO}.log" 2>&1 &

PORT="$PORTA_GATEWAY" \
SERVICE_NAME=strideredge-gateway \
SERVICE_VERSION=piloto \
DEPLOYMENT_ENVIRONMENT=piloto \
OTEL_EXPORTER_OTLP_ENDPOINT="$ENDPOINT" \
UPSTREAM_URL="http://127.0.0.1:${PORTA_APP}" \
GATEWAY_DELAY_FILE="$FALHA_GATEWAY" \
  nohup "$SCRATCH/pilot-gateway-bin" > "$SCRATCH/gateway-${CENARIO}.log" 2>&1 &

# A prontidão precisa exigir 200 ATRAVÉS da cadeia inteira.
#
# Esperar apenas o gateway responder não basta: curl trata 502 como sucesso, e o
# tráfego da baseline batia no gateway enquanto o backend ainda subia. O
# resultado era uma baseline em que o backend praticamente não aparecia, e o
# diagnóstico acusava o gateway — corretamente, porque com aqueles dados ele era
# o único serviço que havia piorado.
until [ "$(curl -s -o /dev/null -m 3 -w '%{http_code}' -H "Authorization: Bearer $TOKEN" "$URL")" = "200" ]; do
  sleep 0.5
done

trafego() {
  local rodadas="$1" pids=() pid
  for _ in $(seq 1 "$rodadas"); do
    pids=()
    for _ in $(seq 1 12); do
      curl -s -o /dev/null -m 25 -H "Authorization: Bearer $TOKEN" "$URL" || true &
      pids+=($!)
    done
    # Esperar só os curls: o app e o gateway também são filhos deste script e
    # nunca terminam, então `wait` puro travaria aqui para sempre.
    for pid in "${pids[@]}"; do wait "$pid" || true; done
  done
}

# Uma rodada de aquecimento descartada: a primeira consulta de cada processo
# paga compilação de rota, abertura de conexão e cache frio, e entraria na
# baseline como se fosse comportamento normal.
trafego 1
sleep 2

BASELINE_INICIO=$(date +%s)
trafego 6
sleep "$FLUSH"

# Confirma que os DOIS serviços apareceram na baseline. Sem isso, um serviço
# ausente na janela de comparação vira "não piorou" em vez de "não foi medido".
for servico in strideredge-gateway strideredge-api; do
  quantos=$(python3 -c "
import sqlite3
print(sqlite3.connect('$SCRATCH/pilot/faultmap.db').execute(
  \"SELECT COUNT(*) FROM signals WHERE service_name = ?\", ('$servico',)).fetchone()[0])")
  if [ "$quantos" -lt 30 ]; then
    echo "baseline com apenas ${quantos} sinais de ${servico}; a comparação seria inválida" >&2
    exit 1
  fi
done

case "$CENARIO" in
  backend-lento) printf 'db_slow:%s:150' "$ROTA" > "$FALHA_APP" ;;
  gateway-lento) printf '400' > "$FALHA_GATEWAY" ;;
  # sem-falha não escreve nada: as duas janelas são idênticas, e o resultado
  # correto é silêncio. É o cenário que testa se o produto sabe se calar.
  sem-falha) ;;
esac

INCIDENTE_INICIO=$(date +%s)
trafego 6
rm -f "$FALHA_APP" "$FALHA_GATEWAY"
sleep "$FLUSH"
FIM=$(date +%s)

cat <<EOF
Cenário ${CENARIO} pronto.

  --since    $((FIM - INCIDENTE_INICIO + 2))s
  --baseline $((INCIDENTE_INICIO - BASELINE_INICIO + 2))s
  --until    $(date -u -r "$FIM" +'%Y-%m-%dT%H:%M:%SZ')
EOF
