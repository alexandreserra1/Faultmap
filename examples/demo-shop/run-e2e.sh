#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIRECTORY="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPOSITORY_ROOT="$(cd -- "${SCRIPT_DIRECTORY}/../.." && pwd)"
BASE_COMPOSE="${SCRIPT_DIRECTORY}/compose.yaml"
PROJECT_NAME="${FAULTMAP_E2E_PROJECT_NAME:-faultmap-demo-shop-e2e}"
CHECKOUT_URL="${FAULTMAP_E2E_CHECKOUT_URL:-http://127.0.0.1:18080/checkout}"
ALL_SCENARIOS="database-slow small-pool payment-500 retry-storm timeout-after-deploy table-lock"
KEEP_E2E_ENVIRONMENT="${KEEP_E2E_ENVIRONMENT:-0}"
OTEL_FLUSH_WAIT_SECONDS="${OTEL_FLUSH_WAIT_SECONDS:-6}"
TIMEOUT_DEPLOY_VERSION="${TIMEOUT_DEPLOY_VERSION:-0123456789abcdef0123456789abcdef01234567}"
export TIMEOUT_DEPLOY_VERSION

if [[ ! "${PROJECT_NAME}" =~ ^faultmap-demo-shop-e2e(-[a-z0-9][a-z0-9-]{0,30})?$ ]]; then
  printf 'Projeto E2E inválido; use faultmap-demo-shop-e2e ou um sufixo seguro.\n' >&2
  exit 2
fi
if [[ ! "${OTEL_FLUSH_WAIT_SECONDS}" =~ ^[0-9]+$ ]] || \
  ((OTEL_FLUSH_WAIT_SECONDS < 1 || OTEL_FLUSH_WAIT_SECONDS > 30)); then
  printf 'OTEL_FLUSH_WAIT_SECONDS deve estar entre 1 e 30.\n' >&2
  exit 2
fi

# A matriz controla entradas conhecidas; nomes arbitrários nunca viram caminhos
# de Compose nem comandos executados pelo runner.
if [[ "$#" -eq 0 ]]; then
  read -r -a scenarios <<<"${ALL_SCENARIOS}"
else
  scenarios=("$@")
fi

compose() {
  docker compose --project-name "${PROJECT_NAME}" -f "${BASE_COMPOSE}" "$@"
}

scenario_compose() {
  local scenario="$1"
  shift
  docker compose --project-name "${PROJECT_NAME}" \
    -f "${BASE_COMPOSE}" \
    -f "${SCRIPT_DIRECTORY}/scenarios/${scenario}/compose.yaml" \
    "$@"
}

cleanup() {
  if [[ "${KEEP_E2E_ENVIRONMENT}" == "1" ]]; then
    printf 'Ambiente E2E preservado: projeto=%s\n' "${PROJECT_NAME}"
    return
  fi
  # A remoção é restrita ao nome fixo do projeto E2E e nunca alcança os
  # volumes da demo normal ou recursos Docker não relacionados.
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

validate_scenario() {
  local candidate="$1"
  local known
  for known in ${ALL_SCENARIOS}; do
    if [[ "${candidate}" == "${known}" ]]; then
      return 0
    fi
  done
  printf 'Cenário E2E desconhecido: %s\n' "${candidate}" >&2
  return 2
}

scenario_service() {
  case "$1" in
    database-slow|small-pool|payment-500|table-lock) printf 'payment-service\n' ;;
    retry-storm|timeout-after-deploy) printf 'checkout-service\n' ;;
  esac
}

scenario_findings() {
  case "$1" in
    database-slow|small-pool|table-lock) printf 'latency_delta\n' ;;
    payment-500) printf 'error_rate_delta\n' ;;
    retry-storm) printf 'error_rate_delta retry_storm\n' ;;
    timeout-after-deploy) printf 'deployment_proximity error_rate_delta latency_delta\n' ;;
  esac
}

scenario_request_count() {
  case "$1" in
    small-pool) printf '10\n' ;;
    *) printf '8\n' ;;
  esac
}

run_traffic() {
  local scenario="$1"
  local phase="$2"
  local count
  count="$(scenario_request_count "${scenario}")"
  env \
    CHECKOUT_URL="${CHECKOUT_URL}" \
    RUN_ID="e2e-${scenario}-${phase}-$(date +%s)" \
    REQUEST_COUNT="${count}" \
    REQUEST_INTERVAL="0.05" \
    CONCURRENCY="5" \
    "${SCRIPT_DIRECTORY}/scenarios/${scenario}/generate-traffic.sh"
}

