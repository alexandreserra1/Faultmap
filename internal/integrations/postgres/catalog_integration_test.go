package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/integrations/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// conectarPostgres abre a base indicada por FAULTMAP_TEST_PG_DSN.
//
// Este arquivo existe porque o sqlmock não executa SQL: ele prova o mapeamento
// das linhas e não prova que a consulta é PostgreSQL válido, nem que o catálogo
// contém o que supomos. O primeiro defeito real desta funcionalidade — as
// restrições implícitas de NOT NULL nomeadas com OIDs, que fariam recriar uma
// tabela parecer migração — só apareceu contra um servidor de verdade.
func conectarPostgres(t *testing.T) *sql.DB {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("FAULTMAP_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("FAULTMAP_TEST_PG_DSN não definido; integração PostgreSQL ignorada")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open() erro = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar conexão: %v", closeErr)
		}
	})

	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("PingContext() erro = %v", err)
	}
	return database
}

// prepararSchema cria um schema isolado para o teste e o remove no fim, para
// que execuções concorrentes não enxerguem os objetos umas das outras.
func prepararSchema(t *testing.T, database *sql.DB, ddl ...string) string {
	t.Helper()

	nome := fmt.Sprintf("faultmap_teste_%d", time.Now().UnixNano())
	ctx := context.Background()
	if _, err := database.ExecContext(ctx, "CREATE SCHEMA "+nome); err != nil {
		t.Fatalf("criar schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.ExecContext(context.Background(), "DROP SCHEMA "+nome+" CASCADE"); err != nil {
			t.Errorf("remover schema: %v", err)
		}
	})
	// Sem SET search_path: todo DDL abaixo é qualificado, e o coletor varre as
	// schemas explicitamente. Ajustar o search_path em um *sql.DB seria ilusório
	// de qualquer forma — ele é um pool, e o comando valeria só para a conexão
	// que por acaso o atendeu.
	for _, statement := range ddl {
		if _, err := database.ExecContext(ctx, strings.ReplaceAll(statement, "{schema}", nome)); err != nil {
			t.Fatalf("executar %q: %v", statement, err)
		}
	}
	return nome
}

func coletarDoServidor(t *testing.T, database *sql.DB) changedomain.SchemaSnapshot {
	t.Helper()

	client, err := postgres.NewClient(database, "payments", nil)
	if err != nil {
		t.Fatalf("NewClient() erro = %v", err)
	}
	snapshot, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() erro = %v", err)
	}
	return snapshot
}

func objetosDoSchema(snapshot changedomain.SchemaSnapshot, prefixo string) []changedomain.SchemaObject {
	filtrados := make([]changedomain.SchemaObject, 0, len(snapshot.Objects))
	for _, object := range snapshot.Objects {
		if strings.HasPrefix(object.Name, prefixo) {
			filtrados = append(filtrados, object)
		}
	}
	return filtrados
}

// somenteDoSchema recorta a coleta ao schema do teste.
//
// O coletor varre todas as schemas não-sistema da base, que é o comportamento
// certo em produção e faz um teste enxergar os objetos do outro quando a suíte
// roda contra o mesmo servidor. O recorte mantém cada teste comparando apenas o
// que ele próprio criou.
func somenteDoSchema(snapshot changedomain.SchemaSnapshot, schema string) changedomain.SchemaSnapshot {
	snapshot.Objects = objetosDoSchema(snapshot, schema+".")
	return snapshot
}

// TestIntegracaoConsultasSaoPostgreSQLValido é o mínimo que o mock não cobre:
// as três consultas precisam ser aceitas por um servidor real.
func TestIntegracaoConsultasSaoPostgreSQLValido(t *testing.T) {
	database := conectarPostgres(t)
	nome := prepararSchema(t, database, `
		CREATE TABLE {schema}.pedidos (
			id uuid PRIMARY KEY,
			valor integer NOT NULL,
			plano text DEFAULT 'gratuito'
		)`,
		`CREATE INDEX idx_pedidos_valor ON {schema}.pedidos (valor)`,
	)

	snapshot := somenteDoSchema(coletarDoServidor(t, database), nome)
	if len(snapshot.Objects) == 0 {
		t.Fatal("a coleta contra o servidor real devolveu catálogo vazio")
	}
	if !snapshot.CapturedAt.After(time.Time{}) {
		t.Fatal("coleta sem instante")
	}
}

