.PHONY: fmt fmt-check verify test test-race test-sorteio vet test-integration demo-up demo-down demo-logs demo-test-e2e demo-test-hard

# fmt aplica a formatação padrão do Go em todos os pacotes do módulo.
fmt:
	go fmt ./...

# fmt-check falha quando algum arquivo está fora da formatação padrão.
# O gofmt -l apenas lista os arquivos e sempre sai com sucesso, então a decisão
# precisa ser tomada a partir da saída, e não do código de retorno dele.
fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		printf 'Arquivos fora do gofmt:\n%s\n' "$$unformatted" >&2; \
		exit 1; \
	fi

# verify reúne as verificações que precisam estar verdes antes de um commit.
#
# Cada alvo decide pelo próprio código de saída, e o make interrompe no primeiro
# que falhar. Filtrar a saída de `go test` com grep para enxugar a leitura
# descarta justamente esse código: o resultado passa a ser o do grep, que devolve
# sucesso quando encontra linhas — ou seja, sucesso exatamente quando há falhas.
verify: fmt-check vet test test-race test-sorteio

# test-sorteio verifica o sorteio do piloto cego. Roda dentro do verify porque
# o sorteio é a única coisa que torna o piloto possível com uma pessoa: se ele
# enviesar, o piloto passa a medir outra coisa e ninguém percebe. Não sobe
# contêiner — são sorteios puros, em segundos.
test-sorteio:
	./examples/pilot/scripts/sortear-incidente-test.sh

# test executa a suíte de testes do projeto.
test:
	go test ./...

# test-race executa a suíte de testes com detecção de condições de corrida.
test-race:
	go test -race ./...

# test-integration sobe um PostgreSQL descartável e roda a suíte contra ele.
#
# Sem FAULTMAP_TEST_PG_DSN os testes de integração se marcam como ignorados e o
# `make verify` segue verde, o que mantém a suíte rodável sem Docker. O que só
# aparece aqui é o que um mock não pode provar: que as consultas são PostgreSQL
# válido e que o catálogo real contém o que supomos. Os dois defeitos de
# catálogo já corrigidos — as restrições internas nomeadas com OID e a colisão
# entre schemas homônimos — apareceram exclusivamente por esta porta.
test-integration:
	@container=faultmap-pg-test; \
	docker rm -f $$container >/dev/null 2>&1 || true; \
	docker run -d --rm --name $$container \
		-e POSTGRES_PASSWORD=faultmap -e POSTGRES_DB=faultmap \
		-p 55432:5432 postgres:16-alpine >/dev/null; \
	trap 'docker rm -f '"$$container"' >/dev/null 2>&1 || true' EXIT INT TERM; \
	for attempt in $$(seq 1 60); do \
		docker exec $$container pg_isready -U postgres >/dev/null 2>&1 && break; \
		sleep 1; \
	done; \
	FAULTMAP_TEST_PG_DSN="postgres://postgres:faultmap@localhost:55432/faultmap?sslmode=disable" \
		go test ./... -run Integracao -count=1

# vet executa as verificações estáticas padrão do Go.
vet:
	go vet ./...

# demo-up constrói e inicia a loja de demonstração e sua infraestrutura local.
demo-up:
	docker compose -f examples/demo-shop/compose.yaml up --build -d --wait

# demo-down encerra a demonstração sem remover os volumes persistentes.
demo-down:
	docker compose -f examples/demo-shop/compose.yaml down

# demo-logs acompanha os logs dos componentes da demonstração.
demo-logs:
	docker compose -f examples/demo-shop/compose.yaml logs -f

# demo-test-e2e executa cenários isolados; E2E_SCENARIOS vazio roda a matriz completa.
demo-test-e2e:
	examples/demo-shop/run-e2e.sh $(E2E_SCENARIOS)

# demo-test-hard executa o modo difícil, que verifica se o Faultmap se cala
# quando não há regressão. HARD_SCENARIOS vazio roda todos os cenários.
demo-test-hard:
	examples/demo-shop/run-hard-mode.sh $(HARD_SCENARIOS)
