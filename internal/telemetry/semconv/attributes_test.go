package semconv_test

import (
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/semconv"
)

// TestPrecedênciaAceitaAsDuasConvenções fixa o contrato: a convenção estável do
// OpenTelemetry vence, e a anterior é aceita como alternativa. Quem escolhe o
// nome é a biblioteca de instrumentação, não a aplicação — reconhecer apenas um
// dos dois deixou o Faultmap cego para aplicações inteiras em duas releases.
func TestPrecedênciaAceitaAsDuasConvenções(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		nome      string
		ler       func(map[string]string) string
		estavel   string
		legado    string
		atributos map[string]string
	}{
		{
			nome: "código HTTP", ler: semconv.HTTPStatusCode,
			estavel: "http.response.status_code", legado: "http.status_code",
		},
		{
			nome: "sistema de banco", ler: semconv.DatabaseSystem,
			estavel: "db.system.name", legado: "db.system",
		},
		{
			nome: "operação de banco", ler: semconv.DatabaseOperation,
			estavel: "db.operation.name", legado: "db.operation",
		},
		{
			nome: "tipo da falha", ler: semconv.FailureType,
			estavel: "error.type", legado: "exception.type",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.nome, func(t *testing.T) {
			t.Parallel()

			if valor := testCase.ler(map[string]string{testCase.estavel: "atual"}); valor != "atual" {
				t.Fatalf("convenção estável %q não foi reconhecida: %q", testCase.estavel, valor)
			}
			if valor := testCase.ler(map[string]string{testCase.legado: "anterior"}); valor != "anterior" {
				t.Fatalf("convenção anterior %q não foi reconhecida: %q", testCase.legado, valor)
			}
			// Com as duas presentes, a estável decide. A escolha é nossa e precisa
			// ser a mesma em todo o produto, senão a tela e os detectores voltam a
			// discordar sobre o mesmo span.
			ambas := map[string]string{testCase.estavel: "atual", testCase.legado: "anterior"}
			if valor := testCase.ler(ambas); valor != "atual" {
				t.Fatalf("com as duas convenções presentes venceu %q, esperado a estável", valor)
			}
			if valor := testCase.ler(map[string]string{}); valor != "" {
				t.Fatalf("atributo ausente devolveu %q, esperado vazio", valor)
			}
			if valor := testCase.ler(map[string]string{testCase.estavel: "   "}); valor != "" {
				t.Fatalf("valor em branco devolveu %q, esperado vazio", valor)
			}
		})
	}
}

// TestFailureMessageUsaStatusAntesDaExceção garante a ordem escolhida para a
// mensagem de falha: o status do span é o resumo que a instrumentação produz
// para o span inteiro, enquanto a exceção descreve um evento dentro dele.
func TestFailureMessageUsaStatusAntesDaExceção(t *testing.T) {
	t.Parallel()

	valor := semconv.FailureMessage(map[string]string{
		"status.message":    "QueryCanceled: statement timeout",
		"exception.message": "canceling statement",
	})
	if valor != "QueryCanceled: statement timeout" {
		t.Fatalf("FailureMessage() = %q, esperado a mensagem do status", valor)
	}
}
