// Package postgres lê o catálogo de uma base PostgreSQL sem escrever nela.
//
// A alternativa seria a replicação lógica, que é o caminho das ferramentas de
// CDC. Ela não serve aqui por uma razão que não é de custo: a replicação lógica
// do PostgreSQL não decodifica DDL. Capturar `ALTER TABLE` por ali exigiria
// instalar event triggers na base de quem usa o produto — escrever no banco
// alheio para poder lê-lo. Comparar duas leituras do catálogo custa a precisão
// do instante, que passa a ser um intervalo, e não custa nada ao banco
// observado. A ADR 0014 registra a decisão e o que ela deixa de ver.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/platform/identifier"
)

// systemSchemas ficam de fora: elas descrevem o PostgreSQL, não a aplicação, e
// mudam quando o servidor é atualizado — o que apareceria como migração.
const systemSchemaFilter = `table_schema NOT IN ('pg_catalog', 'information_schema')
	AND table_schema NOT LIKE 'pg_toast%'
	AND table_schema NOT LIKE 'pg_temp%'`

// Client lê o catálogo de uma base nomeada.
type Client struct {
	database     *sql.DB
	databaseName string
	now          func() time.Time
}

// NewClient recebe o pool já aberto para que a escolha de driver e a leitura da
// credencial fiquem na borda, fora do coletor.
func NewClient(database *sql.DB, databaseName string, now func() time.Time) (*Client, error) {
	if database == nil {
		return nil, fmt.Errorf("coletar catálogo: conexão é obrigatória")
	}
	if strings.TrimSpace(databaseName) == "" {
		return nil, fmt.Errorf("coletar catálogo: nome da base é obrigatório")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Client{database: database, databaseName: strings.TrimSpace(databaseName), now: now}, nil
}

// Fetch lê colunas, índices e restrições e devolve a coleta normalizada.
//
// São três consultas somente leitura, cada uma com ORDER BY explícito. A ordem
// vem do servidor e é reimposta aqui: a coleta é gravada e comparada com a
// próxima, e uma ordem instável produziria diferenças falsas a cada execução.
func (client *Client) Fetch(ctx context.Context) (changedomain.SchemaSnapshot, error) {
	objects := make([]changedomain.SchemaObject, 0, 256)

	columns, err := client.fetchColumns(ctx)
	if err != nil {
		return changedomain.SchemaSnapshot{}, err
	}
	objects = append(objects, columns...)

	indexes, err := client.fetchIndexes(ctx)
	if err != nil {
		return changedomain.SchemaSnapshot{}, err
	}
	objects = append(objects, indexes...)

	constraints, err := client.fetchConstraints(ctx)
	if err != nil {
		return changedomain.SchemaSnapshot{}, err
	}
	objects = append(objects, constraints...)

	sort.Slice(objects, func(first, second int) bool {
		if objects[first].Kind != objects[second].Kind {
			return objects[first].Kind < objects[second].Kind
		}
		return objects[first].Name < objects[second].Name
	})

	capturedAt := client.now().UTC()
	return changedomain.SchemaSnapshot{
		// O instante à frente faz a ordenação por ID coincidir com a cronológica,
		// que é como a coleta anterior é buscada. O formato anterior começava
		// pelo literal "catalog", então todas as bases ordenavam juntas e o tempo
		// só desempatava no fim.
		ID:           identifier.New(capturedAt, "catalog", client.databaseName),
		DatabaseName: client.databaseName,
		CapturedAt:   capturedAt,
		Objects:      objects,
	}, nil
}

// fetchColumns lê tabela, coluna, tipo e nulabilidade. A expressão de DEFAULT é
// lida e imediatamente reduzida a resumo: ela carrega valor e regra de negócio,
// e a ADR 0014 a mantém fora da persistência. O resumo basta para acusar que
// mudou, que é o que interessa a quem investiga.
func (client *Client) fetchColumns(ctx context.Context) ([]changedomain.SchemaObject, error) {
	rows, err := client.database.QueryContext(ctx, `
		SELECT table_schema, table_name, column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE `+systemSchemaFilter+`
		ORDER BY table_schema, table_name, column_name
	`)
	if err != nil {
		return nil, fmt.Errorf("consultar colunas da base %q: %w", client.databaseName, err)
	}
	defer func() { _ = rows.Close() }()

	objects := make([]changedomain.SchemaObject, 0, 256)
	for rows.Next() {
		var tableSchema, tableName, columnName, dataType, isNullable string
		var columnDefault sql.NullString
		if err := rows.Scan(&tableSchema, &tableName, &columnName, &dataType, &isNullable, &columnDefault); err != nil {
			return nil, fmt.Errorf("ler coluna do catálogo: %w", err)
		}
		detail := dataType
		if strings.EqualFold(isNullable, "NO") {
			detail += " not null"
		}
		objects = append(objects, changedomain.SchemaObject{
			Kind:             changedomain.SchemaObjectColumn,
			Name:             qualify(tableSchema, tableName, columnName),
			TableName:        tableName,
			Detail:           detail,
			ExpressionDigest: changedomain.DigestExpression(columnDefault.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("percorrer colunas do catálogo: %w", err)
	}
	return objects, nil
}

func (client *Client) fetchIndexes(ctx context.Context) ([]changedomain.SchemaObject, error) {
	rows, err := client.database.QueryContext(ctx, `
		SELECT schemaname, tablename, indexname
		FROM pg_indexes
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY schemaname, indexname
	`)
	if err != nil {
		return nil, fmt.Errorf("consultar índices da base %q: %w", client.databaseName, err)
	}
	defer func() { _ = rows.Close() }()

	objects := make([]changedomain.SchemaObject, 0, 64)
	for rows.Next() {
		var indexSchema, indexTable, indexName string
		if err := rows.Scan(&indexSchema, &indexTable, &indexName); err != nil {
			return nil, fmt.Errorf("ler índice do catálogo: %w", err)
		}
		// A definição do índice não é lida: ela contém expressões, e um índice
		// parcial carrega o predicado inteiro. Nome e existência bastam para
		// acusar que um índice sumiu, que é o caso que quebra consulta.
		objects = append(objects, changedomain.SchemaObject{
			Kind:      changedomain.SchemaObjectIndex,
			Name:      qualify(indexSchema, indexName),
			TableName: indexTable,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("percorrer índices do catálogo: %w", err)
	}
	return objects, nil
}

// fetchConstraints lê nome e tipo. A expressão de CHECK vive em outra tabela do
// catálogo e não é consultada: `CHECK (cpf ~ '^[0-9]{11}$')` é regra de negócio
// e formato de documento, exatamente o que a política de privacidade barra.
//
// As restrições implícitas de NOT NULL ficam de fora, descartadas em Go e não
// no WHERE. A regra vive em um lugar só de propósito: escrevê-la também na
// consulta a tornaria invisível para o teste, que exercita o mapeamento sem
// executar SQL, e este repositório já pagou duas vezes por manter a mesma regra
// em dois lugares que depois discordaram.
// qualify monta o nome do objeto com o schema à frente.
//
// Um banco PostgreSQL quase nunca tem um schema só: há `public` mais os da
// aplicação, e instalações multi-inquilino usam um schema por cliente. Sem o
// schema no nome, `public.pedidos.valor` e `tenant_a.pedidos.valor` viram a
// mesma chave na comparação — uma mudança em um mascara a do outro, e dropar
// uma tabela que existe com o mesmo nome em outro schema aparece como "nada
// mudou". Descoberto pela suíte de integração, quando dois testes criaram
// tabelas homônimas em schemas diferentes na mesma base.
func qualify(parts ...string) string {
	return strings.Join(parts, ".")
}

// isImplicitNotNullConstraint reconhece as restrições que o PostgreSQL cria
// sozinho para cada coluna NOT NULL.
//
// O servidor as nomeia com OIDs — "2200_16385_1_not_null" —, e o OID muda
// quando a tabela é recriada: recriar uma tabela sem alterar nada apareceria
// como uma restrição removida e outra adicionada, ou seja, migração inventada
// durante um incidente. A nulabilidade já viaja no detalhe da coluna, então
// nada se perde ao descartá-las. Descoberto rodando a coleta contra um
// PostgreSQL real, que é o tipo de detalhe que mock nenhum revela.
func isImplicitNotNullConstraint(constraintName string) bool {
	return strings.HasSuffix(constraintName, "_not_null")
}

func (client *Client) fetchConstraints(ctx context.Context) ([]changedomain.SchemaObject, error) {
	rows, err := client.database.QueryContext(ctx, `
		SELECT table_schema, table_name, constraint_name, constraint_type
		FROM information_schema.table_constraints
		WHERE `+systemSchemaFilter+`
		ORDER BY table_schema, constraint_name
	`)
	if err != nil {
		return nil, fmt.Errorf("consultar restrições da base %q: %w", client.databaseName, err)
	}
	defer func() { _ = rows.Close() }()

	objects := make([]changedomain.SchemaObject, 0, 64)
	for rows.Next() {
		var constraintSchema, constraintTable, constraintName, constraintType string
		if err := rows.Scan(&constraintSchema, &constraintTable, &constraintName, &constraintType); err != nil {
			return nil, fmt.Errorf("ler restrição do catálogo: %w", err)
		}
		if isImplicitNotNullConstraint(constraintName) {
			continue
		}
		objects = append(objects, changedomain.SchemaObject{
			Kind:      changedomain.SchemaObjectConstraint,
			Name:      qualify(constraintSchema, constraintName),
			TableName: constraintTable,
			Detail:    strings.ToLower(constraintType),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("percorrer restrições do catálogo: %w", err)
	}
	return objects, nil
}
