package domain

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/platform/identifier"
)

var (
	firstCapture  = time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	secondCapture = time.Date(2026, 9, 3, 11, 0, 0, 0, time.UTC)
)

func snapshotWith(capturedAt time.Time, objects ...SchemaObject) SchemaSnapshot {
	return SchemaSnapshot{
		ID:           "snap:" + capturedAt.Format(time.RFC3339),
		DatabaseName: "payments",
		CapturedAt:   capturedAt,
		Objects:      objects,
	}
}

func column(name, dataType string) SchemaObject {
	return SchemaObject{Kind: SchemaObjectColumn, Name: name, Detail: dataType}
}

func index(name string) SchemaObject {
	return SchemaObject{Kind: SchemaObjectIndex, Name: name}
}

// TestDiffDetectsAddedRemovedAndAltered cobre as três formas de mudança que a
// comparação de dois catálogos consegue observar.
func TestDiffDetectsAddedRemovedAndAltered(t *testing.T) {
	t.Parallel()

	previous := snapshotWith(firstCapture,
		column("payments.amount", "integer"),
		column("payments.legacy_code", "text"),
		index("idx_payments_created_at"),
	)
	current := snapshotWith(secondCapture,
		column("payments.amount", "bigint"),
		column("payments.currency", "text"),
		index("idx_payments_created_at"),
	)

	changes := Diff(previous, current)

	want := []struct {
		object string
		kind   SchemaChangeKind
	}{
		{"payments.amount", SchemaChangeAltered},
		{"payments.currency", SchemaChangeAdded},
		{"payments.legacy_code", SchemaChangeRemoved},
	}
	if len(changes) != len(want) {
		t.Fatalf("Diff() devolveu %d mudanças, esperado %d: %#v", len(changes), len(want), changes)
	}
	for position, expected := range want {
		if changes[position].ObjectName != expected.object || changes[position].ChangeKind != expected.kind {
			t.Fatalf(
				"Diff()[%d] = %s/%s, esperado %s/%s",
				position, changes[position].ObjectName, changes[position].ChangeKind,
				expected.object, expected.kind,
			)
		}
	}
}

// TestDiffMarcaIntervaloEntreAsDuasColetas verifica a limitação central da
// abordagem: a mudança não tem instante, tem intervalo. A ADR 0014 depende
// desses dois campos para poder declarar isso em cada finding.
func TestDiffMarcaIntervaloEntreAsDuasColetas(t *testing.T) {
	t.Parallel()

	changes := Diff(
		snapshotWith(firstCapture, column("payments.amount", "integer")),
		snapshotWith(secondCapture),
	)

	if len(changes) != 1 {
		t.Fatalf("Diff() devolveu %d mudanças, esperado 1", len(changes))
	}
	if !changes[0].ObservedAfter.Equal(firstCapture) {
		t.Fatalf("ObservedAfter = %v, esperado %v", changes[0].ObservedAfter, firstCapture)
	}
	if !changes[0].ObservedBefore.Equal(secondCapture) {
		t.Fatalf("ObservedBefore = %v, esperado %v", changes[0].ObservedBefore, secondCapture)
	}
}

// TestDiffNaoAcusaMudancaQuandoOCatalogoEstaIgual protege contra o pior falso
// positivo possível nesta regra: coletar de novo, sem migração alguma, e o
// produto apontar mudança de schema durante um incidente.
func TestDiffNaoAcusaMudancaQuandoOCatalogoEstaIgual(t *testing.T) {
	t.Parallel()

	objects := []SchemaObject{
		column("payments.amount", "integer"),
		index("idx_payments_created_at"),
	}
	changes := Diff(snapshotWith(firstCapture, objects...), snapshotWith(secondCapture, objects...))

	if len(changes) != 0 {
		t.Fatalf("Diff() devolveu %d mudanças para catálogos iguais: %#v", len(changes), changes)
	}
}