activate_scenario() {
  local scenario="$1"
  case "${scenario}" in
    database-slow|small-pool|payment-500)
      scenario_compose "${scenario}" up -d --wait payment-service
      ;;
    retry-storm)
      scenario_compose "${scenario}" up -d --wait payment-service checkout-service
      ;;
    timeout-after-deploy)
      scenario_compose "${scenario}" up --build -d --wait faultmap payment-service checkout-service
      scenario_compose "${scenario}" exec -d faultmap github-mock
      local attempt
      for attempt in $(seq 1 20); do
        if scenario_compose "${scenario}" exec -T faultmap \
          wget --quiet --output-document=- http://127.0.0.1:9090/health >/dev/null 2>&1; then
          break
        fi
        sleep 0.25
      done
      if ((attempt == 20)) && ! scenario_compose "${scenario}" exec -T faultmap \
        wget --quiet --output-document=- http://127.0.0.1:9090/health >/dev/null 2>&1; then
        printf 'github-mock não ficou saudável.\n' >&2
        return 1
      fi
      # O token é fixo e sem privilégio: autentica apenas o mock no loopback do
      # container e nunca é enviado à rede ou gravado no repositório do usuário.
      scenario_compose "${scenario}" exec -T -e GITHUB_TOKEN=e2e-token faultmap \
        faultmap ingest github \
        --config /etc/faultmap/faultmap.yaml \
        --repo acme/checkout \
        --commits \
        --deployments \
        --service checkout-service \
        --environment demo \
        --since 10m \
        --limit 20
      ;;
    table-lock)
      scenario_compose "${scenario}" up -d --wait payment-service
      scenario_compose "${scenario}" up -d lock-holder
      # O container efêmero precisa adquirir o lock antes da primeira chamada.
      sleep 1
      ;;
  esac
}

assert_contains() {
  local output="$1"
  local expected="$2"
  if [[ "${output}" != *"${expected}"* ]]; then
    printf 'Saída E2E não contém %q.\n' "${expected}" >&2
    return 1
  fi
}

run_scenario() {
  local scenario="$1"
  local service finding
  local baseline_start incident_start until_epoch until_rfc
  local incident_seconds baseline_seconds output

  validate_scenario "${scenario}"
  service="$(scenario_service "${scenario}")"
  printf '\n=== E2E %s ===\n' "${scenario}"

  # Cada cenário nasce com SQLite e PostgreSQL vazios no projeto isolado para
  # que sua baseline não seja contaminada por uma execução anterior.
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  compose up --build -d --wait

  baseline_start="$(date +%s)"
  run_traffic "${scenario}" baseline
  sleep "${OTEL_FLUSH_WAIT_SECONDS}"

  activate_scenario "${scenario}"
  incident_start="$(date +%s)"
  run_traffic "${scenario}" incident
  sleep "${OTEL_FLUSH_WAIT_SECONDS}"

  until_epoch="$(date +%s)"
  until_rfc="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  incident_seconds="$((until_epoch - incident_start + 2))"
  baseline_seconds="$((incident_start - baseline_start + 2))"
  if ((baseline_seconds < 5)); then
    baseline_seconds=5
  fi

  output="$(scenario_compose "${scenario}" exec -T faultmap \
    faultmap diagnose incident \
    --config /etc/faultmap/faultmap.yaml \
    --service "${service}" \
    --environment demo \
    --since "${incident_seconds}s" \
    --baseline "${baseline_seconds}s" \
    --until "${until_rfc}" \
    --limit 500 2>&1)"
  printf '%s\n' "${output}"

  assert_contains "${output}" "Baseline:"
  assert_contains "${output}" "Incidente:"
  assert_contains "${output}" "1. ${service}"
  assert_contains "${output}" "Confiança: alta"
  for finding in $(scenario_findings "${scenario}"); do
    assert_contains "${output}" "ID da regra: ${finding}"
  done
  if [[ "${scenario}" == "timeout-after-deploy" ]]; then
    assert_contains "${output}" "O commit corresponde à service.version observada no incidente."
  fi
  printf 'E2E %s: PASS\n' "${scenario}"
}

cd "${REPOSITORY_ROOT}"
for scenario in "${scenarios[@]}"; do
  run_scenario "${scenario}"
done

printf '\nMatriz E2E concluída: %s\n' "${scenarios[*]}"
