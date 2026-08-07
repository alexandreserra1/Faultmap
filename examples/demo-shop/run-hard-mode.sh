#!/usr/bin/env bash
set -euo pipefail

# Modo difícil: ao contrário da matriz E2E, que verifica se o Faultmap ACUSA o
# serviço certo, estes cenários verificam se ele SE CONTÉM quando não há
# regressão. Falso positivo é o defeito mais caro de um produto de diagnóstico:
# um ranking que sempre acha um culpado é indistinguível de um que adivinha.
#
# Todos os cenários usam telemetria OTLP real, banco isolado e o mesmo binário
# publicado na release. Nenhum deles substitui um incidente de produção.

SCRIPT_DIRECTORY="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPOSITORY_ROOT="$(cd -- "${SCRIPT_DIRECTORY}/../.." && pwd)"
BASE_COMPOSE="${SCRIPT_DIRECTORY}/compose.yaml"
PROJECT_NAME="${FAULTMAP_HARD_PROJECT_NAME:-faultmap-demo-shop-hard}"
CHECKOUT_URL="${FAULTMAP_HARD_CHECKOUT_URL:-http://127.0.0.1:18080/checkout}"
ALL_SCENARIOS="ruido-cronico sem-culpado janela-imprecisa fan-out-legitimo causas-concorrentes"
OTEL_FLUSH_WAIT_SECONDS="${OTEL_FLUSH_WAIT_SECONDS:-6}"

if [[ ! "${PROJECT_NAME}" =~ ^faultmap-demo-shop-hard(-[a-z0-9][a-z0-9-]{0,30})?$ ]]; then
  printf 'Projeto inválido; use faultmap-demo-shop-hard ou um sufixo seguro.\n' >&2
  exit 2
fi

if [[ "$#" -eq 0 ]]; then
  read -r -a scenarios <<<"${ALL_SCENARIOS}"
else
  scenarios=("$@")
fi

compose() {
  docker compose --project-name "${PROJECT_NAME}" -f "${BASE_COMPOSE}" "$@"
}

