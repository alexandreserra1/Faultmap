package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

const (
	maxSchemaObjectsPerSnapshot = 20_000
	maxSchemaChangesPerQuery    = 1_000
)

// SchemaRepository guarda coletas do catálogo e as diferenças entre elas.
type SchemaRepository struct {
	database *sql.DB
}

// NewSchemaRepository injeta o pool compartilhado sem abrir conexões próprias.
func NewSchemaRepository(database *sql.DB) *SchemaRepository {
	return &SchemaRepository{database: database}
}

// SaveSnapshot grava a coleta e as mudanças em relação à coleta anterior da
// mesma base, em uma transação curta.
//
// O diff é calculado aqui, no momento da coleta, e não no diagnóstico. Duas
// razões: o `diagnose` passa a apenas ler, mantendo a investigação sem trabalho
// de derivação; e a mudança fica auditável mesmo depois que a retenção remover
// a coleta anterior que a revelou.
func (repository *SchemaRepository) SaveSnapshot(
	ctx context.Context,
	snapshot changedomain.SchemaSnapshot,
) (application.SchemaImportResult, error) {
	if err := validateSnapshot(snapshot); err != nil {
		return application.SchemaImportResult{}, err
	}
	objectsJSON, err := json.Marshal(snapshot.Objects)
	if err != nil {
		return application.SchemaImportResult{}, fmt.Errorf("serializar catálogo da base %q: %w", snapshot.DatabaseName, err)
	}
	if err := ctx.Err(); err != nil {
		return application.SchemaImportResult{}, fmt.Errorf("salvar catálogo: contexto cancelado: %w", err)
	}

	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return application.SchemaImportResult{}, fmt.Errorf("iniciar transação de catálogo: %w", err)
	}

	previous, err := readPreviousSnapshot(ctx, transaction, snapshot.DatabaseName, snapshot.CapturedAt)
	if err != nil {
		return application.SchemaImportResult{}, rollbackSchemaTransaction(transaction, err)
	}
	// A verificação vem antes de qualquer escrita, e a coleta suspeita não é
	// gravada. Registrá-la faria a comparação seguinte partir de um catálogo
	// vazio e acusar o schema inteiro como recém-criado — o mesmo estrago,
	// adiado em uma coleta.
	if err := changedomain.CheckCollection(previous, snapshot); err != nil {
		return application.SchemaImportResult{}, rollbackSchemaTransaction(transaction, err)
	}

	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO schema_snapshots (id, database_name, captured_at, objects_json)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING
	`, snapshot.ID, snapshot.DatabaseName, snapshot.CapturedAt.UTC(), string(objectsJSON)); err != nil {
		return application.SchemaImportResult{}, rollbackSchemaTransaction(
			transaction, fmt.Errorf("inserir coleta %q: %w", snapshot.ID, err))
	}

	result := application.SchemaImportResult{ObjectsCollected: len(snapshot.Objects)}
	for _, change := range changedomain.Diff(previous, snapshot) {
		if err := change.Validate(); err != nil {
			return application.SchemaImportResult{}, rollbackSchemaTransaction(transaction, err)
		}
		execution, err := transaction.ExecContext(ctx, `
			INSERT INTO schema_changes (
				id, database_name, table_name, object_kind, object_name, change_kind,
				detail, observed_after, observed_before, snapshot_id
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (id) DO NOTHING
		`, change.ID, change.DatabaseName, change.TableName, string(change.ObjectKind), change.ObjectName,
			string(change.ChangeKind), change.Detail,
			change.ObservedAfter.UTC(), change.ObservedBefore.UTC(), snapshot.ID)
		if err != nil {
			return application.SchemaImportResult{}, rollbackSchemaTransaction(
				transaction, fmt.Errorf("inserir mudança de schema %q: %w", change.ID, err))
		}
		rowsAffected, err := execution.RowsAffected()
		if err != nil {
			return application.SchemaImportResult{}, rollbackSchemaTransaction(
				transaction, fmt.Errorf("contar mudança de schema %q inserida: %w", change.ID, err))
		}
		result.ChangesPersisted += int(rowsAffected)
	}

	if err := transaction.Commit(); err != nil {
		return application.SchemaImportResult{}, fmt.Errorf("confirmar catálogo: %w", err)
	}
	return result, nil
}

// ListSchemaChangesForScope lê, em uma consulta, as mudanças de todas as bases
// do escopo investigado.
//
// A consulta é em lote pelo mesmo motivo que a de deployments: perguntar base
// por base seria um N+1 que cresce justamente quando o incidente é mais amplo.
func (repository *SchemaRepository) ListSchemaChangesForScope(
	ctx context.Context,
	databases []string,
	tables []string,
	start time.Time,
	end time.Time,
	limit int,
) ([]changedomain.SchemaChange, error) {
	databaseNames := normalizeDatabaseNames(databases)
	tableNames := normalizeDatabaseNames(tables)
	if len(databaseNames) == 0 && len(tableNames) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > maxSchemaChangesPerQuery {
		limit = maxSchemaChangesPerQuery
	}

	// Os dois alvos entram na mesma consulta, unidos por OR: a telemetria
	// costuma trazer só um deles, e perguntar duas vezes seria duas idas ao
	// banco para responder à mesma pergunta.
	conditions := make([]string, 0, 2)
	arguments := make([]any, 0, len(databaseNames)+len(tableNames)+3)
	proximo := 1
	if len(databaseNames) > 0 {
		conditions = append(conditions, "database_name IN ("+placeholders(len(databaseNames), proximo)+")")
		for _, name := range databaseNames {
			arguments = append(arguments, name)
		}
		proximo += len(databaseNames)
	}
	if len(tableNames) > 0 {
		conditions = append(conditions, "table_name IN ("+placeholders(len(tableNames), proximo)+")")
		for _, name := range tableNames {
			arguments = append(arguments, name)
		}
		proximo += len(tableNames)
	}
	arguments = append(arguments, start.UTC(), end.UTC(), limit)

	query := fmt.Sprintf(`
		SELECT id, database_name, table_name, object_kind, object_name, change_kind,
		       detail, observed_after, observed_before
		FROM schema_changes
		WHERE (%s)
		  AND observed_before >= $%d
		  AND observed_before <= $%d
		ORDER BY observed_before DESC, id ASC
		LIMIT $%d
	`, strings.Join(conditions, " OR "), proximo, proximo+1, proximo+2)

	rows, err := repository.database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("consultar mudanças de schema: %w", err)
	}
	defer func() { _ = rows.Close() }()

	changes := make([]changedomain.SchemaChange, 0, limit)
	for rows.Next() {
		var change changedomain.SchemaChange
		var objectKind, changeKind string
		if err := rows.Scan(
			&change.ID, &change.DatabaseName, &change.TableName, &objectKind, &change.ObjectName,
			&changeKind, &change.Detail, &change.ObservedAfter, &change.ObservedBefore,
		); err != nil {
			return nil, fmt.Errorf("ler mudança de schema: %w", err)
		}
		change.ObjectKind = changedomain.SchemaObjectKind(objectKind)
		change.ChangeKind = changedomain.SchemaChangeKind(changeKind)
		change.ObservedAfter = change.ObservedAfter.UTC()
		change.ObservedBefore = change.ObservedBefore.UTC()
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("percorrer mudanças de schema: %w", err)
	}

	// A ordem apresentada é do produto: o mesmo critério do Diff, para que a
	// evidência não dependa de qual índice o banco escolheu.
	changedomain.SortSchemaChanges(changes)
	return changes, nil
}

// readPreviousSnapshot busca a última coleta anterior da mesma base.
//
// A comparação é sempre dentro da mesma base. Misturar bases faria dois
// catálogos distintos aparecerem como migração massiva a cada coleta alternada.
func readPreviousSnapshot(
	ctx context.Context,
	transaction *sql.Tx,
	databaseName string,
	before time.Time,
) (changedomain.SchemaSnapshot, error) {
	var snapshot changedomain.SchemaSnapshot
	var objectsJSON string
	err := transaction.QueryRowContext(ctx, `
		SELECT id, database_name, captured_at, objects_json
		FROM schema_snapshots
		WHERE database_name = $1 AND captured_at < $2
		ORDER BY captured_at DESC, id ASC
		LIMIT 1
	`, databaseName, before.UTC()).Scan(
		&snapshot.ID, &snapshot.DatabaseName, &snapshot.CapturedAt, &objectsJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return changedomain.SchemaSnapshot{}, nil
	}
	if err != nil {
		return changedomain.SchemaSnapshot{}, fmt.Errorf("ler coleta anterior da base %q: %w", databaseName, err)
	}
	// Catálogo liberado pela retenção (ADR 0015). A política preserva a coleta
	// mais recente de cada base justamente para isto não acontecer; se acontecer
	// mesmo assim, falhar é a única saída honesta. Tratar como catálogo vazio
	// faria a comparação reportar todo objeto da base como recém-criado — uma
	// migração inventada em cada tabela.
	//
	// O backend SQLite tem a mesma guarda. Ela nasceu lá e não foi espelhada
	// aqui de imediato; a bateria de conformidade não pegou porque nenhum caso
	// recoletava depois de podar. O caso existe agora.
	if strings.TrimSpace(objectsJSON) == "" {
		return changedomain.SchemaSnapshot{}, fmt.Errorf(
			"ler coleta anterior da base %q: a coleta %q teve o catálogo liberado pela retenção "+
				"e não serve de linha de base; a próxima coleta com instante posterior ao da "+
				"linha de base preservada volta a comparar normalmente",
			databaseName, snapshot.ID,
		)
	}
	if err := json.Unmarshal([]byte(objectsJSON), &snapshot.Objects); err != nil {
		return changedomain.SchemaSnapshot{}, fmt.Errorf("desserializar coleta %q: %w", snapshot.ID, err)
	}
	snapshot.CapturedAt = snapshot.CapturedAt.UTC()
	return snapshot, nil
}

func validateSnapshot(snapshot changedomain.SchemaSnapshot) error {
	if strings.TrimSpace(snapshot.ID) == "" {
		return fmt.Errorf("salvar catálogo: ID da coleta é obrigatório")
	}
	if strings.TrimSpace(snapshot.DatabaseName) == "" {
		return fmt.Errorf("salvar catálogo %q: base é obrigatória", snapshot.ID)
	}
	if snapshot.CapturedAt.IsZero() {
		return fmt.Errorf("salvar catálogo %q: instante da coleta é obrigatório", snapshot.ID)
	}
	if len(snapshot.Objects) > maxSchemaObjectsPerSnapshot {
		return fmt.Errorf(
			"salvar catálogo %q: %d objetos excedem o limite de %d por coleta",
			snapshot.ID, len(snapshot.Objects), maxSchemaObjectsPerSnapshot,
		)
	}
	return nil
}

func normalizeDatabaseNames(databases []string) []string {
	seen := make(map[string]struct{}, len(databases))
	names := make([]string, 0, len(databases))
	for _, name := range databases {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func rollbackSchemaTransaction(transaction *sql.Tx, cause error) error {
	if rollbackErr := transaction.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; desfazer transação de catálogo: %v", cause, rollbackErr)
	}
	return cause
}
