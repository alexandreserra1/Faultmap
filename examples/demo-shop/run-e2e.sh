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

# now_ms usa python3 porque o date do BSD não oferece precisão de milissegundos,
# necessária para comparar o tempo de diagnóstico com a meta do MVP.
now_ms() {
  python3 -c 'import time; print(int(time.time() * 1000))'
}

# suspect_position devolve a posição do serviço no ranking impresso, ou 0 quando
# ele não aparece. A leitura é restrita às linhas numeradas do ranking.
suspect_position() {
  local output="$1"
  local service="$2"
  local position
  for position in 1 2 3; do
    if [[ "${output}" == *"${position}. ${service}"* ]]; then
      printf '%s\n' "${position}"
      return 0
    fi
  done
  printf '0\n'
}

# ranking_section isola o trecho comparável entre duas execuções do mesmo
# diagnóstico. A faixa termina no cabeçalho das evidências porque a última linha
# da saída muda legitimamente quando o snapshot já existe.
ranking_section() {
  printf '%s\n' "$1" | sed -n '/Ranking de suspeitos/,/^Evidências:/p'
}

# assert_provenance confirma a meta de 100% das evidências com proveniência:
# toda evidência precisa citar ao menos um sinal ou uma mudança de origem.
assert_provenance() {
  local report_json="$1"
  printf '%s' "${report_json}" | python3 -c '
import json, sys

document = json.load(sys.stdin)
total = 0
sem_proveniencia = []
for finding in document.get("findings", []):
    for evidence in finding.get("evidence", []):
        total += 1
        if not evidence.get("signal_ids") and not evidence.get("change_ids"):
            sem_proveniencia.append((finding.get("rule_id"), evidence.get("summary")))
if total == 0:
    print("Relatório sem evidências para auditar.", file=sys.stderr)
    sys.exit(1)
if sem_proveniencia:
    for rule, summary in sem_proveniencia:
        print(f"Evidência sem proveniência em {rule}: {summary}", file=sys.stderr)
    sys.exit(1)
print(total)
'
}

# assert_json valida que o artefato é JSON bem formado e não vazio.
assert_json() {
  local content="$1"
  local label="$2"
  if ! printf '%s' "${content}" | python3 -c 'import json,sys; document = json.load(sys.stdin); sys.exit(0 if document else 1)'; then
    printf '%s não é um JSON válido e não vazio.\n' "${label}" >&2
    return 1
  fi
}

