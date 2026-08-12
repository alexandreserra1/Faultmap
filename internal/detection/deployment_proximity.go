package detection

import (
	"fmt"
	"sort"
	"strings"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	telemetrydomain "github.com/faultmap/faultmap/internal/telemetry/domain"
)

// DeploymentLookback limita quanto tempo antes do incidente ainda sustenta uma hipótese de proximidade.
const DeploymentLookback = time.Hour

// detectDeploymentProximity seleciona o deployment mais forte do mesmo serviço
// dentro da janela anterior ao incidente e verifica a versão observada nos spans.
func detectDeploymentProximity(input Input, deployments []changedomain.Deployment, incidentStart time.Time) (Finding, bool) {
	incidentVersions := observedVersions(input.Incident)
	baselineVersions := observedVersions(input.Baseline)
	candidates := make([]deploymentCandidate, 0, len(deployments))
	for _, deployment := range deployments {
		age := incidentStart.Sub(deployment.DeployedAt)
		if deployment.ServiceName != input.ServiceName || age < 0 || age > DeploymentLookback {
			continue
		}
		_, versionMatches := incidentVersions[deployment.CommitSHA]
		candidates = append(candidates, deploymentCandidate{
			deployment:     deployment,
			age:            age,
			versionMatches: versionMatches,
		})
	}
	if len(candidates) == 0 {
		return Finding{}, false
	}
	sort.Slice(candidates, func(first, second int) bool {
		if candidates[first].versionMatches != candidates[second].versionMatches {
			return candidates[first].versionMatches
		}
		if candidates[first].age != candidates[second].age {
			return candidates[first].age < candidates[second].age
		}
		return candidates[first].deployment.ID < candidates[second].deployment.ID
	})
	selected := candidates[0]
	score := 1 - selected.age.Seconds()/DeploymentLookback.Seconds()
	confidence := ConfidenceLow
	if selected.versionMatches {
		confidence = ConfidenceHigh
	}

	limitations := []string{"Proximidade temporal e correspondência de versão não provam causalidade."}
	if !strings.EqualFold(strings.TrimSpace(selected.deployment.State), "success") {
		limitations = append(limitations, "O status de sucesso do deployment não foi confirmado pela coleta atual.")
	}
	if !selected.versionMatches {
		limitations = append(limitations, "A versão observada no incidente não confirmou o commit do deployment.")
	}
	return Finding{
		Rule:        RuleDeploymentProximity,
		ServiceName: input.ServiceName,
		Score:       clamp(score),
		Confidence:  confidence,
		Evidence: []Evidence{{
			Summary:       deploymentSummary(selected, baselineVersions, incidentVersions),
			ChangeIDs:     []string{selected.deployment.ID},
			BaselineValue: 0,
			IncidentValue: clamp(score),
		}},
		Limitations: limitations,
	}, true
}

// DetectDeploymentProximityFindings devolve a proximidade do deployment acusando
// tanto o serviço quanto o commit implantado.
//
// Antes, o commit existia apenas dentro do texto da evidência do serviço: quem
// lia via "o checkout está suspeito" e precisava caçar, na explicação, qual
// mudança havia entrado. Num incidente causado por deploy, essa é a informação
// mais acionável que existe, e ela ficava escondida.
//
// O commit acusado é sempre o que foi implantado. Um commit que não chegou a ser
// implantado não aparece: ele não tem relação observável com o incidente.
func DetectDeploymentProximityFindings(
	input Input,
	deployments []changedomain.Deployment,
	commitMessages map[string]string,
	incidentStart time.Time,
) []Finding {
	serviceFinding, found := detectDeploymentProximity(input, deployments, incidentStart)
	if !found {
		return nil
	}

	commitSHA := commitSHAFromEvidence(deployments, serviceFinding)
	if commitSHA == "" {
		return []Finding{serviceFinding}
	}

	commitFinding := serviceFinding
	commitFinding.SubjectKind = SubjectCommit
	commitFinding.SubjectID = commitSHA
	commitFinding.SubjectLabel = commitLabel(commitSHA, commitMessages[commitSHA])
	return []Finding{serviceFinding, commitFinding}
}

// commitSHAFromEvidence recupera o commit do deployment que sustentou o finding,
// usando o mesmo critério de seleção do detector para não divergir dele.
func commitSHAFromEvidence(deployments []changedomain.Deployment, finding Finding) string {
	for _, evidence := range finding.Evidence {
		for _, changeID := range evidence.ChangeIDs {
			for _, deployment := range deployments {
				if deployment.ID == changeID {
					return strings.TrimSpace(deployment.CommitSHA)
				}
			}
		}
	}
	return ""
}

// commitLabel monta um rótulo legível: quem investiga não decora SHA.
func commitLabel(commitSHA, message string) string {
	short := commitSHA
	if len(short) > 8 {
		short = short[:8]
	}
	label := "commit " + short
	if trimmed := strings.TrimSpace(message); trimmed != "" {
		label += " — " + trimmed
	}
	return label
}

type deploymentCandidate struct {
	deployment     changedomain.Deployment
	age            time.Duration
	versionMatches bool
}

func deploymentSummary(candidate deploymentCandidate, baselineVersions, incidentVersions map[string]struct{}) string {
	minutes := int(candidate.age.Round(time.Minute) / time.Minute)
	summary := fmt.Sprintf(
		"O deployment %s do commit %s ocorreu %d minuto(s) antes do incidente no ambiente %s.",
		candidate.deployment.ID,
		candidate.deployment.CommitSHA,
		minutes,
		candidate.deployment.Environment,
	)
	if candidate.versionMatches {
		summary += " O commit corresponde à service.version observada no incidente."
	}
	if !sameStringSet(baselineVersions, incidentVersions) && len(baselineVersions) > 0 && len(incidentVersions) > 0 {
		summary += fmt.Sprintf(
			" As versões observadas mudaram de %s para %s.",
			strings.Join(sortedSetValues(baselineVersions), ", "),
			strings.Join(sortedSetValues(incidentVersions), ", "),
		)
	}
	return summary
}

func observedVersions(signals []telemetrydomain.Signal) map[string]struct{} {
	versions := make(map[string]struct{})
	for _, signal := range signals {
		if version := strings.TrimSpace(signal.Attributes["service.version"]); version != "" {
			versions[version] = struct{}{}
		}
	}
	return versions
}

func sameStringSet(first, second map[string]struct{}) bool {
	if len(first) != len(second) {
		return false
	}
	for value := range first {
		if _, exists := second[value]; !exists {
			return false
		}
	}
	return true
}

func sortedSetValues(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
