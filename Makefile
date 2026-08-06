.PHONY: fmt test test-race vet demo-up demo-down demo-logs

# fmt aplica a formatação padrão do Go em todos os pacotes do módulo.
fmt:
	go fmt ./...

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
