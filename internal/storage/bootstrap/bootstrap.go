// Package bootstrap escolhe o backend de armazenamento a partir da
// configuração e entrega os repositórios já ligados a ele.
//
// Ele existe para que a CLI não precise saber qual banco está atrás. Sem este
// pacote, cada comando teria um `if driver == "postgres"` em volta da abertura
// do pool e da construção de cada repositório — quinze comandos, cinquenta e
// dois pontos de decisão, todos com a mesma resposta.
package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/storage/postgres"
	"github.com/faultmap/faultmap/internal/storage/sqlite"
)

const (
	// DriverSQLite é o padrão e continua sendo: o produto se vende como binário
	// único e local-first, e exigir um servidor de banco mataria isso.
	DriverSQLite = "sqlite"
	// DriverPostgres é a alternativa para quando o Faultmap deixa de rodar na
	// máquina de quem investiga e passa a ser compartilhado.
	DriverPostgres = "postgres"

	// VariavelDSN nomeia a variável de ambiente que carrega a DSN do PostgreSQL.
	//
	// A DSN fica fora do faultmap.yaml de propósito. O `init` promete um arquivo
	// de configuração sem credenciais, e um arquivo que ganha usuário e senha
	// tende a ir para o controle de versão sem que ninguém perceba — o mesmo
	// raciocínio que a ADR 0014 aplica à coleta de catálogo, onde a conexão com
	// a base observada também nunca é gravada.
	VariavelDSN = "FAULTMAP_STORAGE_DSN"
)

// Conexao guarda o pool aberto e o driver que o produziu.
//
// O driver precisa viajar junto com o pool porque a escolha do repositório
// acontece depois da abertura: `*sql.DB` não diz de qual banco veio, e
// construir um repositório PostgreSQL sobre um pool SQLite falharia só na
// primeira consulta.
type Conexao struct {
	database *sql.DB
	driver   string
}

// DB expõe o pool para quem precisa executar SQL fora dos repositórios — hoje,
// apenas os testes que inspecionam o schema.
func (conexao *Conexao) DB() *sql.DB { return conexao.database }

// Driver informa qual backend está atrás desta conexão.
func (conexao *Conexao) Driver() string { return conexao.driver }

// Close encerra o pool. O chamador é dono da conexão e deve fechá-la no
// encerramento controlado.
func (conexao *Conexao) Close() error {
	if conexao == nil || conexao.database == nil {
		return nil
	}
	return conexao.database.Close()
}

// Open abre o pool do backend indicado por driver.
//
// caminho só é usado pelo SQLite; o PostgreSQL lê a DSN do ambiente. Os dois
// parâmetros convivem para que o chamador continue sendo um único ponto de
// bootstrap por comando, em vez de ramificar antes de chamar.
func Open(ctx context.Context, driver string, caminho string) (*Conexao, error) {
	switch normalizarDriver(driver) {
	case DriverSQLite:
		database, err := sqlite.Open(ctx, caminho)
		if err != nil {
			return nil, err
		}
		return &Conexao{database: database, driver: DriverSQLite}, nil
	case DriverPostgres:
		dsn := strings.TrimSpace(os.Getenv(VariavelDSN))
		if dsn == "" {
			return nil, fmt.Errorf(
				"storage.driver é %q mas %s não está definida: exporte a DSN no ambiente, ela não é lida do faultmap.yaml",
				DriverPostgres, VariavelDSN,
			)
		}
		database, err := postgres.Open(ctx, dsn)
		if err != nil {
			return nil, err
		}
		return &Conexao{database: database, driver: DriverPostgres}, nil
	default:
		return nil, fmt.Errorf("storage.driver %q não é suportado: use %q ou %q", driver, DriverSQLite, DriverPostgres)
	}
}

// Migrate aplica as migrations versionadas do backend aberto.
func Migrate(ctx context.Context, conexao *Conexao) error {
	if conexao == nil {
		return fmt.Errorf("aplicar migrations: conexão não inicializada")
	}
	switch conexao.driver {
	case DriverSQLite:
		return sqlite.Migrate(ctx, conexao.database)
	case DriverPostgres:
		return postgres.Migrate(ctx, conexao.database)
	default:
		return fmt.Errorf("aplicar migrations: driver %q não é suportado", conexao.driver)
	}
}

