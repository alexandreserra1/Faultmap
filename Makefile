.PHONY: fmt fmt-check verify test test-race vet demo-up demo-down demo-logs demo-test-e2e demo-test-hard

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
verify: fmt-check vet test test-race

# test executa a suíte de testes do projeto.
test:
	go test ./...

# test-race executa a suíte de testes com detecção de condições de corrida.
test-race:
	go test -race ./...

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
