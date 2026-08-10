package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/detection"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"github.com/faultmap/faultmap/internal/ranking"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

const (
	// DefaultMaxScopeServices limita quantos serviços uma investigação compara.
	DefaultMaxScopeServices = 20
	// DefaultScopeDepth mantém a expansão em um salto por padrão. Dentro de um
	// mesmo trace a cadeia inteira já é alcançada nesse nível; saltos adicionais
	// servem para serviços ligados por outros traces.
	DefaultScopeDepth = 1
	// MaxScopeDepth impede que a expansão percorra o sistema inteiro.
	MaxScopeDepth = 5
)

// ScopeDiscovery descreve como o conjunto de serviços investigados foi obtido,
// para que a saída possa declarar a origem do escopo em vez de apresentá-lo
// como um dado sem procedência.
type ScopeDiscovery string

const (
	// ScopeFromTraces indica expansão a partir dos traces do serviço de entrada.
	ScopeFromTraces ScopeDiscovery = "expansão pelos traces do serviço de entrada"
	// ScopeFromEntryService indica investigação restrita ao serviço informado.
	ScopeFromEntryService ScopeDiscovery = "apenas o serviço informado"
	// ScopeFromWindow indica varredura de todos os serviços com telemetria.
	ScopeFromWindow ScopeDiscovery = "todos os serviços com telemetria na janela"
	// ScopeFromExplicitList indica escopo informado na linha de comando.
	ScopeFromExplicitList ScopeDiscovery = "lista informada na linha de comando"
)

// DiagnosisScope registra quais serviços entraram na investigação e por quê.
type DiagnosisScope struct {
	Services   []string
	TraceCount int
	Discovery  ScopeDiscovery
	Truncated  bool
	// Distances registra a quantos saltos de trace cada serviço está do serviço
	// de entrada. Zero é a própria entrada. A distância explica por que um
	// serviço entrou na comparação e permite julgar o quanto ele é próximo.
	Distances map[string]int
	// Depth é a profundidade efetivamente percorrida, que pode ser menor que a
	// pedida quando a topologia se esgota antes.
	Depth int
}

// ScopedDiagnosisRequest descreve uma investigação que compara serviços.
type ScopedDiagnosisRequest struct {
	EntryService string
	Services     []string
	Environment  string
	Windows      incidentdomain.InvestigationWindow
	Limit        int
	MaxServices  int
	NoExpand     bool
	AllServices  bool
	Depth        int
	Ranking      ranking.Config
}

// ScopeReader descobre quais serviços pertencem a uma investigação.
type ScopeReader interface {
	ListServicesSharingTraces(
		ctx context.Context, entryService string, start time.Time, end time.Time, limit int,
	) ([]string, int, error)
	ListServicesSharingTracesWithAny(
		ctx context.Context, serviceNames []string, start time.Time, end time.Time, limit int,
	) ([]string, error)
	ListServicesInWindow(ctx context.Context, start time.Time, end time.Time, limit int) ([]string, error)
}

// ScopedSignalReader carrega os sinais de todos os serviços do escopo de uma vez.
type ScopedSignalReader interface {
	ListByServicesAndWindow(
		ctx context.Context, serviceNames []string, start time.Time, end time.Time, limit int,
	) ([]domain.Signal, error)
}

// ScopedDeploymentReader carrega deployments de todo o escopo em uma consulta.
type ScopedDeploymentReader interface {
	ListDeploymentsForServices(
		ctx context.Context, serviceNames []string, environment string,
		start time.Time, end time.Time, limit int,
	) ([]changedomain.Deployment, error)
}

