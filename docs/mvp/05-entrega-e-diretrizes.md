# Entrega e diretrizes de implementação

Este documento é parte obrigatória da especificação do MVP. Leia também o [índice normativo](../../FAULTMAP_MVP.md) e todos os documentos que ele referencia.

## Roadmap

1. **Fundação:** repositório, Go, Cobra, SQLite, migrations, lint, testes e `faultmap init`.
2. **Demo Shop:** checkout, payment, PostgreSQL, load generator, OTel Collector, instrumentação e primeiro cenário de falha.
3. **Ingestão:** importação de arquivo, receiver OTLP HTTP, normalização de spans, persistência e consultas por janela.
4. **Incidentes:** modelo de incidente, baseline/janela do incidente e estatísticas básicas.
5. **Detectores:** `error_rate_delta`, `latency_delta`, `database_timeout`, `deployment_proximity` e `retry_storm`.
6. **Grafo:** nós, arestas, proveniência, subgrafo do incidente e exportação Mermaid.
7. **Ranking:** pesos, agregação de findings, confiança, explicações e top 3.
8. **GitHub:** commits, deploys, relação de versões e proximidade no ranking.
9. **Relatórios:** terminal, JSON, Markdown, Mermaid e timeline.
10. **Validação:** executar cenários, medir top-1/top-3, reduzir falsos positivos, documentar limitações e publicar a primeira release.

## Primeira demonstração obrigatória

1. Subir `checkout-service`, `payment-service`, PostgreSQL, OTel Collector e Faultmap.
2. Gerar no mínimo oito traces saudáveis com uma chamada ao pagamento por trace.
3. Simular a nova versão com `PAYMENT_MAX_ATTEMPTS=4` e o pagamento retornando HTTP 503.
4. Gerar no mínimo oito traces do incidente e aguardar o batch do Collector.
5. Observar aumento de erros e repetição da mesma operação cliente dentro do trace.
6. Executar:

```bash
faultmap diagnose incident --service checkout-service --since 15s --baseline 30s
```

Resultado esperado: suspeito principal `checkout-service`, findings `error_rate_delta` e `retry_storm` com confiança alta, baseline próxima de uma tentativa e incidente próximo de quatro tentativas por trace. A saída deve afirmar que é uma hipótese sustentada por evidências, não prova absoluta de causalidade, e listar explicações alternativas para spans repetidos.

## Diretrizes para implementação

1. Trabalhar um marco por vez e não antecipar funcionalidades futuras.
2. Não criar microsserviços, LLM, Neo4j ou Kubernetes.
3. Não criar abstrações sem um segundo caso concreto; manter interfaces pequenas.
4. Preservar a separação entre domínio e infraestrutura.
5. Escrever testes junto de cada módulo; executar `go test ./...` e, em mudanças concorrentes, `go test -race ./...`.
6. Usar `context.Context` em I/O, encapsular erros com `%w` e não ignorar erros.
7. Não usar variáveis globais para estado mutável nem expor structs de bibliotecas externas ao domínio.
8. Documentar decisões arquiteturais relevantes em `docs/adr/`, manter README atualizado e gerar commits pequenos, claros e de escopo limitado.

## Regras de implementação para código e banco de dados

1. Comentários devem explicar intenção, decisões não óbvias, garantias, limitações e efeitos colaterais. Não escrever comentários que apenas repitam o código.

2. Identificadores exportados em Go devem possuir comentários compatíveis com as convenções da linguagem. Funções internas simples não precisam de comentários, salvo quando o comportamento não for evidente.

3. Toda consulta SQL deve ser deliberada e orientada ao caso de uso:

   - selecionar somente as colunas necessárias;
   - filtrar o mais cedo possível;
   - limitar ou paginar leituras potencialmente grandes;
   - utilizar ordenação determinística quando houver paginação;
   - evitar carregar dados que não serão utilizados.