// TestDiffTrataPrimeiraColetaComoLinhaDeBase impede que a primeira execução do
// comando acuse o schema inteiro como recém-criado, o que apontaria mudança em
// todo serviço no primeiro diagnóstico depois de instalar o produto.
func TestDiffTrataPrimeiraColetaComoLinhaDeBase(t *testing.T) {
	t.Parallel()

	changes := Diff(SchemaSnapshot{}, snapshotWith(secondCapture, column("payments.amount", "integer")))

	if len(changes) != 0 {
		t.Fatalf("Diff() sem coleta anterior devolveu %d mudanças: %#v", len(changes), changes)
	}
}

// TestDiffOrdenaEstavelmenteComEntradaEmbaralhada existe porque este projeto já
// perdeu a reprodutibilidade exatamente assim: a proveniência de todos os
// findings mudava conforme a ordem em que os dados vinham do banco. O catálogo
// chega ordenado pelo ORDER BY da coleta, mas a garantia não pode depender de
// uma camada que pode mudar sem que ninguém relacione as duas coisas.
func TestDiffOrdenaEstavelmenteComEntradaEmbaralhada(t *testing.T) {
	t.Parallel()

	previousObjects := []SchemaObject{
		column("payments.amount", "integer"),
		column("payments.legacy_code", "text"),
		index("idx_payments_created_at"),
		index("idx_payments_customer"),
	}
	currentObjects := []SchemaObject{
		column("payments.amount", "bigint"),
		column("payments.currency", "text"),
		index("idx_payments_customer"),
		{Kind: SchemaObjectTable, Name: "refunds"},
	}

	expected := Diff(snapshotWith(firstCapture, previousObjects...), snapshotWith(secondCapture, currentObjects...))
	if len(expected) == 0 {
		t.Fatal("Diff() não produziu mudanças; o teste de ordem precisa de dados")
	}

	shuffler := rand.New(rand.NewSource(1))
	for round := 0; round < 20; round++ {
		shuffledPrevious := append([]SchemaObject(nil), previousObjects...)
		shuffledCurrent := append([]SchemaObject(nil), currentObjects...)
		shuffler.Shuffle(len(shuffledPrevious), func(first, second int) {
			shuffledPrevious[first], shuffledPrevious[second] = shuffledPrevious[second], shuffledPrevious[first]
		})
		shuffler.Shuffle(len(shuffledCurrent), func(first, second int) {
			shuffledCurrent[first], shuffledCurrent[second] = shuffledCurrent[second], shuffledCurrent[first]
		})

		got := Diff(snapshotWith(firstCapture, shuffledPrevious...), snapshotWith(secondCapture, shuffledCurrent...))
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("Diff() na rodada %d divergiu com entrada embaralhada:\n got = %#v\nwant = %#v", round, got, expected)
		}
	}
}

// TestSchemaChangeValidateRejeitaMudancaSemIdentidade impede que uma mudança
// sem base, objeto ou intervalo chegue à persistência, onde o detector não
// conseguiria nem localizá-la no tempo nem ligá-la a um serviço.
func TestSchemaChangeValidateRejeitaMudancaSemIdentidade(t *testing.T) {
	t.Parallel()

	valid := SchemaChange{
		ID:           "schema:payments:column:payments.amount:altered:" + secondCapture.Format(time.RFC3339),
		DatabaseName: "payments", ObjectKind: SchemaObjectColumn, ObjectName: "payments.amount",
		ChangeKind: SchemaChangeAltered, ObservedAfter: firstCapture, ObservedBefore: secondCapture,
	}
	for _, change := range []SchemaChange{
		{},
		{DatabaseName: valid.DatabaseName, ObjectKind: valid.ObjectKind, ObjectName: valid.ObjectName, ChangeKind: valid.ChangeKind, ObservedAfter: valid.ObservedAfter, ObservedBefore: valid.ObservedBefore},
		{ID: valid.ID, ObjectKind: valid.ObjectKind, ObjectName: valid.ObjectName, ChangeKind: valid.ChangeKind, ObservedAfter: valid.ObservedAfter, ObservedBefore: valid.ObservedBefore},
		{ID: valid.ID, DatabaseName: valid.DatabaseName, ObjectKind: valid.ObjectKind, ChangeKind: valid.ChangeKind, ObservedAfter: valid.ObservedAfter, ObservedBefore: valid.ObservedBefore},
		{ID: valid.ID, DatabaseName: valid.DatabaseName, ObjectKind: valid.ObjectKind, ObjectName: valid.ObjectName, ChangeKind: valid.ChangeKind, ObservedAfter: valid.ObservedAfter},
	} {
		if err := change.Validate(); err == nil {
			t.Fatalf("SchemaChange.Validate() erro = nil para %#v", change)
		}
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("SchemaChange.Validate() erro = %v", err)
	}
}

