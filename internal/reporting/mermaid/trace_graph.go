// Package mermaid transforma grafos internos em diagramas textuais reproduzíveis.
package mermaid

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/evidencegraph"
)

// RenderTraceGraph escreve um flowchart Mermaid determinístico, usando IDs
// sintéticos e rótulos escapados para não interpretar telemetria como sintaxe.
func RenderTraceGraph(writer io.Writer, graph evidencegraph.Graph) error {
	nodes := append([]evidencegraph.Node(nil), graph.Nodes...)
	sort.Slice(nodes, func(first, second int) bool { return nodes[first].ID < nodes[second].ID })
	nodeAliases := make(map[string]string, len(nodes))

	var output strings.Builder
	output.WriteString("flowchart TD\n")
	for index, node := range nodes {
		if strings.TrimSpace(node.ID) == "" {
			return fmt.Errorf("renderizar Mermaid: nó sem ID")
		}
		if _, duplicate := nodeAliases[node.ID]; duplicate {
			return fmt.Errorf("renderizar Mermaid: nó duplicado %q", node.ID)
		}
		alias := fmt.Sprintf("n%d", index)
		nodeAliases[node.ID] = alias
		fmt.Fprintf(&output, "  %s[\"%s\"]\n", alias, escapeLabel(node.Label))
	}

	edges := append([]evidencegraph.Edge(nil), graph.Edges...)
	sort.Slice(edges, func(first, second int) bool {
		firstKey := edges[first].From + "\x00" + string(edges[first].Relation) + "\x00" + edges[first].To
		secondKey := edges[second].From + "\x00" + string(edges[second].Relation) + "\x00" + edges[second].To
		return firstKey < secondKey
	})
	for _, edge := range edges {
		from, fromExists := nodeAliases[edge.From]
		to, toExists := nodeAliases[edge.To]
		if !fromExists || !toExists {
			return fmt.Errorf("renderizar Mermaid: aresta %q -> %q referencia nó ausente", edge.From, edge.To)
		}
		relation, err := relationLabel(edge.Relation)
		if err != nil {
			return err
		}
		fmt.Fprintf(&output, "  %s -->|%s| %s\n", from, relation, to)
	}

	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("escrever Mermaid: %w", err)
	}
	return nil
}

func relationLabel(relation evidencegraph.Relation) (string, error) {
	switch relation {
	case evidencegraph.RelationContains:
		return "contém", nil
	case evidencegraph.RelationQueries:
		return "consulta", nil
	default:
		return "", fmt.Errorf("renderizar Mermaid: relação desconhecida %q", relation)
	}
}

// escapeLabel neutraliza caracteres que poderiam encerrar o rótulo Mermaid ou
// introduzir HTML, preservando o texto compreensível para investigação.
func escapeLabel(label string) string {
	label = strings.ReplaceAll(label, "\r", " ")
	label = strings.ReplaceAll(label, "\n", " ")
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"\"", "&quot;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(label)
}