// RepositorioDeSinais reúne as quatro interfaces de sinais declaradas em
// internal/application. O tipo de retorno é a interface, e não a struct
// concreta, porque é ela que a camada de aplicação consome — devolver o
// concreto obrigaria o chamador a saber qual backend está atrás.
type RepositorioDeSinais interface {
	application.SignalStore
	application.SignalReader
	application.TraceSignalReader
	application.ScopedSignalReader
}

// RepositorioDeMudancas cobre commits e deployments.
type RepositorioDeMudancas interface {
	application.ChangeWriter
	application.ScopedDeploymentReader
	ListDeployments(
		ctx context.Context, serviceName string, environment string,
		start time.Time, end time.Time, limit int,
	) ([]changedomain.Deployment, error)
}

// RepositorioDeCatalogo cobre a coleta de schema e a leitura das diferenças.
type RepositorioDeCatalogo interface {
	application.SchemaWriter
	application.ScopedSchemaChangeReader
}

// RepositorioDeDiagnostico cobre a escrita e a leitura de snapshots publicados.
type RepositorioDeDiagnostico interface {
	application.DiagnosisStore
	application.IncidentHistoryReader
}

// NewSignalRepository devolve o repositório de sinais do backend aberto.
func NewSignalRepository(conexao *Conexao) RepositorioDeSinais {
	if conexao.driver == DriverPostgres {
		return postgres.NewSignalRepository(conexao.database)
	}
	return sqlite.NewSignalRepository(conexao.database)
}

// NewScopeRepository devolve o descobridor de escopo do backend aberto.
func NewScopeRepository(conexao *Conexao) application.ScopeReader {
	if conexao.driver == DriverPostgres {
		return postgres.NewScopeRepository(conexao.database)
	}
	return sqlite.NewScopeRepository(conexao.database)
}

// NewChangeRepository devolve o repositório de commits e deployments.
func NewChangeRepository(conexao *Conexao) RepositorioDeMudancas {
	if conexao.driver == DriverPostgres {
		return postgres.NewChangeRepository(conexao.database)
	}
	return sqlite.NewChangeRepository(conexao.database)
}

// NewSchemaRepository devolve o repositório de catálogo do backend aberto.
func NewSchemaRepository(conexao *Conexao) RepositorioDeCatalogo {
	if conexao.driver == DriverPostgres {
		return postgres.NewSchemaRepository(conexao.database)
	}
	return sqlite.NewSchemaRepository(conexao.database)
}

// NewDiagnosisRepository devolve o repositório de snapshots do backend aberto.
func NewDiagnosisRepository(conexao *Conexao) RepositorioDeDiagnostico {
	if conexao.driver == DriverPostgres {
		return postgres.NewDiagnosisRepository(conexao.database)
	}
	return sqlite.NewDiagnosisRepository(conexao.database)
}

// RetentionRepository reúne as duas frentes da retenção: a limpeza de
// telemetria (ADR 0003) e a liberação dos catálogos de schema (ADR 0015).
//
// O tipo existe porque `ApplyRetention` recebe as duas capacidades, e devolver
// só a primeira obrigaria quem chama a fazer uma asserção de tipo para chegar à
// segunda — transformando em verificação de execução o que o compilador resolve.
type RetentionRepository interface {
	application.SignalRetentionRemover
	application.SchemaCatalogPruner
	CountSchemaChanges(ctx context.Context) (int, error)
	CountIntactCatalogs(ctx context.Context, databaseName string) (int, error)
}

// NewRetentionRepository devolve o aplicador de retenção do backend aberto.
func NewRetentionRepository(conexao *Conexao) RetentionRepository {
	if conexao.driver == DriverPostgres {
		return postgres.NewRetentionRepository(conexao.database)
	}
	return sqlite.NewRetentionRepository(conexao.database)
}

// normalizarDriver trata o driver vazio como SQLite.
//
// Uma configuração anterior à existência do campo, ou escrita à mão sem ele,
// continua abrindo o banco local em vez de falhar. É a escolha coerente com o
// SQLite ser o padrão: a ausência de opinião significa o comportamento de
// sempre.
func normalizarDriver(driver string) string {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		return DriverSQLite
	}
	return driver
}