// TestIntegracaoRecriarTabelaIdenticaNaoAcusaMigracao é o teste do defeito que
// motivou o filtro de restrições internas. O PostgreSQL nomeia as restrições
// implícitas de NOT NULL com OIDs, que mudam quando a tabela é recriada;
// sem o filtro, recriar a mesma tabela viraria migração fantasma no meio de um
// incidente.
func TestIntegracaoRecriarTabelaIdenticaNaoAcusaMigracao(t *testing.T) {
	database := conectarPostgres(t)
	const criar = `
		CREATE TABLE {schema}.pedidos (
			id uuid PRIMARY KEY,
			valor integer NOT NULL,
			plano text DEFAULT 'gratuito'
		)`
	nome := prepararSchema(t, database, criar, `CREATE INDEX idx_pedidos_valor ON {schema}.pedidos (valor)`)

	primeira := somenteDoSchema(coletarDoServidor(t, database), nome)
	for _, statement := range []string{
		"DROP TABLE " + nome + ".pedidos CASCADE",
		strings.ReplaceAll(criar, "{schema}", nome),
		"CREATE INDEX idx_pedidos_valor ON " + nome + ".pedidos (valor)",
	} {
		if _, err := database.ExecContext(context.Background(), statement); err != nil {
			t.Fatalf("executar %q: %v", statement, err)
		}
	}
	segunda := somenteDoSchema(coletarDoServidor(t, database), nome)

	mudancas := changedomain.Diff(primeira, segunda)
	if len(mudancas) != 0 {
		t.Fatalf("recriar a tabela idêntica acusou %d mudanças: %#v", len(mudancas), mudancas)
	}
}

// TestIntegracaoDetectaMigracaoReal fecha o ciclo: um DDL de verdade precisa
// aparecer, com o objeto certo e o tipo de mudança certo.
func TestIntegracaoDetectaMigracaoReal(t *testing.T) {
	database := conectarPostgres(t)
	nome := prepararSchema(t, database, `
		CREATE TABLE {schema}.pedidos (
			id uuid PRIMARY KEY,
			valor integer NOT NULL
		)`,
		`CREATE INDEX idx_pedidos_valor ON {schema}.pedidos (valor)`,
	)

	primeira := somenteDoSchema(coletarDoServidor(t, database), nome)
	for _, statement := range []string{
		"DROP INDEX " + nome + ".idx_pedidos_valor",
		"ALTER TABLE " + nome + ".pedidos ALTER COLUMN valor TYPE bigint",
		"ALTER TABLE " + nome + ".pedidos ADD COLUMN moeda text",
	} {
		if _, err := database.ExecContext(context.Background(), statement); err != nil {
			t.Fatalf("executar %q: %v", statement, err)
		}
	}
	segunda := somenteDoSchema(coletarDoServidor(t, database), nome)

	esperado := map[string]changedomain.SchemaChangeKind{
		nome + ".idx_pedidos_valor": changedomain.SchemaChangeRemoved,
		nome + ".pedidos.valor":     changedomain.SchemaChangeAltered,
		nome + ".pedidos.moeda":     changedomain.SchemaChangeAdded,
	}
	observado := make(map[string]changedomain.SchemaChangeKind)
	for _, mudanca := range changedomain.Diff(primeira, segunda) {
		observado[mudanca.ObjectName] = mudanca.ChangeKind
	}
	for objeto, tipo := range esperado {
		if observado[objeto] != tipo {
			t.Fatalf("objeto %q veio como %q, esperado %q; observadas: %#v", objeto, observado[objeto], tipo, observado)
		}
	}
}

// TestIntegracaoNaoGuardaExpressoesDoCatalogo é a ADR 0014 verificada contra um
// servidor real, com expressões que de fato carregam valor e regra de negócio.
func TestIntegracaoNaoGuardaExpressoesDoCatalogo(t *testing.T) {
	database := conectarPostgres(t)
	const segredo = "plano-empresarial-secreto"
	nome := prepararSchema(t, database, `
		CREATE TABLE {schema}.clientes (
			id uuid PRIMARY KEY,
			plano text DEFAULT '`+segredo+`',
			cpf text CONSTRAINT clientes_cpf_check CHECK (cpf ~ '^[0-9]{11}$')
		)`)

	snapshot := somenteDoSchema(coletarDoServidor(t, database), nome)
	for _, object := range snapshot.Objects {
		for _, vazamento := range []string{segredo, "[0-9]", "{11}", "cpf ~"} {
			if strings.Contains(object.Detail, vazamento) || strings.Contains(object.ExpressionDigest, vazamento) {
				t.Fatalf("objeto %q vazou %q: %#v", object.Name, vazamento, object)
			}
		}
	}

	// O resumo precisa existir para a coluna com default: sem ele, mudar o valor
	// padrão passaria despercebido.
	var plano changedomain.SchemaObject
	for _, object := range snapshot.Objects {
		if object.Name == nome+".clientes.plano" {
			plano = object
		}
	}
	if plano.ExpressionDigest == "" {
		t.Fatalf("coluna com default não produziu resumo: %#v", snapshot.Objects)
	}
}