# first_trace_id extrai um trace real da telemetria persistida para exportar o
# grafo Mermaid do cenário em vez de um identificador inventado.
first_trace_id() {
  local scenario="$1"
  local service="$2"
  scenario_compose "${scenario}" exec -T faultmap \
    faultmap telemetry list \
    --config /etc/faultmap/faultmap.yaml \
    --service "${service}" --since 1h --limit 1 2>/dev/null |
    sed -n 's/.*trace \([0-9a-f][0-9a-f]*\).*/\1/p' | head -n 1
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

  diagnose() {
    scenario_compose "${scenario}" exec -T faultmap \
      faultmap diagnose incident \
      --config /etc/faultmap/faultmap.yaml \
      --service "${service}" \
      --environment demo \
      --since "${incident_seconds}s" \
      --baseline "${baseline_seconds}s" \
      --until "${until_rfc}" \
      --limit 500 2>&1
  }

  local diagnose_started diagnose_finished diagnose_ms
  diagnose_started="$(now_ms)"
  output="$(diagnose)"
  diagnose_finished="$(now_ms)"
  diagnose_ms="$((diagnose_finished - diagnose_started))"
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

  # Métrica top-1/top-3: o serviço esperado precisa liderar e, por consequência,
  # estar entre os três primeiros suspeitos apresentados.
  local position
  position="$(suspect_position "${output}" "${service}")"
  if [[ "${position}" != "1" ]]; then
    printf 'Top-1 falhou: %s apareceu na posição %s.\n' "${service}" "${position}" >&2
    return 1
  fi

  # Meta do MVP: diagnóstico local abaixo de 10 segundos. O tempo medido inclui
  # o overhead de docker compose exec, portanto é um limite superior honesto.
  if ((diagnose_ms >= 10000)); then
    printf 'Tempo de diagnóstico %s ms excedeu a meta de 10000 ms.\n' "${diagnose_ms}" >&2
    return 1
  fi

  # Estabilidade: repetir a mesma investigação sobre a mesma janela precisa
  # produzir o mesmo ranking e reencontrar o snapshot já gravado.
  local repeated
  repeated="$(diagnose)"
  if [[ "$(ranking_section "${output}")" != "$(ranking_section "${repeated}")" ]]; then
    printf 'Ranking instável entre execuções idênticas.\n' >&2
    diff <(ranking_section "${output}") <(ranking_section "${repeated}") >&2 || true
    return 1
  fi
  assert_contains "${repeated}" "Diagnóstico já existente:"

  local incident_id
  incident_id="$(printf '%s\n' "${output}" | sed -n 's/^Diagnóstico salvo: \(inc_[0-9a-f]*\)$/\1/p' | head -n 1)"
  if [[ -z "${incident_id}" ]]; then
    printf 'Não foi possível recuperar o ID do incidente diagnosticado.\n' >&2
    return 1
  fi

  # Artefatos exigidos pelo .md: JSON, Markdown, Mermaid e timeline, todos
  # derivados do snapshot persistido e validados a cada cenário.
  local report_json report_markdown timeline_json graph_mermaid trace_id evidence_count
  report_json="$(scenario_compose "${scenario}" exec -T faultmap \
    faultmap export report --config /etc/faultmap/faultmap.yaml \
    --incident "${incident_id}" --format json)"
  assert_json "${report_json}" "report.json"
  evidence_count="$(assert_provenance "${report_json}")"

  report_markdown="$(scenario_compose "${scenario}" exec -T faultmap \
    faultmap export report --config /etc/faultmap/faultmap.yaml \
    --incident "${incident_id}" --format markdown)"
  assert_contains "${report_markdown}" "# Diagnóstico do incidente"
  assert_contains "${report_markdown}" "## Ranking de suspeitos"

  timeline_json="$(scenario_compose "${scenario}" exec -T faultmap \
    faultmap export timeline --config /etc/faultmap/faultmap.yaml \
    --incident "${incident_id}")"
  assert_json "${timeline_json}" "timeline.json"
  assert_contains "${timeline_json}" '"incident_window_start"'
  assert_contains "${timeline_json}" '"diagnosis"'

  trace_id="$(first_trace_id "${scenario}" "${service}")"
  if [[ -z "${trace_id}" ]]; then
    printf 'Nenhum trace disponível para exportar o grafo.\n' >&2
    return 1
  fi
  graph_mermaid="$(scenario_compose "${scenario}" exec -T faultmap \
    faultmap export graph --config /etc/faultmap/faultmap.yaml \
    --trace "${trace_id}" --format mermaid)"
  assert_contains "${graph_mermaid}" "flowchart TD"

  # A retenção é exercida contra o banco real do cenário: com a política padrão
  # de 7 dias, nenhuma telemetria recém-ingerida pode ser removida.
  local retention_output
  retention_output="$(scenario_compose "${scenario}" exec -T faultmap \
    faultmap retention apply --config /etc/faultmap/faultmap.yaml)"
  assert_contains "${retention_output}" "Retenção aplicada: 0 sinais removidos"

  printf '\n--- Métricas E2E %s ---\n' "${scenario}"
  printf 'Top-1 (%s): PASS\n' "${service}"
  printf 'Top-3: PASS (posição %s)\n' "${position}"
  printf 'Tempo de diagnóstico: %s ms — PASS (meta < 10000 ms)\n' "${diagnose_ms}"
  printf 'Estabilidade entre execuções: PASS\n'
  printf 'Proveniência: 100%% (%s evidências) — PASS\n' "${evidence_count}"
  printf 'report.json: PASS\nreport.md: PASS\ntimeline.json: PASS\ngraph.mmd: PASS\n'
  printf 'Retenção sem remoção indevida: PASS\n'
  printf 'E2E %s: PASS\n' "${scenario}"
}

if ! command -v python3 >/dev/null 2>&1; then
  printf 'python3 é necessário para medir tempo e validar artefatos do E2E.\n' >&2
  exit 2
fi

cd "${REPOSITORY_ROOT}"
for scenario in "${scenarios[@]}"; do
  run_scenario "${scenario}"
done

printf '\nMatriz E2E concluída: %s\n' "${scenarios[*]}"
