package identifier_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/platform/identifier"
)

var instante = time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)

// TestSingularidade cobre a primeira exigência: conteúdos diferentes nunca
// produzem o mesmo identificador. Colisão aqui não é registro perdido — é
// registro trocado, porque a persistência usa ON CONFLICT(id) DO NOTHING e a
// segunda mudança seria descartada em silêncio como se já existisse.
func TestSingularidade(t *testing.T) {
	t.Parallel()

	vistos := make(map[string]string, 8192)
	entradas := 0
	for _, base := range []string{"demo", "payments", "catalog"} {
		for _, tabela := range []string{"payments", "pedidos", "clientes"} {
			for coluna := 0; coluna < 800; coluna++ {
				chave := []string{base, tabela, fmt.Sprintf("coluna_%d", coluna)}
				canonica := strings.Join(chave, "|")
				entradas++
				gerado := identifier.New(instante, chave...)
				if anterior, colidiu := vistos[gerado]; colidiu {
					t.Fatalf("colisão entre %q e %q no identificador %q", anterior, canonica, gerado)
				}
				vistos[gerado] = canonica
			}
		}
	}
	// Um a um: qualquer entrada distinta precisa ter identificador distinto.
	if len(vistos) != entradas {
		t.Fatalf("identificadores distintos = %d para %d entradas distintas", len(vistos), entradas)
	}
}

// TestSingularidadeSeparaInstantesIguaisDeConteudosDiferentes garante que o
// prefixo de tempo não engula a distinção: dois objetos observados na mesma
// coleta precisam continuar distinguíveis.
func TestSingularidadeSeparaInstantesIguaisDeConteudosDiferentes(t *testing.T) {
	t.Parallel()

	primeiro := identifier.New(instante, "demo", "payments.amount")
	segundo := identifier.New(instante, "demo", "payments.currency")
	if primeiro == segundo {
		t.Fatalf("dois conteúdos no mesmo instante produziram %q", primeiro)
	}
}

// TestClassificabilidade é a propriedade que o formato anterior não tinha: o
// identificador começava pelo nome do objeto, então ordenar por ID ordenava por
// tabela e não por tempo. Com o instante à frente, a ordenação lexicográfica é
// a ordenação cronológica, e o índice do SQLite serve às duas de uma vez.
func TestClassificabilidade(t *testing.T) {
	t.Parallel()

	// Nomes em ordem inversa de propósito: se o conteúdo vazasse para o começo,
	// a ordenação seguiria o nome em vez do tempo.
	gerados := []string{
		identifier.New(instante.Add(2*time.Hour), "demo", "aaa"),
		identifier.New(instante, "demo", "zzz"),
		identifier.New(instante.Add(time.Hour), "demo", "mmm"),
	}
	ordenados := append([]string(nil), gerados...)
	sort.Strings(ordenados)

	esperado := []string{gerados[1], gerados[2], gerados[0]}
	for posicao := range esperado {
		if ordenados[posicao] != esperado[posicao] {
			t.Fatalf("ordem lexicográfica não é cronológica:\n got = %v\nwant = %v", ordenados, esperado)
		}
	}
}

// TestClassificabilidadeDistingueMilissegundos fixa a resolução: coletas
// separadas por menos de um milissegundo compartilham prefixo e caem no
// desempate por conteúdo, que é estável.
func TestClassificabilidadeDistingueMilissegundos(t *testing.T) {
	t.Parallel()

	anterior := identifier.New(instante, "demo", "x")
	posterior := identifier.New(instante.Add(time.Millisecond), "demo", "x")
	if !(anterior < posterior) {
		t.Fatalf("%q não ordena antes de %q com 1 ms de diferença", anterior, posterior)
	}
}

// TestTamanhoCompacto fixa o comprimento em 26 caracteres, contra os 80 do
// formato anterior. O identificador viaja em JSON, em Markdown, na timeline e
// na resposta do MCP: o tamanho é armazenamento, banda e ruído de leitura.
func TestTamanhoCompacto(t *testing.T) {
	t.Parallel()

	for _, partes := range [][]string{
		{"demo", "payments.amount"},
		{"uma", "base", "com", "muitas", "partes", strings.Repeat("longa", 200)},
		{},
	} {
		gerado := identifier.New(instante, partes...)
		if len(gerado) != 26 {
			t.Fatalf("comprimento = %d para %v, esperado 26 sempre", len(gerado), partes)
		}
	}
}

// TestSemCoordenacao exige que a geração seja uma função pura do instante e do
// conteúdo. Qualquer estado compartilhado — contador, sequência no banco,
// serviço de IDs — seria um ponto único de falha dentro de um produto que se
// distribui como binário único.
func TestSemCoordenacao(t *testing.T) {
	t.Parallel()

	primeiro := identifier.New(instante, "demo", "payments.amount")
	for repeticao := 0; repeticao < 100; repeticao++ {
		if novamente := identifier.New(instante, "demo", "payments.amount"); novamente != primeiro {
			t.Fatalf("repetição %d devolveu %q, esperado %q: há estado escondido", repeticao, novamente, primeiro)
		}
	}
}

// TestDeterminismoPreservaIdempotencia é a razão de não usarmos ULID nem
// UUIDv7 como especificados.
//
// Os dois tiram a unicidade de entropia aleatória, e o produto depende do
// oposto: a persistência é idempotente por ON CONFLICT(id) DO NOTHING, e uma
// coleta repetida com IDs novos gravaria tudo de novo como se fossem mudanças
// inéditas. Aqui a entropia é substituída por um resumo do conteúdo, o que
// entrega as mesmas propriedades e mantém a promessa de mesma entrada, mesma
// saída.
func TestDeterminismoPreservaIdempotencia(t *testing.T) {
	t.Parallel()

	mesmaColeta := []string{"demo", "column", "public.payments.amount", "altered"}
	if identifier.New(instante, mesmaColeta...) != identifier.New(instante, mesmaColeta...) {
		t.Fatal("duas gerações do mesmo conteúdo divergiram; a idempotência da persistência quebraria")
	}
}

// TestAlfabetoNaoTemCaracteresAmbiguos protege quem lê e digita um ID de um
// relatório: o alfabeto exclui I, L, O e U, então não há confusão entre 1 e I,
// 0 e O, nem palavra acidental formada por acaso.
func TestAlfabetoNaoTemCaracteresAmbiguos(t *testing.T) {
	t.Parallel()

	const permitido = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	for indice := 0; indice < 500; indice++ {
		gerado := identifier.New(instante.Add(time.Duration(indice)*time.Second), "demo", string(rune('a'+indice%26)))
		for _, caractere := range gerado {
			if !strings.ContainsRune(permitido, caractere) {
				t.Fatalf("caractere %q fora do alfabeto em %q", caractere, gerado)
			}
		}
	}
}

// TestInstanteZeroNaoQuebra cobre a borda que a coleta pode produzir se um
// relógio vier vazio: o identificador precisa continuar válido em vez de
// devolver algo que a persistência rejeitaria.
func TestInstanteZeroNaoQuebra(t *testing.T) {
	t.Parallel()

	gerado := identifier.New(time.Time{}, "demo", "x")
	if len(gerado) != 26 {
		t.Fatalf("comprimento = %d com instante zero", len(gerado))
	}
}
