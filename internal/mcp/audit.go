package mcp

import (
	"fmt"
	"io"
	"time"
)

// auditor registra uma linha por chamada de ferramenta.
type auditor struct {
	writer io.Writer
	now    func() time.Time
}

func newAuditor(writer io.Writer, now func() time.Time) *auditor {
	return &auditor{writer: writer, now: now}
}

// record grava a chamada com instante, ferramenta e desfecho.
//
// A saída vai para stderr, nunca para stdout: stdout é o transporte JSON-RPC, e
// uma linha de log ali vira mensagem malformada para o cliente. É o tipo de
// defeito que só aparece quando alguém liga um cliente de verdade.
//
// Os argumentos ficam de fora de propósito. Eles carregam identificadores de
// incidente, e a trilha de auditoria não precisa virar um segundo lugar onde
// esses identificadores se acumulam. Nome da ferramenta e desfecho respondem à
// pergunta que uma auditoria faz — o que foi consultado e se funcionou.
func (audit *auditor) record(tool string, cause error) {
	if audit == nil || audit.writer == nil {
		return
	}
	outcome := "ok"
	if cause != nil {
		outcome = "erro"
	}
	_, _ = fmt.Fprintf(
		audit.writer, "%s tool=%s resultado=%s\n",
		audit.now().UTC().Format(time.RFC3339), tool, outcome,
	)
}