cleanup() {
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

validate_scenario() {
  local candidate="$1" known
  for known in ${ALL_SCENARIOS}; do
    [[ "${candidate}" == "${known}" ]] && return 0
  done
  printf 'Cenário desconhecido: %s\n' "${candidate}" >&2
  return 2
}

# generate_traffic emite pedidos com IDs estáveis por fase. O erro crônico do
# payment é derivado do order_id, então a mesma fase produz sempre as mesmas
# falhas — é isso que torna a baseline ruidosa reproduzível.
generate_traffic() {
  local phase="$1" count="$2" prefix="$3" index code
  for ((index = 1; index <= count; index++)); do
    code=$(curl --silent --show-error --max-time 5 \
      --output /dev/null --write-out '%{http_code}' \
      --header 'Content-Type: application/json' \
      --data "{\"order_id\":\"${prefix}-${phase}-${index}\",\"amount_cents\":1990}" \
      "${CHECKOUT_URL}") || code="transport-error"
    printf '%s request=%s status=%s\n' "${phase}" "${index}" "${code}"
  done
}

# generate_burst_traffic alterna rajadas e vales para produzir volume irregular,
# em vez do tráfego uniforme que a matriz E2E usa.
generate_burst_traffic() {
  local phase="$1" prefix="$2" burst index code
  for burst in 1 2 3; do
    for ((index = 1; index <= 6; index++)); do
      code=$(curl --silent --show-error --max-time 5 \
        --output /dev/null --write-out '%{http_code}' \
        --header 'Content-Type: application/json' \
        --data "{\"order_id\":\"${prefix}-${phase}-${burst}-${index}\",\"amount_cents\":1990}" \
        "${CHECKOUT_URL}") &
    done
    wait
    sleep 1
  done
  printf '%s rajadas=3 concluídas\n' "${phase}"
}

diagnose() {
  local service="$1" incident_seconds="$2" baseline_seconds="$3" until_rfc="$4"
  compose exec -T faultmap \
    faultmap diagnose incident \
    --config /etc/faultmap/faultmap.yaml \
    --service "${service}" \
    --since "${incident_seconds}s" \
    --baseline "${baseline_seconds}s" \
    --until "${until_rfc}" \
    --limit 500 2>&1
}

# assert_no_finding é a asserção central do modo difícil: a regra citada NÃO
# pode aparecer. Ela detecta o falso positivo que a matriz E2E nunca procura.
assert_no_finding() {
  local output="$1" rule="$2"
  if [[ "${output}" == *"ID da regra: ${rule}"* ]]; then
    printf 'FALSO POSITIVO: o detector %s disparou sem regressão real.\n' "${rule}" >&2
    return 1
  fi
}

assert_contains() {
  local output="$1" expected="$2"
  if [[ "${output}" != *"${expected}"* ]]; then
    printf 'Saída não contém %q.\n' "${expected}" >&2
    return 1
  fi
}

# start_stack sobe o ambiente do zero com as variáveis do cenário já aplicadas,
# garantindo que a baseline nasça com o mesmo comportamento do incidente quando
# o cenário exige ruído permanente.
start_stack() {
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  compose up --build -d --wait
}

run_scenario() {
  local scenario="$1"
  validate_scenario "${scenario}"
  printf '\n=== MODO DIFÍCIL: %s ===\n' "${scenario}"

  local baseline_start incident_start until_epoch until_rfc
  local incident_seconds baseline_seconds output service

  case "${scenario}" in
    ruido-cronico)
      # Erro crônico de 25% presente nas DUAS janelas. Não há regressão: a taxa
      # de erro do incidente é igual à da baseline. O detector deve se calar.
      printf 'Ruído permanente de 25%% de erro em ambas as janelas.\n'
      service="payment-service"
      CHRONIC_ERROR_PERCENT=25 start_stack
      baseline_start="$(date +%s)"
      generate_traffic baseline 16 ruido
      sleep "${OTEL_FLUSH_WAIT_SECONDS}"
      incident_start="$(date +%s)"
      generate_traffic incidente 16 ruido
      ;;
    sem-culpado)
      # Nenhuma mudança entre as janelas. O sistema está saudável nas duas.
      printf 'Sistema saudável nas duas janelas; nada mudou.\n'
      service="payment-service"
      start_stack
      baseline_start="$(date +%s)"
      generate_traffic baseline 16 limpo
      sleep "${OTEL_FLUSH_WAIT_SECONDS}"
      incident_start="$(date +%s)"
      generate_traffic incidente 16 limpo
      ;;
    janela-imprecisa)
      # Tráfego irregular em rajadas, como quem descobre o problema tarde e
      # escolhe uma janela que mistura períodos saudáveis com o incidente.
      printf 'Tráfego em rajadas e janela deslocada.\n'
      service="payment-service"
      start_stack
      baseline_start="$(date +%s)"
      generate_burst_traffic baseline rajada
      sleep "${OTEL_FLUSH_WAIT_SECONDS}"
      incident_start="$(date +%s)"
      generate_burst_traffic incidente rajada
      ;;
    fan-out-legitimo)
      # Quatro chamadas paralelas por checkout, todas bem-sucedidas, nas duas
      # janelas. É repetição normal da mesma operação: retry_storm não deve ver
      # tempestade de retry onde só existe fan-out.
      printf 'Fan-out legítimo de 4 chamadas paralelas por checkout.\n'
      service="checkout-service"
      PAYMENT_FANOUT=4 start_stack
      baseline_start="$(date +%s)"
      generate_traffic baseline 12 fanout
      sleep "${OTEL_FLUSH_WAIT_SECONDS}"
      incident_start="$(date +%s)"
      generate_traffic incidente 12 fanout
      ;;
    causas-concorrentes)
      # Banco lento E erro crônico simultâneos no incidente. Duas evidências
      # verdadeiras competindo: o ranking precisa ordená-las, não empatá-las.
      printf 'Banco lento e erro crônico simultâneos no incidente.\n'
      service="payment-service"
      start_stack
      baseline_start="$(date +%s)"
      generate_traffic baseline 16 concorrente
      sleep "${OTEL_FLUSH_WAIT_SECONDS}"
      DB_DELAY=300ms CHRONIC_ERROR_PERCENT=40 compose up -d --wait payment-service
      incident_start="$(date +%s)"
      generate_traffic incidente 16 concorrente
      ;;
  esac

  sleep "${OTEL_FLUSH_WAIT_SECONDS}"
  until_epoch="$(date +%s)"
  until_rfc="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  incident_seconds="$((until_epoch - incident_start + 2))"
  baseline_seconds="$((incident_start - baseline_start + 2))"
  ((baseline_seconds < 5)) && baseline_seconds=5

  output="$(diagnose "${service}" "${incident_seconds}" "${baseline_seconds}" "${until_rfc}")"
  printf '%s\n' "${output}"

  printf '\n--- Veredito %s ---\n' "${scenario}"
  case "${scenario}" in
    ruido-cronico)
      assert_no_finding "${output}" "error_rate_delta" || return 1
      printf 'Não acusou regressão de erro com ruído idêntico nas duas janelas: PASS\n'
      ;;
    sem-culpado)
      assert_no_finding "${output}" "error_rate_delta" || return 1
      assert_no_finding "${output}" "latency_delta" || return 1
      assert_no_finding "${output}" "retry_storm" || return 1
      assert_contains "${output}" "Nenhuma anomalia determinística" || return 1
      printf 'Não inventou suspeito em sistema saudável: PASS\n'
      ;;
    janela-imprecisa)
      assert_no_finding "${output}" "error_rate_delta" || return 1
      printf 'Volume irregular não virou falso positivo: PASS\n'
      ;;
    fan-out-legitimo)
      assert_no_finding "${output}" "retry_storm" || return 1
      printf 'Fan-out legítimo não foi confundido com retry storm: PASS\n'
      ;;
    causas-concorrentes)
      assert_contains "${output}" "ID da regra: error_rate_delta" || return 1
      assert_contains "${output}" "ID da regra: latency_delta" || return 1
      printf 'Ordenou duas evidências verdadeiras sem descartar nenhuma: PASS\n'
      ;;
  esac
  printf 'MODO DIFÍCIL %s: PASS\n' "${scenario}"
}

cd "${REPOSITORY_ROOT}"

# O runner não aborta no primeiro veredito negativo: o objetivo é medir quantos
# falsos positivos existem, e parar no primeiro esconderia os demais.
declare -a results=()
failures=0
for scenario in "${scenarios[@]}"; do
  if run_scenario "${scenario}"; then
    results+=("${scenario}: PASS")
  else
    results+=("${scenario}: FALHOU")
    failures=$((failures + 1))
  fi
done

printf '\n=== Resumo do modo difícil ===\n'
for result in "${results[@]}"; do
  printf '%s\n' "${result}"
done
printf 'Cenários com falso positivo ou expectativa quebrada: %d de %d\n' "${failures}" "${#scenarios[@]}"
((failures == 0))