4. Não limitar artificialmente a quantidade de `JOIN`s. Utilizar a consulta que represente corretamente o caso de uso e validar seu custo com dados reais, plano de execução e métricas.

5. Quando uma consulta ficar complexa ou cara, avaliar:

   - índices adequados;
   - pré-agregações;
   - modelos de leitura específicos;
   - tabelas materializadas;
   - cache;
   - divisão da consulta, somente quando isso não gerar N+1 ou inconsistência.

6. Nunca implementar consultas N+1. Quando dados relacionados forem necessários, utilizar operações em lote, `JOIN`, `IN`, pré-carregamento ou um modelo de leitura apropriado.

7. O processo deve criar e reutilizar um `*sql.DB` por banco configurado durante todo o ciclo de vida da aplicação. `*sql.DB` representa um pool de conexões e não deve ser recriado por repositório, consulta ou requisição.

8. O `*sql.DB` deve ser criado no bootstrap da aplicação, injetado nos repositórios e fechado somente durante o encerramento controlado do processo.

9. Configurar explicitamente, de acordo com o banco e a carga esperada:

   - `SetMaxOpenConns`;
   - `SetMaxIdleConns`;
   - `SetConnMaxLifetime`;
   - `SetConnMaxIdleTime`.

10. Todo acesso a banco, rede, arquivos ou APIs externas deve receber `context.Context` e respeitar cancelamento e timeout.

11. Não usar `context.Background()` dentro de repositórios ou integrações para substituir o contexto recebido pelo caso de uso.

12. Transações devem:

   - ser curtas;
   - possuir responsabilidade clara;
   - ser utilizadas somente quando atomicidade for necessária;
   - evitar chamadas HTTP ou processamento pesado enquanto estiverem abertas;
   - garantir `Rollback` em caminhos de erro;
   - executar `Commit` apenas após todas as operações obrigatórias terem sucesso.

13. Repositórios são os únicos responsáveis pela persistência SQL. Casos de uso, domínio, CLI, handlers e detectores não devem conter SQL.

14. Repositórios recebem a dependência compartilhada do banco. Eles nunca devem abrir novas conexões ou criar pools próprios.

15. O domínio não deve importar `database/sql`, drivers de banco, structs do SQLite ou tipos específicos de bibliotecas de persistência.

16. Repositórios devem retornar tipos do domínio ou DTOs de aplicação claramente definidos. Estruturas de persistência não devem vazar para as camadas superiores.

17. Toda consulta deve utilizar parâmetros. Nunca concatenar entradas do usuário, filtros ou identificadores não validados diretamente em SQL.

18. Valores dinâmicos que não podem ser parametrizados, como nomes de colunas e direção de ordenação, devem ser validados por uma allowlist explícita.

19. Sempre verificar erros de `QueryContext`, `QueryRowContext`, `ExecContext`, `Scan`, `Rows.Err`, `Commit` e `Rollback`, quando relevante.

20. Sempre executar `defer rows.Close()` imediatamente após confirmar que `QueryContext` não retornou erro.

21. Tratar `sql.ErrNoRows` separadamente de falhas de infraestrutura. Ausência de registro deve ser representada por um erro de domínio ou aplicação apropriado.

22. Erros devem ser encapsulados com contexto usando `%w`, sem perder a causa original.

23. Não registrar queries completas ou parâmetros que possam conter tokens, credenciais, documentos pessoais, corpos de requisição ou dados sensíveis.

24. SQL bruto completo não deve ser armazenado por padrão na telemetria. Preferir nomes de operação, resumos normalizados ou hashes seguros.

25. Índices devem ser criados a partir dos padrões reais de filtros, ordenações, relacionamentos, unicidade e volume de leitura/escrita.

26. Não criar índices preventivamente sem caso de uso. Todo índice aumenta custo de escrita, armazenamento e manutenção.