// TestDiffAcusaMudancaDeExpressaoSemGuardarAExpressao é a ADR 0014 em forma de
// teste. `DEFAULT 'acme-corp'` e `CHECK (cpf ~ '^[0-9]{11}$')` carregam valor e
// regra de negócio — a mesma classe de informação que a política de privacidade
// barra na ingestão de telemetria. O produto precisa apontar que a expressão
// mudou, porque isso quebra código em execução, e não pode revelar qual é.
func TestDiffAcusaMudancaDeExpressaoSemGuardarAExpressao(t *testing.T) {
	t.Parallel()

	const (
		antes  = "'plano-gratuito'"
		depois = "'plano-empresarial'"
	)
	previous := snapshotWith(firstCapture, SchemaObject{
		Kind: SchemaObjectColumn, Name: "clientes.plano", Detail: "text",
		ExpressionDigest: DigestExpression(antes),
	})
	current := snapshotWith(secondCapture, SchemaObject{
		Kind: SchemaObjectColumn, Name: "clientes.plano", Detail: "text",
		ExpressionDigest: DigestExpression(depois),
	})

	changes := Diff(previous, current)

	if len(changes) != 1 {
		t.Fatalf("Diff() devolveu %d mudanças, esperado 1: %#v", len(changes), changes)
	}
	if changes[0].ChangeKind != SchemaChangeAltered {
		t.Fatalf("ChangeKind = %s, esperado %s", changes[0].ChangeKind, SchemaChangeAltered)
	}
	for _, leaked := range []string{antes, depois, "plano-gratuito", "plano-empresarial"} {
		if strings.Contains(changes[0].Detail, leaked) {
			t.Fatalf("Detail vazou a expressão %q: %q", leaked, changes[0].Detail)
		}
	}
	if !strings.Contains(changes[0].Detail, "expressão") {
		t.Fatalf("Detail não diz que a expressão mudou: %q", changes[0].Detail)
	}
}

// TestDigestExpressionNaoEhReversivelNemVazaTamanho garante que o resumo não
// funcione como um armazenamento disfarçado da expressão.
func TestDigestExpressionNaoEhReversivelNemVazaTamanho(t *testing.T) {
	t.Parallel()

	curta := DigestExpression("'a'")
	longa := DigestExpression("'" + strings.Repeat("segredo", 200) + "'")

	if len(curta) != len(longa) {
		t.Fatalf("resumos com tamanhos diferentes vazam o tamanho da expressão: %d e %d", len(curta), len(longa))
	}
	if strings.Contains(longa, "segredo") {
		t.Fatalf("o resumo contém a expressão original: %q", longa)
	}
	if DigestExpression("'a'") != curta {
		t.Fatal("DigestExpression não é determinístico")
	}
	if DigestExpression("") != "" {
		t.Fatal("expressão ausente deveria produzir resumo vazio, para não confundir com uma expressão real")
	}
}

// TestCheckCollectionSoRecusaOVazioSobreCatalogoPovoado cobre a regra nos
// quatro estados possíveis. Ela precisa ser cirúrgica: recusar demais tornaria
// impossível observar uma base que legitimamente esvaziou de uma vez.
func TestCheckCollectionSoRecusaOVazioSobreCatalogoPovoado(t *testing.T) {
	t.Parallel()

	povoada := snapshotWith(firstCapture, column("payments.amount", "integer"))
	vazia := snapshotWith(secondCapture)

	for nome, caso := range map[string]struct {
		anterior, atual SchemaSnapshot
		recusa          bool
	}{
		"vazia sobre povoada":     {povoada, vazia, true},
		"vazia sobre vazia":       {snapshotWith(firstCapture), vazia, false},
		"povoada sobre vazia":     {snapshotWith(firstCapture), povoada, false},
		"povoada sobre povoada":   {povoada, snapshotWith(secondCapture, column("payments.amount", "bigint")), false},
		"primeira coleta povoada": {SchemaSnapshot{}, povoada, false},
	} {
		err := CheckCollection(caso.anterior, caso.atual)
		if caso.recusa && err == nil {
			t.Fatalf("%s: CheckCollection() erro = nil", nome)
		}
		if !caso.recusa && err != nil {
			t.Fatalf("%s: CheckCollection() erro = %v", nome, err)
		}
		if caso.recusa && !errors.Is(err, ErrEmptyCollection) {
			t.Fatalf("%s: erro não é ErrEmptyCollection: %v", nome, err)
		}
	}
}

