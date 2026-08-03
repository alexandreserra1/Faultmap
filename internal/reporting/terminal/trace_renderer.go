package terminal

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/evidencegraph"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// RenderTraceInvestigation escreve o fluxo de um trace usando apenas atributos
// permitidos, sem expor SQL bruto, credenciais ou outros valores arbitrários.
func RenderTraceInvestigation(writer io.Writer, traceID string, signals []domain.Signal, graph evidencegraph.Graph) error {
	var output strings.Builder
	fmt.Fprintf(&output, "Investigação do trace — %s\n", traceID)
	if len(signals) == 0 {
		output.WriteString("\nNenhum sinal encontrado para este trace.\n")
		if _, err := io.WriteString(writer, output.String()); err != nil {
			return fmt.Errorf("escrever investigação do trace: %w", err)
		}
		return nil
	}

	services := traceServices(signals)
	if len(services) == 1 {
		fmt.Fprintf(&output, "Serviço: %s\n", services[0])
	} else {
		fmt.Fprintf(&output, "Serviços: %s\n", strings.Join(services, ", "))
	}
	output.WriteString("\nGrafo de evidências:\n")

	orderedSignals := append([]domain.Signal(nil), signals...)
	sort.Slice(orderedSignals, func(first, second int) bool {
		if !orderedSignals[first].Timestamp.Equal(orderedSignals[second].Timestamp) {
			return orderedSignals[first].Timestamp.Before(orderedSignals[second].Timestamp)
		}
		return orderedSignals[first].ID < orderedSignals[second].ID
	})
	spanLabels := make(map[string]string, len(orderedSignals))
	for _, signal := range orderedSignals {
		spanLabels["span:"+signal.SpanID] = spanName(signal)
	}
	relations := traceRelations(graph, spanLabels)
	for _, signal := range orderedSignals {
		fmt.Fprintf(&output, "- %s\n", spanName(signal))
		fmt.Fprintf(&output, "  %s\n", strings.Join(traceDetails(signal), " · "))
		for _, relation := range relations["span:"+signal.SpanID] {
			fmt.Fprintf(&output, "  └─ %s\n", relation)
		}
	}

	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("escrever investigação do trace: %w", err)
	}
	return nil
}

func traceServices(signals []domain.Signal) []string {
	set := make(map[string]struct{})
	for _, signal := range signals {
		if serviceName := strings.TrimSpace(signal.ServiceName); serviceName != "" {
			set[serviceName] = struct{}{}
		}
	}
	services := make([]string, 0, len(set))
	for serviceName := range set {
		services = append(services, serviceName)
	}
	sort.Strings(services)
	return services
}

func traceDetails(signal domain.Signal) []string {
	details := make([]string, 0, 5)
	if statusCode := firstAttribute(signal.Attributes, "http.response.status_code", "http.status_code"); statusCode != "" {
		details = append(details, "HTTP "+statusCode)
	}
	if system := displayDatabaseSystem(firstAttribute(signal.Attributes, "db.system.name", "db.system")); system != "" {
		details = append(details, system)
	}
	if operation := firstAttribute(signal.Attributes, "db.operation.name"); operation != "" {
		details = append(details, "operação "+operation)
	}
	if errorType := firstAttribute(signal.Attributes, "error.type", "exception.type"); errorType != "" {
		details = append(details, "erro "+errorType)
	}
	details = append(details, "duração "+duration(signal))
	return details
}

func traceRelations(graph evidencegraph.Graph, spanLabels map[string]string) map[string][]string {
	relations := make(map[string][]string)
	for _, edge := range graph.Edges {
		if edge.Relation != evidencegraph.RelationQueries {
			continue
		}
		targetLabel := spanLabels[edge.To]
		if targetLabel == "" {
			targetLabel = strings.TrimPrefix(edge.To, "span:")
		}
		relations[edge.From] = append(relations[edge.From], "consulta → "+targetLabel)
	}
	for source := range relations {
		sort.Strings(relations[source])
	}
	return relations
}