27. Consultas críticas devem ser avaliadas com ferramentas apropriadas, como `EXPLAIN` ou `EXPLAIN ANALYZE`, utilizando volume de dados representativo.

28. Otimizações devem ser justificadas por medição. Não adicionar cache, agregações ou estruturas duplicadas apenas por suposição.

29. Toda tabela deve possuir:

   - chave primária;
   - tipos coerentes;
   - restrições de integridade;
   - `NOT NULL` quando ausência não fizer sentido;
   - `UNIQUE` quando a regra de negócio exigir unicidade;
   - chaves estrangeiras quando representarem relações reais.

30. Regras de integridade importantes devem ser protegidas pelo banco, não apenas pelo código da aplicação.

31. Migrations devem ser versionadas, executadas em ordem determinística, evitar alterações destrutivas sem estratégia de transição, possuir rollback quando tecnicamente seguro e ser testadas em banco limpo e banco já populado.

32. Nunca alterar manualmente um banco compartilhado fora do sistema de migrations.

33. Alterações incompatíveis de schema devem seguir uma estratégia expand-and-contract:

   1. adicionar a nova estrutura;
   2. manter compatibilidade;
   3. migrar os dados;
   4. atualizar consumidores;
   5. remover a estrutura antiga somente depois.

34. Para SQLite:

   - habilitar foreign keys explicitamente;
   - configurar `busy_timeout`;
   - avaliar modo WAL para permitir leituras concorrentes;
   - configurar o pool de acordo com o padrão de leitura e escrita;
   - considerar que existe apenas um escritor por vez;
   - manter transações de escrita curtas.

35. Escritas concorrentes no SQLite devem possuir tratamento explícito para contenção e `database is locked`. Não implementar retries infinitos.

36. Repositórios devem possuir testes de integração contra o banco real utilizado pelo módulo. Mocks não substituem testes de SQL, migrations, constraints e transações.

37. Testes de repositório devem cobrir inserção, leitura, atualização, exclusão quando suportada, ausência de registro, constraints, rollback, paginação, ordenação, concorrência relevante e migrations.

38. Toda paginação deve definir uma ordenação estável. Para volumes altos, preferir paginação por cursor quando `OFFSET` se tornar caro ou inconsistente.

39. Operações em lote devem possuir limites configuráveis para evitar queries grandes demais, excesso de parâmetros, transações prolongadas e consumo excessivo de memória.

40. Nenhum repositório deve retornar coleções ilimitadas por padrão. Métodos de listagem precisam receber limite, paginação ou janela temporal.

41. O código não deve usar `panic` para erros esperados de banco, configuração, validação ou integração. `panic` fica reservado para estados realmente irrecuperáveis durante o bootstrap.

42. Não ignorar erros usando `_` sem justificativa documentada.

43. Toda operação com efeitos colaterais deve deixar claro o que modifica, quais garantias oferece, se é idempotente e como se comporta em retry.

44. Operações que possam ser repetidas por causa de retry devem possuir idempotência quando duplicidade puder causar inconsistência.

45. O código deve priorizar clareza e manutenção. Não criar abstrações genéricas antes de existir pelo menos um segundo caso concreto que justifique sua reutilização.

46. APIs e handlers devem permanecer stateless sempre que possível. Estado durável pertence ao banco de dados; estado efêmero que precise ser compartilhado entre instâncias pertence a um cache apropriado. O processo pode manter apenas recursos operacionais e imutáveis, como configuração carregada e pools de conexões.

## Definição final do MVP

> Um monólito modular em Go, distribuído como binário único, capaz de ingerir telemetria OpenTelemetry, comparar uma janela de incidente com uma baseline, correlacionar sinais com deploys e operações PostgreSQL, construir um grafo de evidências e gerar um ranking determinístico, explicável e auditável dos principais suspeitos de uma falha.

O primeiro objetivo não é monitorar tudo: é reduzir o tempo necessário para um desenvolvedor descobrir onde começar a investigar e por quê.
