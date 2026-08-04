package application

import (
	"context"
	"fmt"
	"strings"
)

// DiagnosisStore define a única gravação atômica necessária para preservar um diagnóstico.
type DiagnosisStore interface {
	Save(ctx context.Context, diagnosis Diagnosis) (created bool, err error)
}

// PersistDiagnosis valida o snapshot e delega sua persistência idempotente sem conter SQL.
func PersistDiagnosis(ctx context.Context, diagnosis Diagnosis, store DiagnosisStore) (bool, error) {
	if strings.TrimSpace(diagnosis.ID) == "" {
		return false, fmt.Errorf("persistir diagnóstico: ID é obrigatório")
	}
	if strings.TrimSpace(diagnosis.ServiceName) == "" {
		return false, fmt.Errorf("persistir diagnóstico: serviço é obrigatório")
	}
	if err := diagnosis.Windows.Validate(); err != nil {
		return false, fmt.Errorf("persistir diagnóstico: janelas inválidas: %w", err)
	}
	expectedID := DiagnosisID(diagnosis.ServiceName, diagnosis.Windows)
	if diagnosis.ID != expectedID {
		return false, fmt.Errorf("persistir diagnóstico: ID não corresponde ao serviço e às janelas")
	}
	if store == nil {
		return false, fmt.Errorf("persistir diagnóstico: repositório é obrigatório")
	}

	created, err := store.Save(ctx, diagnosis)
	if err != nil {
		return false, fmt.Errorf("persistir diagnóstico %q: %w", diagnosis.ID, err)
	}
	return created, nil
}
