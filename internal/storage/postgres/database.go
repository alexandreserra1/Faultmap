// Package postgres implementa o mesmo contrato de armazenamento do Faultmap
// sobre PostgreSQL.
//
// O SQLite continua sendo o padrão: o produto se vende como binário único e
// local-first, e exigir um servidor de banco mataria isso. Este backend existe
// para o caso em que o Faultmap deixa de rodar na máquina de quem investiga e
// passa a ser compartilhado por um time — quando o escritor único do SQLite
// vira um gargalo e a telemetria não cabe mais em um arquivo.
//
// A equivalência entre os dois não é uma promessa deste pacote: ela é cobrada
// pela bateria em internal/storage/storagetest, que roda igual contra ambos.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	// O driver database/sql do pgx já era dependência direta por causa da
	// coleta de catálogo; usá-lo aqui evita trazer um segundo driver para o
	// binário só para persistir.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	// O SQLite limita o pool a uma conexão porque só admite um escritor. O
	// PostgreSQL não tem essa restrição, mas um teto baixo continua sendo o
	// certo: o Faultmap é um processo de linha de comando que faz rajadas
	// curtas, não um servidor de aplicação.
	maxOpenConnections = 8
	maxIdleConnections = 4
)

// Open cria o pool PostgreSQL pertencente ao processo.
//
// A DSN nunca vem do faultmap.yaml. O `init` promete um arquivo de configuração
// sem credenciais, e gravar usuário e senha nele quebraria essa promessa de um
// jeito difícil de desfazer — o arquivo tende a ir para o controle de versão. O
// chamador lê a DSN do ambiente e a entrega aqui já resolvida.
//
// O chamador é dono do pool retornado e deve fechá-lo no encerramento controlado.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("abrir banco PostgreSQL: DSN é obrigatória")
	}

	database, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir banco PostgreSQL: %w", err)
	}

	database.SetMaxOpenConns(maxOpenConnections)
	database.SetMaxIdleConns(maxIdleConnections)

	if err := database.PingContext(ctx); err != nil {
		return closeAfterFailure(database, "conectar ao banco PostgreSQL", err)
	}
	return database, nil
}

// closeAfterFailure preserva o erro da operação e também informa uma falha ao fechar o pool.
func closeAfterFailure(database *sql.DB, operacao string, causa error) (*sql.DB, error) {
	if closeErr := database.Close(); closeErr != nil {
		return nil, fmt.Errorf("%s: %w; fechar banco PostgreSQL: %v", operacao, causa, closeErr)
	}
	return nil, fmt.Errorf("%s: %w", operacao, causa)
}

// placeholders monta $n consecutivos a partir de um deslocamento.
//
// O SQLite aceita `?` posicional e o PostgreSQL exige `$1..$n` numerados, então
// as consultas com lista variável — escopo de serviços, bases, SHAs — precisam
// numerar os parâmetros em vez de repetir um símbolo. Os valores continuam
// entrando como parâmetros; nada é concatenado no SQL.
func placeholders(quantidade, deslocamento int) string {
	partes := make([]string, 0, quantidade)
	for indice := 0; indice < quantidade; indice++ {
		partes = append(partes, fmt.Sprintf("$%d", deslocamento+indice))
	}
	return strings.Join(partes, ", ")
}