// TestCheckCollectionExplicaOQueVerificar garante que a mensagem leve a pessoa
// à causa provável, em vez de deixá-la caçando uma migração que não houve.
func TestCheckCollectionExplicaOQueVerificar(t *testing.T) {
	t.Parallel()

	err := CheckCollection(
		snapshotWith(firstCapture, column("payments.amount", "integer")),
		snapshotWith(secondCapture),
	)
	if err == nil {
		t.Fatal("CheckCollection() erro = nil")
	}
	for _, esperado := range []string{"payments", "SELECT", "DSN"} {
		if !strings.Contains(err.Error(), esperado) {
			t.Fatalf("mensagem não menciona %q: %v", esperado, err)
		}
	}
}

// TestCheckCollectionRecusaClasseInteiraQueSumiu vem de uma medição contra o
// PostgreSQL: com um usuário sem SELECT nas tabelas,
// `information_schema.columns` devolve zero linhas, mas `pg_indexes` continua
// devolvendo os índices normalmente. A coleta de um usuário cego não chega
// vazia — ela chega mutilada, com uma classe inteira ausente e as outras
// intactas, o que passaria por uma guarda que só olha o total.
//
// O efeito seria "todas as colunas da base foram removidas" durante o próximo
// incidente, com evidência produzida por uma permissão revogada.
func TestCheckCollectionRecusaClasseInteiraQueSumiu(t *testing.T) {
	t.Parallel()

	comColunasEIndices := snapshotWith(firstCapture,
		column("pedidos.id", "uuid"),
		column("pedidos.valor", "integer"),
		index("idx_pedidos_valor"),
	)
	semColunas := snapshotWith(secondCapture, index("idx_pedidos_valor"))

	err := CheckCollection(comColunasEIndices, semColunas)
	if err == nil {
		t.Fatal("CheckCollection() aceitou coleta sem nenhuma coluna, assinatura de privilégio revogado")
	}
	if !errors.Is(err, ErrEmptyCollection) {
		t.Fatalf("erro = %v, esperado ErrEmptyCollection", err)
	}
	if !strings.Contains(err.Error(), "coluna") {
		t.Fatalf("erro = %v, esperado nomear a classe que sumiu", err)
	}
}

// TestCheckCollectionNaoAtrapalhaRemocaoParcial mantém a guarda cirúrgica:
// remover algumas colunas, ou o último objeto de uma classe que já não existia
// antes, é migração legítima e precisa passar.
func TestCheckCollectionNaoAtrapalhaRemocaoParcial(t *testing.T) {
	t.Parallel()

	for nome, caso := range map[string]struct{ anterior, atual SchemaSnapshot }{
		"remove uma coluna de duas": {
			snapshotWith(firstCapture, column("pedidos.id", "uuid"), column("pedidos.valor", "integer")),
			snapshotWith(secondCapture, column("pedidos.id", "uuid")),
		},
		"classe que nunca existiu segue ausente": {
			snapshotWith(firstCapture, column("pedidos.id", "uuid")),
			snapshotWith(secondCapture, column("pedidos.id", "uuid"), column("pedidos.moeda", "text")),
		},
		"troca todos os índices de uma vez": {
			snapshotWith(firstCapture, column("pedidos.id", "uuid"), index("idx_antigo")),
			snapshotWith(secondCapture, column("pedidos.id", "uuid"), index("idx_novo")),
		},
		// O caso canônico da funcionalidade: a base tem um índice só, e ele é
		// dropado. Recusar isso calaria o produto no incidente que ele existe
		// para explicar.
		"dropa o único índice da base": {
			snapshotWith(firstCapture, column("pedidos.id", "uuid"), index("idx_pedidos_valor")),
			snapshotWith(secondCapture, column("pedidos.id", "uuid")),
		},
	} {
		if err := CheckCollection(caso.anterior, caso.atual); err != nil {
			t.Fatalf("%s: CheckCollection() recusou migração legítima: %v", nome, err)
		}
	}
}

