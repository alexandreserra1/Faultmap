package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrEmptyCollection sinaliza uma coleta vazia sobre uma base que tinha
// objetos, situação que quase sempre é falha de coleta, não migração.
var ErrEmptyCollection = errors.New("coleta do catálogo veio vazia")

// SchemaObjectKind identifica a natureza do objeto observado no catálogo.
type SchemaObjectKind string

const (
	// SchemaObjectTable identifica uma tabela.
	SchemaObjectTable SchemaObjectKind = "table"
	// SchemaObjectColumn identifica uma coluna, nomeada como tabela.coluna.
	SchemaObjectColumn SchemaObjectKind = "column"
	// SchemaObjectIndex identifica um índice.
	SchemaObjectIndex SchemaObjectKind = "index"
	// SchemaObjectConstraint identifica uma restrição.
	SchemaObjectConstraint SchemaObjectKind = "constraint"
)

// SchemaChangeKind identifica o que aconteceu com o objeto entre duas coletas.
type SchemaChangeKind string

const (
	// SchemaChangeAdded indica objeto ausente na coleta anterior.
	SchemaChangeAdded SchemaChangeKind = "added"
	// SchemaChangeRemoved indica objeto ausente na coleta atual.
	SchemaChangeRemoved SchemaChangeKind = "removed"
	// SchemaChangeAltered indica objeto presente nas duas coletas, com diferença.
	SchemaChangeAltered SchemaChangeKind = "altered"
)

// SchemaObject é uma entrada do catálogo já reduzida ao que o Faultmap guarda.
//
// Detail carrega apenas metadado — tipo de dado, por exemplo. A ADR 0014 impede
// que expressões de DEFAULT e de CHECK cheguem aqui: elas carregam valor e
// regra de negócio, exatamente o que a política de privacidade barra na
// ingestão de telemetria.
type SchemaObject struct {
	Kind SchemaObjectKind
	Name string
	// TableName é a tabela a que o objeto pertence, sem o schema.
	//
	// Existe porque a instrumentação real frequentemente não emite o nome da
	// base: medindo 199 spans de banco de uma aplicação instrumentada, havia
	// `db.collection.name` e nunca `db.namespace`. É por este campo que uma
	// migração se liga ao serviço que consulta aquela tabela.
	TableName string
	Detail    string
	// ExpressionDigest é o resumo da expressão de DEFAULT ou de CHECK. Guardar
	// o resumo, e não o texto, é o que permite acusar a mudança sem armazenar o
	// valor: dois resumos diferentes provam que a expressão mudou e não dizem
	// qual ela era.
	ExpressionDigest string
}

// DigestExpression reduz uma expressão do catálogo a um resumo de tamanho fixo.
//
// O tamanho fixo importa tanto quanto a irreversibilidade: um resumo que
// crescesse com a entrada vazaria o comprimento da expressão, e comprimento é
// informação sobre o conteúdo. Expressão ausente produz resumo vazio, para não
// se confundir com uma expressão real cujo resumo por acaso fosse curto.
func DigestExpression(expression string) string {
	if strings.TrimSpace(expression) == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(expression))
	return hex.EncodeToString(digest[:16])
}

// SchemaSnapshot é uma leitura completa do catálogo de uma base, em um instante.
type SchemaSnapshot struct {
	ID           string
	DatabaseName string
	CapturedAt   time.Time
	Objects      []SchemaObject
}

// SchemaChange descreve uma diferença observada entre duas coletas do catálogo.
//
// Não existe campo de instante, e a ausência é deliberada: comparar duas fotos
// do catálogo localiza a mudança em um intervalo, não em um momento. Os dois
// campos de observação preservam esse intervalo para que o detector possa
// declará-lo como limitação em vez de inventar uma precisão que não foi medida.
type SchemaChange struct {
	ID           string
	DatabaseName string
	// TableName permite ligar a mudança ao serviço pela tabela quando a
	// telemetria não nomeia a base.
	TableName      string
	ObjectKind     SchemaObjectKind
	ObjectName     string
	ChangeKind     SchemaChangeKind
	Detail         string
	ObservedAfter  time.Time
	ObservedBefore time.Time
}

