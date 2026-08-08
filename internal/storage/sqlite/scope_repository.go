package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ScopeRepository descobre quais serviços pertencem a uma investigação.
//
// Ele existe porque diagnosticar um serviço por vez nunca produz mais de um
// suspeito: o ranking não tinha o que comparar. O escopo é derivado dos traces
// que atravessaram o serviço de entrada, ou seja, do caminho que a requisição
// de fato percorreu — e não de um palpite sobre quem seria o culpado.
type ScopeRepository struct {
	database *sql.DB
}

// NewScopeRepository liga o repositório ao pool criado durante o bootstrap.
func NewScopeRepository(database *sql.DB) *ScopeRepository {
	return &ScopeRepository{database: database}
}

// ListServicesSharingTraces devolve o serviço de entrada e todos os que
// aparecem nos mesmos traces dentro da janela, em ordem estável, junto da
// quantidade de traces considerados.
//
// A descoberta acontece em uma única consulta com subconsulta limitada: uma
// varredura por serviço seria um N+1 disfarçado. A ordenação determinística faz
// o mesmo incidente produzir sempre o mesmo escopo, o que é indispensável para
// que dois diagnósticos iguais sejam comparáveis.
func (repository *ScopeRepository) ListServicesSharingTraces(
	ctx context.Context,
	entryService string,
	start time.Time,
	end time.Time,
	limit int,
) ([]string, int, error) {
	entryService = strings.TrimSpace(entryService)
	if entryService == "" {
		return nil, 0, fmt.Errorf("descobrir escopo: serviço de entrada é obrigatório")
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("descobrir escopo: limite deve ser maior que zero")
	}
	if !start.Before(end) {
		return nil, 0, fmt.Errorf("descobrir escopo: início da janela deve preceder o fim")
	}

	// maxScopeTraces limita quantos traces do serviço de entrada alimentam a
	// expansão. Um incidente com muito volume não precisa de todos eles para
	// revelar quem participou do caminho.
	const maxScopeTraces = 1_000

	rows, err := repository.database.QueryContext(ctx, `
		SELECT DISTINCT service_name
		FROM signals
		WHERE service_name IS NOT NULL AND service_name != ''
			AND trace_id IN (
				SELECT DISTINCT trace_id
				FROM signals
				WHERE service_name = ?
					AND timestamp >= ? AND timestamp < ?
					AND trace_id IS NOT NULL AND trace_id != ''
				ORDER BY trace_id ASC
				LIMIT ?
			)
		ORDER BY service_name ASC
		LIMIT ?
	`, entryService, start.UTC(), end.UTC(), maxScopeTraces, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("descobrir escopo do serviço %q: %w", entryService, err)
	}
	defer rows.Close()

	services := make([]string, 0, limit)
	for rows.Next() {
		var service string
		if err := rows.Scan(&service); err != nil {
			return nil, 0, fmt.Errorf("ler serviço do escopo: %w", err)
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterar serviços do escopo: %w", err)
	}

	traceCount, err := repository.countScopeTraces(ctx, entryService, start, end, maxScopeTraces)
	if err != nil {
		return nil, 0, err
	}
	// Sem traces, a expansão não tem base; o serviço de entrada continua sendo
	// investigado sozinho em vez de o comando falhar.
	if len(services) == 0 {
		services = []string{entryService}
	}
	return services, traceCount, nil
}

// countScopeTraces informa quantos traces sustentaram a expansão, para que a
// saída possa declarar a origem do escopo em vez de apresentá-lo como dado.
func (repository *ScopeRepository) countScopeTraces(
	ctx context.Context,
	entryService string,
	start time.Time,
	end time.Time,
	maxScopeTraces int,
) (int, error) {
	var count int
	if err := repository.database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT DISTINCT trace_id
			FROM signals
			WHERE service_name = ?
				AND timestamp >= ? AND timestamp < ?
				AND trace_id IS NOT NULL AND trace_id != ''
			LIMIT ?
		)
	`, entryService, start.UTC(), end.UTC(), maxScopeTraces).Scan(&count); err != nil {
		return 0, fmt.Errorf("contar traces do escopo de %q: %w", entryService, err)
	}
	return count, nil
}

// ListServicesInWindow enumera todos os serviços com telemetria na janela, em
// ordem estável. Serve ao modo de varredura, para quem não tem um serviço de
// entrada por onde começar.
func (repository *ScopeRepository) ListServicesInWindow(
	ctx context.Context,
	start time.Time,
	end time.Time,
	limit int,
) ([]string, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("listar serviços: limite deve ser maior que zero")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("listar serviços: início da janela deve preceder o fim")
	}

	rows, err := repository.database.QueryContext(ctx, `
		SELECT DISTINCT service_name
		FROM signals
		WHERE service_name IS NOT NULL AND service_name != ''
			AND timestamp >= ? AND timestamp < ?
		ORDER BY service_name ASC
		LIMIT ?
	`, start.UTC(), end.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("listar serviços da janela: %w", err)
	}
	defer rows.Close()

	services := make([]string, 0, limit)
	for rows.Next() {
		var service string
		if err := rows.Scan(&service); err != nil {
			return nil, fmt.Errorf("ler serviço da janela: %w", err)
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar serviços da janela: %w", err)
	}
	return services, nil
}
