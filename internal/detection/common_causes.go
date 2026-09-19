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
	// Esta é a frase mais exigida do produto, porque latência aumenta em quase
	// todo incidente. O confronto com o catálogo do OpenTelemetry Demo mostrou
	// que a versão anterior orientava sobre três das sete classes de falha que
	// produzem latência ali: faltavam pausa de runtime, cache que parou de
	// servir e acúmulo em fila.
	//
	// Os itens são agrupados em vez de enumerados um a um. Uma lista de dez
	// causas não é mais útil que uma de cinco — a literatura de explicabilidade
	// mostra que textos longos são percebidos como mais plausíveis
	// independentemente da qualidade, o que é justamente o efeito a evitar.
	RuleLatencyDelta: "Costuma vir de dependência lenta, contenção por recurso compartilhado " +
		"como CPU, memória ou cache que parou de servir, perda de capacidade por " +
		"instância fora de rotação ou por aumento de carga sem capacidade " +
		"correspondente, acúmulo em fila com consumidor atrasado, pausa de runtime como " +
		"coleta de lixo, ou trabalho novo introduzido na requisição.",
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
	RuleDatabaseLatencyTail: "Costuma vir de lock de tabela retido por uma migração ou por outra " +
		"transação longa, de contenção em poucas linhas muito disputadas, de espera por " +
		"conexão quando o pool esgota em rajadas, ou de uma consulta que só degrada para " +
		"certos parâmetros.",
	RuleDatabaseError: "Costuma vir de violação de restrição por dado inesperado, migração de " +
		"schema incompatível com o código em execução, permissão alterada, ou " +
		"transação abortada por conflito.",
	RuleSchemaChangeProximity: "Costuma vir de coluna adicionada ou removida que o código em " +
		"execução ainda espera de outra forma, índice removido ou ainda não " +
		"construído, tipo alterado que muda o plano de consulta, ou lock retido pela " +
		"própria migração enquanto ela rodava.",
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