// DiagnoseIncidentInScope compara vários serviços na mesma investigação.
//
// Antes desta função o diagnóstico analisava um serviço por vez, então o
// ranking nunca tinha o que comparar: havia sempre um único suspeito, e a meta
// de top-3 era verdadeira por vacuidade. O escopo passa a ser derivado dos
// traces que atravessaram o serviço de entrada — o caminho que a requisição de
// fato percorreu — em vez de exigir que a pessoa já saiba quem suspeitar.
//
// Todas as leituras são em lote: uma consulta de sinais por janela e uma de
// deployments para todo o escopo. Consultar por serviço seria um N+1 que
// cresceria justamente quando o incidente é mais amplo.
func DiagnoseIncidentInScope(
	ctx context.Context,
	request ScopedDiagnosisRequest,
	scopeReader ScopeReader,
	signalReader ScopedSignalReader,
	deploymentReader ScopedDeploymentReader,
) (Diagnosis, error) {
	if err := request.validate(); err != nil {
		return Diagnosis{}, err
	}
	if signalReader == nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: leitor de sinais é obrigatório")
	}

	scope, err := resolveScope(ctx, request, scopeReader)
	if err != nil {
		return Diagnosis{}, err
	}

	baseline, err := signalReader.ListByServicesAndWindow(
		ctx, scope.Services, request.Windows.Baseline.Start, request.Windows.Baseline.End, request.Limit,
	)
	if err != nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: carregar baseline: %w", err)
	}
	incident, err := signalReader.ListByServicesAndWindow(
		ctx, scope.Services, request.Windows.Incident.Start, request.Windows.Incident.End, request.Limit,
	)
	if err != nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: carregar incidente: %w", err)
	}

	deployments, err := loadScopeDeployments(ctx, request, scope, deploymentReader)
	if err != nil {
		return Diagnosis{}, err
	}

	baselineByService := signalsByService(baseline)
	incidentByService := signalsByService(incident)
	findings := make([]detection.Finding, 0, len(scope.Services)*3)
	for _, service := range scope.Services {
		input := detection.Input{
			ServiceName: service,
			Baseline:    baselineByService[service],
			Incident:    incidentByService[service],
		}
		findings = append(findings, detection.Run(input)...)
		if deploymentReader == nil {
			continue
		}
		if finding, found := detection.DetectDeploymentProximity(
			input, deploymentsForService(deployments, service), request.Windows.Incident.Start,
		); found {
			findings = append(findings, finding)
		}
	}

	// Estes dois detectores comparam serviços entre si dentro dos mesmos traces,
	// então recebem as janelas inteiras em vez dos sinais de um serviço só. Cada
	// finding já vem com o serviço a que pertence.
	findings = append(findings, detection.DetectDependencyFailure(baseline, incident)...)
	findings = append(findings, detection.DetectTraceBreak(baseline, incident)...)

	suspects, err := ranking.Rank(findings, request.Ranking)
	if err != nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: ranquear suspeitos: %w", err)
	}
	applyDependencyTieBreak(suspects, serviceDepths(incident))

	return Diagnosis{
		// A identidade da investigação é o serviço de entrada e as janelas, e não
		// o escopo descoberto: o escopo varia conforme a telemetria já recebida, e
		// deixá-lo no ID faria dois retries da mesma investigação criarem
		// incidentes diferentes.
		ID:                  diagnosisID(request.EntryService, request.Environment, request.Windows),
		ServiceName:         request.EntryService,
		Environment:         strings.TrimSpace(request.Environment),
		Windows:             request.Windows,
		BaselineSignalCount: len(baseline),
		IncidentSignalCount: len(incident),
		Findings:            findings,
		Suspects:            suspects,
		Scope:               scope,
	}, nil
}

func (request ScopedDiagnosisRequest) validate() error {
	if strings.TrimSpace(request.EntryService) == "" && !request.AllServices && len(request.Services) == 0 {
		return fmt.Errorf("diagnosticar incidente: serviço de entrada é obrigatório")
	}
	if err := request.Windows.Validate(); err != nil {
		return fmt.Errorf("diagnosticar incidente: janelas inválidas: %w", err)
	}
	if err := request.Ranking.Validate(); err != nil {
		return fmt.Errorf("diagnosticar incidente: ranking inválido: %w", err)
	}
	if request.Limit <= 0 {
		return fmt.Errorf("diagnosticar incidente: limite de sinais deve ser maior que zero")
	}
	if request.MaxServices <= 0 {
		return fmt.Errorf("diagnosticar incidente: limite de serviços deve ser maior que zero")
	}
	return nil
}

