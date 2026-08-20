package detection

// commonCauses associa cada regra às explicações que costumam produzir aquele
// padrão de sinais.
//
// Isto existe por causa de um resultado do piloto cego. Diante da evidência "a
// duração p95 das operações duckdb aumentou de 3,54 ms para 150,73 ms" — que
// estava correta, e cuja causa real era exatamente atraso nas consultas — a
// pessoa que investigava formulou a hipótese de SQL injection. Uma medida
// sozinha não evoca a lista que alguém experiente teria de imediato, e sem ela o
// relatório é um termômetro: mostra a febre sem sugerir o que investigar.
//
// Cada texto oferece mais de uma possibilidade, de propósito. O produto não
// afirma causalidade, e uma causa única leria como veredito; duas ou mais
// deixam explícito que são hipóteses a verificar. A ordem dentro de cada frase
// vai do mais frequente ao menos, para que a leitura rápida encontre primeiro o
// palpite mais provável.
var commonCauses = map[string]string{
	RuleErrorRateDelta: "Costuma vir de mudança recém-implantada, dependência indisponível, " +
		"esgotamento de recurso como conexões ou memória, ou entrada inesperada que " +
		"o código não trata.",
	RuleLatencyDelta: "Costuma vir de dependência lenta, contenção por recurso compartilhado, " +
		"aumento de carga sem capacidade correspondente, ou trabalho novo introduzido " +
		"na requisição.",
	RuleDatabaseTimeout: "Costuma vir de lock retido por transação longa, saturação do pool de " +
		"conexões, consulta que passou a varrer a tabela inteira, ou limite de tempo " +
		"reduzido no cliente.",
	RuleTraceCorrelation: "Costuma vir do banco como origem do impacto visto no HTTP, mas também " +
		"pode ser saturação compartilhada entre os dois, ou um limite de tempo do " +
		"cliente que corta a chamada antes de o banco responder.",
	RuleDeploymentProximity: "Costuma vir da mudança de código em si, mas também de migração de schema " +
		"que acompanhou o deploy, configuração diferente no ambiente, ou " +
		"aquecimento após a reinicialização dos processos.",
	RuleRetryStorm: "Costuma vir de limite de tempo curto demais para a operação, política de " +
		"repetição sem espera crescente, ou dependência que falha de forma " +
		"intermitente e faz cada tentativa parecer recuperável.",
	RuleDependencyFailure: "Costuma vir de falha própria do serviço de trás, mas também de contrato " +
		"rompido entre os dois, ou limite de tempo do chamador menor que o tempo " +
		"normal de resposta do chamado.",
	RuleTraceBreak: "Costuma vir de instrumentação removida ou desatualizada em um dos lados, " +
		"cabeçalho de contexto perdido por proxy ou fila no caminho, ou chamada que " +
		"passou a acontecer fora do fluxo da requisição.",
	RuleLogCorrelation: "Costuma vir do mesmo defeito que derruba as requisições, mas também de " +
		"tratamento de erro que registra e segue adiante, ou de nível de log " +
		"alterado junto com a mudança implantada.",
	RuleDatabaseLatencyDelta: "Costuma vir de lock retido por outra transação, saturação do pool de " +
		"conexões, consulta sem índice adequado, crescimento do volume de dados, ou " +
		"lentidão no armazenamento subjacente.",
	RuleDatabaseError: "Costuma vir de violação de restrição por dado inesperado, migração de " +
		"schema incompatível com o código em execução, permissão alterada, ou " +
		"transação abortada por conflito.",
	RuleVersionRegression: "Costuma vir da diferença de código entre as versões, mas também de " +
		"configuração distinta entre as instâncias, ou de rollout parcial em que a " +
		"versão nova ainda não aqueceu caches e conexões.",
}

// CommonCauses devolve o que aquele padrão de sinais costuma significar, ou
// texto vazio para regra desconhecida.
//
// Regra desconhecida acontece de verdade: um snapshot de incidente gravado por
// uma versão anterior pode citar regra que já não existe, e a renderização
// desse histórico não pode falhar por causa disso.
func CommonCauses(rule string) string {
	return commonCauses[rule]
}
