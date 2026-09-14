// Package semconv concentra a leitura de atributos que o OpenTelemetry renomeou
// ao estabilizar suas convenções.
//
// Este pacote existe por causa de dois defeitos que chegaram a ser publicados. O
// OpenTelemetry trocou nomes como `http.status_code` por
// `http.response.status_code` e `db.system` por `db.system.name`, mas as
// instrumentações mais usadas continuam emitindo os nomes anteriores. Quem
// escolhe o nome é a biblioteca de instrumentação, não a aplicação: reconhecer
// apenas a convenção nova deixou o Faultmap completamente cego para aplicações
// reais — sem erro, sem aviso, apenas "nenhuma anomalia encontrada".
//
// O agravante das duas vezes foi a duplicação. Os renderizadores aceitavam as
// duas convenções e os detectores só uma, então a tela mostrava "HTTP 200"
// enquanto o detector não via nada. Manter a lista em um lugar só é o que
// impede a tela e a análise de discordarem sobre o mesmo span.
//
// A ordem é sempre a mesma: convenção estável primeiro, anterior depois. Quando
// um span traz as duas com valores diferentes, vence a atual — é uma decisão
// nossa, não uma regra do OpenTelemetry.
package semconv

import "strings"

// Listas de precedência. Acrescentar um nome aqui o torna visível para todo o
// produto de uma vez, que é exatamente o ponto deste pacote.
var (
	httpStatusCodeKeys    = []string{"http.response.status_code", "http.status_code"}
	databaseSystemKeys    = []string{"db.system.name", "db.system"}
	databaseOperationKeys = []string{"db.operation.name", "db.operation"}
	databaseNameKeys      = []string{"db.namespace", "db.name"}
	failureTypeKeys       = []string{"error.type", "exception.type"}
	// O status descreve o span inteiro; a exceção descreve um evento dentro
	// dele. Por isso o status vem primeiro.
	failureMessageKeys = []string{"status.message", "exception.message"}

	httpMethodKeys = []string{"http.request.method", "http.method"}
	// A rota é o alvo estável de uma chamada. url.template e server.address são
	// alternativas emitidas por instrumentações que não conhecem a rota.
	httpTargetKeys           = []string{"http.route", "url.template", "server.address"}
	databaseCollectionKeys   = []string{"db.collection.name", "db.sql.table"}
	messagingOperationKeys   = []string{"messaging.operation.name", "messaging.operation"}
	messagingDestinationKeys = []string{"messaging.destination.template", "messaging.destination.name"}
)

// HTTPStatusCode devolve o código de resposta HTTP observado no span.
func HTTPStatusCode(attributes map[string]string) string {
	return firstNonEmpty(attributes, httpStatusCodeKeys)
}

// DatabaseSystem devolve o sistema de banco declarado pela instrumentação.
// Qualquer valor é aceito: o Faultmap não conhece motores específicos.
func DatabaseSystem(attributes map[string]string) string {
	return firstNonEmpty(attributes, databaseSystemKeys)
}

// DatabaseOperation devolve a operação de banco, quando a instrumentação a emite.
func DatabaseOperation(attributes map[string]string) string {
	return firstNonEmpty(attributes, databaseOperationKeys)
}

// DatabaseName devolve o nome da base acessada pelo span.
//
// É o que liga uma mudança de catálogo ao serviço que fala com aquela base. Sem
// esse vínculo, a proximidade entre uma migração e um incidente seria só
// coincidência temporal: qualquer migração em qualquer base acusaria qualquer
// serviço.
func DatabaseName(attributes map[string]string) string {
	return firstNonEmpty(attributes, databaseNameKeys)
}

// FailureType devolve o tipo da falha registrada no span.
func FailureType(attributes map[string]string) string {
	return firstNonEmpty(attributes, failureTypeKeys)
}

// FailureMessage devolve a descrição textual da falha registrada no span.
func FailureMessage(attributes map[string]string) string {
	return firstNonEmpty(attributes, failureMessageKeys)
}

// HTTPMethod devolve o método da requisição observada no span.
func HTTPMethod(attributes map[string]string) string {
	return firstNonEmpty(attributes, httpMethodKeys)
}

// HTTPTarget devolve a rota chamada, ou a melhor alternativa disponível.
func HTTPTarget(attributes map[string]string) string {
	return firstNonEmpty(attributes, httpTargetKeys)
}

// DatabaseCollection devolve a tabela ou coleção acessada pela operação.
func DatabaseCollection(attributes map[string]string) string {
	return firstNonEmpty(attributes, databaseCollectionKeys)
}

// MessagingOperation devolve a operação de mensageria registrada no span.
func MessagingOperation(attributes map[string]string) string {
	return firstNonEmpty(attributes, messagingOperationKeys)
}

// MessagingDestination devolve o destino da mensagem, preferindo o template ao
// nome concreto para não multiplicar rótulos por fila dinâmica.
func MessagingDestination(attributes map[string]string) string {
	return firstNonEmpty(attributes, messagingDestinationKeys)
}

// firstNonEmpty percorre as convenções na ordem de precedência e devolve o
// primeiro valor útil, tratando espaços em branco como ausência.
func firstNonEmpty(attributes map[string]string, keys []string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(attributes[key]); value != "" {
			return value
		}
	}
	return ""
}