// TestDiffProduzIdentificadoresCurtosEOrdenaveis fixa as propriedades exigidas
// dos identificadores de mudança, que o formato anterior não tinha: ele era uma
// concatenação de 80 caracteres começando pelo nome do objeto, então ordenar
// por ID ordenava por tabela e nunca por tempo.
func TestDiffProduzIdentificadoresCurtosEOrdenaveis(t *testing.T) {
	t.Parallel()

	antigas := Diff(
		snapshotWith(firstCapture, column("payments.zzz", "integer")),
		snapshotWith(firstCapture.Add(time.Minute), column("payments.zzz", "bigint")),
	)
	recentes := Diff(
		snapshotWith(secondCapture, column("payments.aaa", "integer")),
		snapshotWith(secondCapture.Add(time.Minute), column("payments.aaa", "bigint")),
	)
	if len(antigas) != 1 || len(recentes) != 1 {
		t.Fatalf("esperado uma mudança de cada lado: %d e %d", len(antigas), len(recentes))
	}

	if len(antigas[0].ID) != identifier.Length {
		t.Fatalf("comprimento do ID = %d, esperado %d", len(antigas[0].ID), identifier.Length)
	}
	// O nome mais recente começa com "a" e o antigo com "z": se o conteúdo
	// mandasse na ordem, a comparação inverteria.
	if !(antigas[0].ID < recentes[0].ID) {
		t.Fatalf("a mudança mais antiga %q não ordena antes da recente %q", antigas[0].ID, recentes[0].ID)
	}
}

// TestDiffMantemIdentificadorEstavelEntreExecucoes protege a idempotência da
// persistência: o mesmo par de coletas precisa produzir os mesmos IDs, senão
// recoletar gravaria tudo de novo como se fossem mudanças inéditas.
func TestDiffMantemIdentificadorEstavelEntreExecucoes(t *testing.T) {
	t.Parallel()

	anterior := snapshotWith(firstCapture, column("payments.amount", "integer"), index("idx_a"))
	atual := snapshotWith(secondCapture, column("payments.amount", "bigint"))

	primeira := Diff(anterior, atual)
	segunda := Diff(anterior, atual)
	if len(primeira) != len(segunda) {
		t.Fatalf("execuções produziram %d e %d mudanças", len(primeira), len(segunda))
	}
	for posicao := range primeira {
		if primeira[posicao].ID != segunda[posicao].ID {
			t.Fatalf("ID divergiu entre execuções: %q e %q", primeira[posicao].ID, segunda[posicao].ID)
		}
	}
}

// TestDiffNaoColideEntreObjetosDaMesmaColeta cobre o risco concreto do resumo:
// duas mudanças da mesma coleta compartilham o prefixo de tempo inteiro, então
// a distinção depende só do conteúdo. Colisão aqui apagaria uma delas em
// silêncio, porque a persistência usa ON CONFLICT DO NOTHING.
func TestDiffNaoColideEntreObjetosDaMesmaColeta(t *testing.T) {
	t.Parallel()

	anteriores := make([]SchemaObject, 0, 500)
	for indice := range 500 {
		anteriores = append(anteriores, column(fmt.Sprintf("payments.coluna_%d", indice), "integer"))
	}
	mudancas := Diff(snapshotWith(firstCapture, anteriores...), snapshotWith(secondCapture))

	if len(mudancas) != 500 {
		t.Fatalf("mudanças = %d, esperado 500 remoções", len(mudancas))
	}
	vistos := make(map[string]string, len(mudancas))
	for _, mudanca := range mudancas {
		if anterior, colidiu := vistos[mudanca.ID]; colidiu {
			t.Fatalf("colisão entre %q e %q no ID %q", anterior, mudanca.ObjectName, mudanca.ID)
		}
		vistos[mudanca.ID] = mudanca.ObjectName
	}
}
