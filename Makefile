.PHONY: fmt test test-race vet

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