// resolveScope escolhe os serviços investigados conforme o modo pedido, sempre
// devolvendo uma lista ordenada para que o mesmo incidente produza o mesmo
// escopo em execuções repetidas.
func resolveScope(
	ctx context.Context,
	request ScopedDiagnosisRequest,
	scopeReader ScopeReader,
) (DiagnosisScope, error) {
	switch {
	case len(request.Services) > 0:
		services := normalizeServices(request.Services)
		truncated := false
		if len(services) > request.MaxServices {
			services, truncated = services[:request.MaxServices], true
		}
		return DiagnosisScope{Services: services, Discovery: ScopeFromExplicitList, Truncated: truncated}, nil

	case request.AllServices:
		if scopeReader == nil {
			return DiagnosisScope{}, fmt.Errorf("diagnosticar incidente: descoberta de escopo é obrigatória para --all")
		}
		services, err := scopeReader.ListServicesInWindow(
			ctx, request.Windows.Incident.Start, request.Windows.Incident.End, request.MaxServices+1,
		)
		if err != nil {
			return DiagnosisScope{}, fmt.Errorf("diagnosticar incidente: descobrir serviços da janela: %w", err)
		}
		services, truncated := limitServices(normalizeServices(services), request.MaxServices)
		if len(services) == 0 {
			return DiagnosisScope{}, fmt.Errorf("diagnosticar incidente: nenhum serviço com telemetria na janela")
		}
		return DiagnosisScope{Services: services, Discovery: ScopeFromWindow, Truncated: truncated}, nil

	case request.NoExpand || scopeReader == nil:
		return DiagnosisScope{
			Services:  []string{strings.TrimSpace(request.EntryService)},
			Discovery: ScopeFromEntryService,
		}, nil

	default:
		return expandScopeByTraces(ctx, request, scopeReader)
	}
}

// expandScopeByTraces percorre a topologia em níveis, a partir do serviço de
// entrada, registrando a distância de cada serviço descoberto.
//
// Cada nível é uma consulta em lote sobre os serviços recém-encontrados; a
// busca para assim que um nível não traz ninguém novo, para não gastar consultas
// depois que a topologia se esgotou.
func expandScopeByTraces(
	ctx context.Context,
	request ScopedDiagnosisRequest,
	scopeReader ScopeReader,
) (DiagnosisScope, error) {
	entryService := strings.TrimSpace(request.EntryService)
	depth := request.Depth
	if depth <= 0 {
		depth = DefaultScopeDepth
	}
	if depth > MaxScopeDepth {
		depth = MaxScopeDepth
	}

	distances := map[string]int{entryService: 0}
	discovered := []string{entryService}
	frontier := []string{entryService}
	traceCount := 0
	reachedDepth := 0
	truncated := false

	for level := 1; level <= depth; level++ {
		var neighbours []string
		var err error
		if level == 1 {
			neighbours, traceCount, err = scopeReader.ListServicesSharingTraces(
				ctx, entryService,
				request.Windows.Incident.Start, request.Windows.Incident.End,
				request.MaxServices+1,
			)
		} else {
			neighbours, err = scopeReader.ListServicesSharingTracesWithAny(
				ctx, frontier,
				request.Windows.Incident.Start, request.Windows.Incident.End,
				request.MaxServices+1,
			)
		}
		if err != nil {
			return DiagnosisScope{}, fmt.Errorf("diagnosticar incidente: descobrir escopo: %w", err)
		}

		nextFrontier := make([]string, 0, len(neighbours))
		for _, neighbour := range normalizeServices(neighbours) {
			if _, known := distances[neighbour]; known {
				continue
			}
			if len(discovered) >= request.MaxServices {
				truncated = true
				break
			}
			distances[neighbour] = level
			discovered = append(discovered, neighbour)
			nextFrontier = append(nextFrontier, neighbour)
		}
		if len(nextFrontier) == 0 {
			break
		}
		reachedDepth = level
		frontier = nextFrontier
	}

	services := normalizeServices(discovered)
	// Serviços cortados pelo limite não podem permanecer no mapa de distâncias,
	// que precisa descrever exatamente o escopo apresentado.
	for service := range distances {
		if !containsService(services, service) {
			delete(distances, service)
		}
	}
	return DiagnosisScope{
		Services:   services,
		TraceCount: traceCount,
		Discovery:  ScopeFromTraces,
		Truncated:  truncated,
		Distances:  distances,
		Depth:      reachedDepth,
	}, nil
}

func containsService(services []string, candidate string) bool {
	for _, service := range services {
		if service == candidate {
			return true
		}
	}
	return false
}

// loadScopeDeployments busca as mudanças de todo o escopo em uma única consulta.
func loadScopeDeployments(
	ctx context.Context,
	request ScopedDiagnosisRequest,
	scope DiagnosisScope,
	deploymentReader ScopedDeploymentReader,
) ([]changedomain.Deployment, error) {
	if deploymentReader == nil {
		return nil, nil
	}
	if strings.TrimSpace(request.Environment) == "" {
		return nil, fmt.Errorf("diagnosticar incidente: ambiente é obrigatório para correlacionar deployments")
	}
	deployments, err := deploymentReader.ListDeploymentsForServices(
		ctx,
		scope.Services,
		request.Environment,
		request.Windows.Incident.Start.Add(-detection.DeploymentLookback),
		request.Windows.Incident.Start.Add(time.Nanosecond),
		request.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("diagnosticar incidente: carregar deployments: %w", err)
	}
	return deployments, nil
}

