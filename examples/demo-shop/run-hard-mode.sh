#!/usr/bin/env bash
set -euo pipefail

# Modo difícil: ao contrário da matriz E2E, que verifica se o Faultmap ACUSA o
# serviço certo, estes cenários verificam se ele SE CONTÉM quando não há
# regressão. Falso positivo é o defeito mais caro de um produto de diagnóstico:
# um ranking que sempre acha um culpado é indistinguível de um que adivinha.
#
# Todos os cenários usam telemetria OTLP real, banco isolado e o mesmo binário
# publicado na release. Nenhum deles substitui um incidente de produção.
#
# RODE COM A MÁQUINA EM REPOUSO. Estes cenários medem latência real de processos
# reais, então contenção de CPU aparece como regressão de latência — que é o que
# eles proíbem. Rodando logo depois da matriz E2E, com builds Docker ainda em
# curso, sem-culpado já acusou latência de 9 ms para 64 ms e falhou; sozinho, na
# máquina parada, passou três vezes seguidas com silêncio completo.
#
# Uma falha aqui merece ser repetida isolada antes de virar diagnóstico de
# defeito: `run-hard-mode.sh sem-culpado`.

SCRIPT_DIRECTORY="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPOSITORY_ROOT="$(cd -- "${SCRIPT_DIRECTORY}/../.." && pwd)"
BASE_COMPOSE="${SCRIPT_DIRECTORY}/compose.yaml"
PROJECT_NAME="${FAULTMAP_HARD_PROJECT_NAME:-faultmap-demo-shop-hard}"
CHECKOUT_URL="${FAULTMAP_HARD_CHECKOUT_URL:-http://127.0.0.1:18080/checkout}"
ALL_SCENARIOS="ruido-cronico sem-culpado janela-imprecisa fan-out-legitimo causas-concorrentes migracao-inofensiva"
# O SDK do Go agrupa spans por 5 segundos antes de exportar, e o coletor
# acrescenta o próprio lote. Esperar 6 deixava menos de um segundo de margem, e a
# janela do incidente chegava vazia de forma intermitente — o diagnóstico
# encontrava zero sinais e o cenário falhava sem que nada estivesse errado no
# produto. Dez segundos dão folga suficiente para o caminho inteiro.
OTEL_FLUSH_WAIT_SECONDS="${OTEL_FLUSH_WAIT_SECONDS:-10}"

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

# collect_schema roda a coleta do catálogo contra a base da própria demo.
#
# O modo difícil não coletava schema em cenário nenhum, e por isso a regra de
# proximidade de migração passava por ele sem nunca ser exercitada — assim como
# deployment_proximity, que também depende de uma ingestão que estes cenários
# não fazem. Uma regra que o modo difícil não consegue exercitar não está
# protegida por ele.
collect_schema() {
  compose exec -T \
    -e FAULTMAP_PG_DSN="postgres://demo:demo@postgres:5432/demo?sslmode=disable" \
    faultmap faultmap ingest schema \
    --config /etc/faultmap/faultmap.yaml --database demo 2>&1
}