// TestIntegracaoPerdaDePrivilegioNaoViraRemocaoEmMassa reproduz a falha com um
// usuário real sem privilégio, e não com uma coleta simulada.
//
// É o teste que justifica a forma da guarda: medindo aqui é que se vê que
// `information_schema.columns` devolve zero linhas ao usuário cego enquanto
// `pg_indexes` continua devolvendo os índices. A coleta não chega vazia, chega
// sem colunas — e uma guarda que olhasse só o total deixaria passar exatamente
// isto.
func TestIntegracaoPerdaDePrivilegioNaoViraRemocaoEmMassa(t *testing.T) {
	database := conectarPostgres(t)
	nome := prepararSchema(t, database, `
		CREATE TABLE {schema}.pedidos (id uuid PRIMARY KEY, valor integer NOT NULL)`,
		`CREATE INDEX idx_pedidos_valor ON {schema}.pedidos (valor)`,
	)

	privilegiada := somenteDoSchema(coletarDoServidor(t, database), nome)
	if len(privilegiada.Objects) == 0 {
		t.Fatal("a coleta privilegiada não enxergou o schema do teste")
	}

	cegaDB := conectarComoUsuarioCego(t, database, nome)
	cega := somenteDoSchema(coletarDoServidor(t, cegaDB), nome)

	// A medição que define a forma da guarda: sem colunas, com índices.
	colunasCegas := 0
	indicesCegos := 0
	for _, object := range cega.Objects {
		switch object.Kind {
		case changedomain.SchemaObjectColumn:
			colunasCegas++
		case changedomain.SchemaObjectIndex:
			indicesCegos++
		}
	}
	if colunasCegas != 0 {
		t.Fatalf("o usuário cego enxergou %d colunas; a premissa da guarda mudou", colunasCegas)
	}
	if indicesCegos == 0 {
		t.Skip("este servidor também esconde pg_indexes do usuário cego; a coleta vazia já é coberta")
	}

	if err := changedomain.CheckCollection(privilegiada, cega); err == nil {
		t.Fatalf(
			"a coleta de um usuário sem privilégio passou pela guarda: %d colunas, %d índices",
			colunasCegas, indicesCegos,
		)
	}
	// Sem a guarda, isto é o que seria gravado como migração.
	if mudancas := changedomain.Diff(privilegiada, cega); len(mudancas) == 0 {
		t.Fatal("o Diff não produziu remoções; o teste perdeu o sentido")
	}
}

// conectarComoUsuarioCego cria um papel com CONNECT na base e nenhum privilégio
// no schema, e devolve uma conexão autenticada como ele.
func conectarComoUsuarioCego(t *testing.T, privilegiada *sql.DB, schema string) *sql.DB {
	t.Helper()

	papel := "cego_" + schema
	ctx := context.Background()
	for _, statement := range []string{
		fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD 'cego'", papel),
		fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", schema, papel),
	} {
		if _, err := privilegiada.ExecContext(ctx, statement); err != nil {
			t.Skipf("sem permissão para criar papel de teste (%v); integração de privilégio ignorada", err)
		}
	}
	t.Cleanup(func() {
		limpeza := []string{
			fmt.Sprintf("REVOKE ALL ON SCHEMA %s FROM %s", schema, papel),
			fmt.Sprintf("DROP ROLE IF EXISTS %s", papel),
		}
		for _, statement := range limpeza {
			if _, err := privilegiada.ExecContext(context.Background(), statement); err != nil {
				t.Errorf("limpar papel: %v", err)
			}
		}
	})

	origem, err := url.Parse(strings.TrimSpace(os.Getenv("FAULTMAP_TEST_PG_DSN")))
	if err != nil {
		t.Fatalf("interpretar DSN: %v", err)
	}
	origem.User = url.UserPassword(papel, "cego")
	cega, err := sql.Open("pgx", origem.String())
	if err != nil {
		t.Fatalf("abrir conexão do usuário cego: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := cega.Close(); closeErr != nil {
			t.Errorf("fechar conexão cega: %v", closeErr)
		}
	})
	return cega
}