func deploymentsForService(deployments []changedomain.Deployment, service string) []changedomain.Deployment {
	filtered := make([]changedomain.Deployment, 0, len(deployments))
	for _, deployment := range deployments {
		if deployment.ServiceName == service {
			filtered = append(filtered, deployment)
		}
	}
	return filtered
}

func signalsByService(signals []domain.Signal) map[string][]domain.Signal {
	grouped := make(map[string][]domain.Signal)
	for _, signal := range signals {
		grouped[signal.ServiceName] = append(grouped[signal.ServiceName], signal)
	}
	return grouped
}

// normalizeServices remove vazios e duplicatas e ordena, garantindo escopo
// estável entre execuções.
func normalizeServices(services []string) []string {
	seen := make(map[string]struct{}, len(services))
	result := make([]string, 0, len(services))
	for _, service := range services {
		service = strings.TrimSpace(service)
		if service == "" {
			continue
		}
		if _, duplicate := seen[service]; duplicate {
			continue
		}
		seen[service] = struct{}{}
		result = append(result, service)
	}
	sort.Strings(result)
	return result
}

func limitServices(services []string, maximum int) ([]string, bool) {
	if len(services) <= maximum {
		return services, false
	}
	return services[:maximum], true
}

// applyDependencyTieBreak reordena apenas suspeitos empatados em score,
// colocando primeiro quem está mais fundo na cadeia da requisição.
//
// Quando um serviço falha, quem o chamou tende a falhar junto, e ambos chegam a
// 100% de erro. O score não distingue origem de propagação, e o desempate
// alfabético colocava a vítima em primeiro lugar — exatamente o oposto do que o
// produto promete. A profundidade no trace é a evidência disponível: o serviço
// mais distante da borda é o candidato mais plausível à origem.
//
// Isto é um desempate, não um score: ele nunca altera a ordem de suspeitos com
// pontuações diferentes, e é uma heurística sobre a topologia observada — não
// uma prova de causalidade.
func applyDependencyTieBreak(suspects []ranking.Suspect, depthByService map[string]int) {
	if len(suspects) < 2 || len(depthByService) == 0 {
		return
	}
	sort.SliceStable(suspects, func(first, second int) bool {
		if suspects[first].Score != suspects[second].Score {
			return suspects[first].Score > suspects[second].Score
		}
		firstDepth, firstKnown := depthByService[suspects[first].ID]
		secondDepth, secondKnown := depthByService[suspects[second].ID]
		if firstKnown && secondKnown && firstDepth != secondDepth {
			return firstDepth > secondDepth
		}
		return suspects[first].ID < suspects[second].ID
	})
}

// serviceDepths calcula, para cada serviço, a maior profundidade observada de
// seus spans dentro dos traces carregados.
//
// A profundidade é contada seguindo span.parent_id dentro do próprio conjunto
// de sinais: um span cujo pai não foi carregado é tratado como raiz, o que
// mantém o cálculo estável mesmo com telemetria parcial. O limite de saltos
// protege contra um encadeamento cíclico vindo de instrumentação defeituosa.
func serviceDepths(signals []domain.Signal) map[string]int {
	const maxDepthHops = 32

	spanByID := make(map[string]domain.Signal, len(signals))
	for _, signal := range signals {
		if key := signal.TraceID + "\x00" + signal.SpanID; signal.SpanID != "" {
			spanByID[key] = signal
		}
	}

	depths := make(map[string]int, 8)
	for _, signal := range signals {
		// Serviços na borda ficam com profundidade zero, e o registro explícito
		// os distingue de um serviço sobre o qual nada foi observado. Sem isso o
		// desempate os trataria como desconhecidos e voltaria à ordem alfabética.
		if _, registered := depths[signal.ServiceName]; !registered {
			depths[signal.ServiceName] = 0
		}
		depth := 0
		current := signal
		for hop := 0; hop < maxDepthHops; hop++ {
			parentID := strings.TrimSpace(current.Attributes["span.parent_id"])
			if parentID == "" {
				break
			}
			parent, found := spanByID[current.TraceID+"\x00"+parentID]
			if !found {
				break
			}
			depth++
			current = parent
		}
		if depth > depths[signal.ServiceName] {
			depths[signal.ServiceName] = depth
		}
	}
	return depths
}