// Validate rejeita mudanças que o detector não conseguiria localizar no tempo
// nem ligar a um serviço.
func (change SchemaChange) Validate() error {
	if strings.TrimSpace(change.ID) == "" {
		return fmt.Errorf("validar mudança de schema: ID é obrigatório")
	}
	if strings.TrimSpace(change.DatabaseName) == "" {
		return fmt.Errorf("validar mudança de schema %q: base é obrigatória", change.ID)
	}
	if strings.TrimSpace(change.ObjectName) == "" {
		return fmt.Errorf("validar mudança de schema %q: objeto é obrigatório", change.ID)
	}
	if change.ObservedAfter.IsZero() || change.ObservedBefore.IsZero() {
		return fmt.Errorf("validar mudança de schema %q: intervalo de observação é obrigatório", change.ID)
	}
	return nil
}

// CheckCollection recusa a comparação quando a coleta atual tem a assinatura de
// uma leitura cega, e não de uma migração.
//
// Esta é a única falha da coleta que o PostgreSQL não reporta como erro:
// information_schema filtra por privilégio. Um usuário que perdeu SELECT nas
// tabelas recebe zero linhas com sucesso, e sem esta verificação isso seria
// lido como "o schema inteiro foi removido" — no incidente seguinte, todo
// serviço que fala com a base apareceria acusado por uma mudança que nunca
// houve, com evidência produzida por uma permissão revogada.
//
// O critério é a ausência total de colunas, e não a de uma classe qualquer,
// porque medimos o comportamento real do servidor: com o usuário sem
// privilégio, `information_schema.columns` devolve zero linhas enquanto
// `pg_indexes` continua devolvendo os índices. A coleta cega não chega vazia,
// chega sem colunas.
//
// Coluna é também a classe onde o critério não tem falso positivo: tabela não
// existe sem coluna, e índice não existe sem tabela. Um catálogo que tinha
// colunas e passa a não ter nenhuma, ainda exibindo outros objetos, descreve um
// estado que não existe em banco nenhum — só em uma leitura incompleta.
//
// As outras classes ficam de fora de propósito. Dropar o único índice de uma
// base é migração legítima, e é justamente o incidente que esta funcionalidade
// existe para explicar: recusá-la calaria o produto no seu melhor caso.
func CheckCollection(previous, current SchemaSnapshot) error {
	previousByKind := countByKind(previous.Objects)
	currentByKind := countByKind(current.Objects)

	previousTotal := 0
	for _, count := range previousByKind {
		previousTotal += count
	}
	currentTotal := 0
	for _, count := range currentByKind {
		currentTotal += count
	}

	switch {
	case previousTotal == 0:
		// Sem linha de base não há remoção a inventar.
		return nil
	case currentTotal == 0:
		return fmt.Errorf(
			"%w: a base %q tinha %d objetos na coleta anterior e nenhum agora; "+
				"verifique se o usuário ainda tem SELECT no catálogo e se a DSN aponta para a base certa",
			ErrEmptyCollection, current.DatabaseName, previousTotal,
		)
	case previousByKind[SchemaObjectColumn] > 0 && currentByKind[SchemaObjectColumn] == 0:
		return fmt.Errorf(
			"%w: na base %q, nenhuma coluna foi lida — havia %d na coleta anterior — "+
				"enquanto outros objetos continuam visíveis; um catálogo assim não existe, "+
				"verifique se o usuário ainda tem SELECT nas tabelas",
			ErrEmptyCollection, current.DatabaseName, previousByKind[SchemaObjectColumn],
		)
	default:
		return nil
	}
}

func countByKind(objects []SchemaObject) map[SchemaObjectKind]int {
	counted := make(map[SchemaObjectKind]int, 4)
	for _, object := range objects {
		if strings.TrimSpace(object.Name) == "" {
			continue
		}
		counted[object.Kind]++
	}
	return counted
}