# apply_harmless_migration adiciona uma coluna que nenhuma consulta usa. É a
# migração mais inofensiva possível: se o produto acusar por causa dela, acusaria
# por qualquer uma.
apply_harmless_migration() {
  compose exec -T postgres \
    psql -U demo -d demo -q -c "ALTER TABLE payments ADD COLUMN observacao_modo_dificil TEXT" >/dev/null
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
  local schema_collected=""
  local acompanhada=0

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
    migracao-inofensiva)
      # Uma migração real aconteceu antes da janela e nada mais mudou.
      #
      # O cenário verifica a corroboração, e não o silêncio absoluto. Silêncio
      # absoluto não é afirmável aqui: estes cenários medem processos reais, e
      # numa máquina ocupada o aquecimento aparece como regressão de latência de
      # poucos milissegundos — legítima, medida, e suficiente para que apresentar
      # a migração passe a ser o comportamento correto. Exigir silêncio faria o
      # cenário medir o quanto a máquina está livre.
      #
      # O que é afirmável sempre: proximidade de mudança é evidência de apoio.
      # Ela não pode ser a única coisa apresentada sobre um serviço, porque uma
      # migração sem nada observado ao redor não sustenta acusação nenhuma. Essa
      # é a invariante que o falso positivo original violava.
      #
      # O outro lado — sistema comprovadamente saudável não produz finding de
      # schema — é garantido de forma determinística pelos testes em Go, onde a
      # telemetria não depende de relógio nem de carga da máquina.
      printf 'Migração aplicada antes da janela; ela só pode aparecer acompanhada.\n'
      service="payment-service"
      start_stack
      collect_schema >/dev/null
      apply_harmless_migration
      schema_collected="$(collect_schema)"
      printf 'Coleta após a migração: %s' "${schema_collected}"
      # Uma rodada descartada antes da baseline. Sem ela, a primeira janela mede
      # processo frio — conexões não abertas, planos não cacheados, JIT do
      # runtime — e a segunda mede processo quente: a diferença aparece como
      # regressão de latência real, e este cenário passaria a depender de quão
      # ocupada está a máquina em vez de medir o que se propõe a medir.
      generate_traffic aquecimento 16 migracao
      sleep "${OTEL_FLUSH_WAIT_SECONDS}"
      baseline_start="$(date +%s)"
      generate_traffic baseline 16 migracao
      sleep "${OTEL_FLUSH_WAIT_SECONDS}"
      incident_start="$(date +%s)"
      generate_traffic incidente 16 migracao
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
      # A falha crônica também atravessa o banco; nenhuma das duas regras de
      # banco pode tratar ruído permanente como crescimento.
      assert_no_finding "${output}" "database_error" || return 1
      assert_no_finding "${output}" "database_timeout" || return 1
      printf 'Não acusou regressão de erro com ruído idêntico nas duas janelas: PASS\n'
      ;;
    migracao-inofensiva)
      # A migração precisa ter sido registrada, senão o cenário passaria por
      # vacuidade: sem mudança no banco, nenhuma regra de schema teria o que
      # disparar e o silêncio não provaria coisa alguma.
      if [[ "${schema_collected}" == *"0 mudanças"* || -z "${schema_collected}" ]]; then
        printf 'A migração não foi registrada; o silêncio seria vacuidade: %s\n' \
          "${schema_collected}" >&2
        return 1
      fi
      # Se a migração aparecer, alguma evidência medida precisa aparecer junto.
      if [[ "${output}" == *"ID da regra: schema_change_proximity"* ]]; then
        acompanhada=0
        for medida in error_rate_delta latency_delta database_latency_delta \
          database_error database_timeout retry_storm dependency_failure \
          trace_break log_correlation version_regression; do
          if [[ "${output}" == *"ID da regra: ${medida}"* ]]; then
            acompanhada=1
            break
          fi
        done
        if (( acompanhada == 0 )); then
          printf 'FALSO POSITIVO: a migração foi apresentada sozinha, sem sintoma medido.\n' >&2
          return 1
        fi
        printf 'Migração apresentada apenas como apoio a evidência medida: PASS\n'
      else
        printf 'Migração sem efeito observável não foi apresentada: PASS\n'
      fi
      ;;
    sem-culpado)
      # Este cenário é a rede de proteção de todo detector novo: nada mudou
      # entre as janelas, então qualquer regra que dispare aqui é falso
      # positivo por definição.
      for regra in error_rate_delta latency_delta retry_storm \
        database_error dependency_failure trace_break version_regression \
        schema_change_proximity; do
        assert_no_finding "${output}" "${regra}" || return 1
      done
      assert_contains "${output}" "Nenhuma anomalia determinística" || return 1
      printf 'Não inventou suspeito em sistema saudável: PASS\n'
      ;;
    janela-imprecisa)
      assert_no_finding "${output}" "error_rate_delta" || return 1
      printf 'Volume irregular não virou falso positivo: PASS\n'
      ;;
    fan-out-legitimo)
      assert_no_finding "${output}" "retry_storm" || return 1
      # O fan-out cria muitas ligações pai-filho entre os dois serviços; elas
      # existem nas duas janelas e não podem virar quebra de propagação.
      assert_no_finding "${output}" "trace_break" || return 1
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