// Diff compara duas coletas do catálogo da mesma base e devolve as diferenças
// em ordem estável.
//
// Sem coleta anterior não há mudança: a primeira execução estabelece a linha de
// base. Tratar o schema inteiro como recém-criado faria o primeiro diagnóstico
// depois da instalação acusar mudança em todo serviço.
//
// A comparação vê o efeito, não o comando. Uma coluna renomeada aparece como
// uma remoção e uma adição, porque é isso que duas fotos do catálogo mostram.
func Diff(previous, current SchemaSnapshot) []SchemaChange {
	if previous.CapturedAt.IsZero() {
		return nil
	}

	previousByKey := indexObjects(previous.Objects)
	currentByKey := indexObjects(current.Objects)

	changes := make([]SchemaChange, 0, len(previousByKey)+len(currentByKey))
	for key, currentObject := range currentByKey {
		previousObject, existed := previousByKey[key]
		switch {
		case !existed:
			changes = append(changes, newSchemaChange(current, previous, currentObject, SchemaChangeAdded, ""))
		case previousObject.Detail != currentObject.Detail ||
			previousObject.ExpressionDigest != currentObject.ExpressionDigest:
			changes = append(changes, newSchemaChange(
				current, previous, currentObject, SchemaChangeAltered,
				alteredDetail(previousObject, currentObject),
			))
		}
	}
	for key, previousObject := range previousByKey {
		if _, exists := currentByKey[key]; !exists {
			changes = append(changes, newSchemaChange(current, previous, previousObject, SchemaChangeRemoved, ""))
		}
	}

	SortSchemaChanges(changes)
	return changes
}

// SortSchemaChanges impõe a ordem que o produto apresenta.
//
// É exportada porque a leitura do banco precisa reproduzir exatamente o mesmo
// critério do Diff. Duas ordenações escritas separadamente divergem — foi assim
// que a lista de convenções do OpenTelemetry acabou duplicada entre detectores e
// renderizadores, duas vezes, e o produto passou a discordar de si mesmo.
func SortSchemaChanges(changes []SchemaChange) {
	sort.Slice(changes, func(first, second int) bool {
		if changes[first].ObjectName != changes[second].ObjectName {
			return changes[first].ObjectName < changes[second].ObjectName
		}
		if changes[first].ObjectKind != changes[second].ObjectKind {
			return changes[first].ObjectKind < changes[second].ObjectKind
		}
		if changes[first].ChangeKind != changes[second].ChangeKind {
			return changes[first].ChangeKind < changes[second].ChangeKind
		}
		return changes[first].ID < changes[second].ID
	})
}

func indexObjects(objects []SchemaObject) map[string]SchemaObject {
	indexed := make(map[string]SchemaObject, len(objects))
	for _, object := range objects {
		if strings.TrimSpace(object.Name) == "" {
			continue
		}
		indexed[string(object.Kind)+"\x00"+object.Name] = object
	}
	return indexed
}

func newSchemaChange(
	current, previous SchemaSnapshot,
	object SchemaObject,
	kind SchemaChangeKind,
	detail string,
) SchemaChange {
	return SchemaChange{
		ID: strings.Join([]string{
			"schema", current.DatabaseName, string(object.Kind), object.Name,
			string(kind), current.CapturedAt.UTC().Format(time.RFC3339Nano),
		}, ":"),
		DatabaseName:   current.DatabaseName,
		TableName:      object.TableName,
		ObjectKind:     object.Kind,
		ObjectName:     object.Name,
		ChangeKind:     kind,
		Detail:         detail,
		ObservedAfter:  previous.CapturedAt,
		ObservedBefore: current.CapturedAt,
	}
}

// alteredDetail descreve a diferença observada sem revelar expressão nenhuma.
//
// Tipo de dado é metadado e aparece por extenso. A expressão de DEFAULT ou de
// CHECK aparece apenas como o fato de ter mudado: ela carrega valor e regra de
// negócio, e a ADR 0014 a mantém fora da persistência. Quem investiga recebe o
// ponteiro exato — base, tabela e coluna — para olhar no banco, que é onde a
// informação já está.
func alteredDetail(previous, current SchemaObject) string {
	parts := make([]string, 0, 2)
	if previous.Detail != current.Detail {
		parts = append(parts, fmt.Sprintf("passou de %s para %s", previous.Detail, current.Detail))
	}
	if previous.ExpressionDigest != current.ExpressionDigest {
		parts = append(parts, "a expressão associada mudou (o Faultmap registra a mudança, não o texto)")
	}
	return strings.Join(parts, "; ")
}
