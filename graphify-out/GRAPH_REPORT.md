# Graph Report - Faultmap  (2026-09-13)

## Corpus Check
- 235 files · ~300,732 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1801 nodes · 5168 edges · 99 communities (86 shown, 13 thin omitted)
- Extraction: 85% EXTRACTED · 14% INFERRED · 0% AMBIGUOUS · INFERRED: 749 edges (avg confidence: 0.83)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `f7d8e307`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- SchemaSnapshot
- Migrate
- Rank
- ExplainSuspect
- graph.go
- IngestFunc
- historicoComUmIncidente
- attributes.go
- payment/handler_test.go
- NewSignalRepository
- NewHandler
- DetectSchemaChangeProximity
- Finding
- Snapshot
- testing.T
- Signal
- Integração GitHub
- detectors_test.go
- Faultmap MVP — Arquitetura
- time.Time
- openDiagnosisRepository
- diagnose_scope.go
- encoding/json.RawMessage
- Diagnosis
- otlp_json.go
- DiagnoseIncidentInScope
- database/sql.Tx
- detectDeploymentProximity
- run-e2e.sh
- DetectDatabaseError
- time.Duration
- DetectRetryStorm
- openSchemaRepository
- timeline.go
- A coleta de schema guarda identificadores, não expressões
- DetectVersionRegression
- otlp_protobuf.go
- New
- CLI faultmap
- Demo Shop
- run-hard-mode.sh
- json/report.go
- NewTimeWindow
- database/sql.DB
- ParseOTLPJSON
- run
- faultmap_app.py
- ParseOTLPTraces
- IngestTelemetry
- acceptance_test.go
- DetectLogCorrelation
- schema_mcp_integration_test.go
- Environment
- Janelas de investigação (baseline e incidente)
- ApplyRetention
- NewEnvironment
- DetectDatabaseLatencyDelta
- Deployment
- run
- run
- run
- Setup
- InitializeProject
- filterDatabaseSignals
- PersistedDiagnosis
- context.Context
- ParseOTLPLogsJSON
- ListSignals
- exigirSinaisCoerentes
- Receiver OTLP HTTP POST /v1/traces
- Grafo de evidências
- Render
- rodar-ranking.sh
- small-pool/generate-traffic.sh
- rodar-incidente.sh
- Módulo incidents
- Evidence
- Health check GET /health
- Serviço faultmap da demo
- payment-500/generate-traffic.sh
- retry-storm/generate-traffic.sh
- table-lock/generate-traffic.sh
- timeout-after-deploy/generate-traffic.sh
- Fora do escopo do primeiro MVP
- Diretrizes para implementação
- github.com/faultmap/faultmap
- io.Writer
- server.go
- RenderDiagnosis
- Piloto cego
- diagnosis_repository.go
- repositorioComRetencao
- Snapshot imutável de diagnóstico
- Fixtures OpenTelemetry
- NewPolicy
- ADR 0015 — A retenção libera o catálogo e preserva as mudanças
- Ingestão OTLP
- postgres-demo

## God Nodes (most connected - your core abstractions)
1. `Signal` - 131 edges
2. `Finding` - 54 edges
3. `newRootCommand()` - 42 edges
4. `Rank()` - 42 edges
5. `DiagnoseIncidentInScope()` - 41 edges
6. `DetectSchemaChangeProximity()` - 37 edges
7. `Migrate()` - 35 edges
8. `Load()` - 32 edges
9. `Open()` - 32 edges
10. `PersistedDiagnosis` - 30 edges

## Surprising Connections (you probably didn't know these)
- `Cenário migracao-inofensiva` --references--> `DetectSchemaChangeProximity()`  [INFERRED]
  CHANGELOG.md → internal/detection/schema_change_proximity.go
- `Defeito no checkout, não na dependência` --references--> `Rank()`  [INFERRED]
  examples/demo-shop/scenarios/retry-storm/compose.yaml → internal/ranking/ranking.go
- `Mapeamento de regras para classes de peso` --rationale_for--> `weightClassForRule()`  [INFERRED]
  docs/mvp/03-diagnostico-integracoes-e-saidas.md → internal/ranking/ranking.go
- `Pool *sql.DB único por banco` --semantically_similar_to--> `Cenário: pool pequeno`  [INFERRED] [semantically similar]
  docs/mvp/05-entrega-e-diretrizes.md → examples/demo-shop/scenarios/small-pool/README.md
- `A lista de bloqueios do YAML soma aos padrões` --rationale_for--> `privacyPolicyFrom()`  [INFERRED]
  docs/adr/0012-bloqueios-de-privacidade-somam-em-vez-de-substituir.md → cmd/faultmap/root.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Fluxo de ingestão OTLP da demo até o SQLite** — examples_demo_shop_compose_checkout_service, examples_demo_shop_compose_payment_service, examples_demo_shop_otel_collector_pipeline_de_traces, docs_mvp_02_dominio_dados_e_telemetria_receiver_otlp_http, docs_mvp_02_dominio_dados_e_telemetria_persistencia_sqlite [EXTRACTED 1.00]
- **Protocolo do piloto cego** — examples_pilot_readme_piloto_cego, examples_pilot_readme_operador_e_investigador, examples_pilot_collector_collector_do_piloto, examples_pilot_registro_registro_do_investigador, examples_pilot_resultado_resultado_do_piloto_cego [EXTRACTED 1.00]
- **Cegueira contra instrumentação real** — docs_adr_0006_detectores_aceitam_duas_convencoes_http_duas_convencoes_http, docs_adr_0007_telemetria_real_como_base_de_teste_telemetria_real_como_base_de_teste, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_semconv_attributes, readme_compatibilidade_com_instrumentacao_real, _claude_skills_consultar_grafo_skill_consultar_o_grafo_antes_de_mudar [INFERRED 0.85]
- **Cenários que exercitam evidência de banco** — examples_demo_shop_scenarios_database_slow_readme_cenario_banco_lento, examples_demo_shop_scenarios_small_pool_readme_cenario_pool_pequeno, examples_demo_shop_scenarios_table_lock_readme_cenario_lock_na_tabela_de_pagamentos, internal_detection_detectors_detectdatabasetimeout, internal_detection_detectors_detectlatencydelta [INFERRED 0.85]
- **Orçamento de peso por classe no ranking** — docs_adr_0001_ranking_reutiliza_peso_graph_proximity_reuso_do_peso_graph_proximity, docs_adr_0010_teto_por_classe_de_peso_no_ranking_teto_por_classe_de_peso, changelog_teto_relativo_para_evidencia_de_apoio, internal_ranking_ranking_weightclassforrule, internal_ranking_ranking_weightforclass, internal_ranking_ranking_rank [INFERRED 0.85]
- **Privacidade aplicada antes do disco** — docs_adr_0008_politica_de_privacidade_aplicada_na_ingestao_politica_aplicada_na_ingestao, docs_adr_0011_logs_guardam_apenas_metadados_logs_sem_texto_da_mensagem, docs_adr_0012_bloqueios_de_privacidade_somam_em_vez_de_substituir_uniao_de_bloqueios, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_privacy_policy, readme_privacidade [INFERRED 0.85]

## Communities (99 total, 13 thin omitted)

### Community 0 - "SchemaSnapshot"
Cohesion: 0.05
Nodes (83): escritorDeSchemaFake, fonteDeSchemaFake, SchemaSource, SchemaWriter, scopedSchemaReaderFake, SchemaChange, SchemaChangeKind, SchemaObject (+75 more)

### Community 1 - "Migrate"
Cohesion: 0.07
Nodes (80): diagnosisEnd(), githubImportEnd(), newBlameCommand(), newBlameTraceCommand(), newDiagnoseCommand(), newDiagnoseIncidentCommand(), newExplainCommand(), newExplainSuspectCommand() (+72 more)

### Community 2 - "Rank"
Cohesion: 0.07
Nodes (49): Módulo platform, Módulo ranking, ScoreContribution, Suspect, Mapeamento de regras para classes de peso, Pesos configuráveis do ranking, Ranking de suspeitos, Configuração inicial faultmap.yaml (+41 more)

### Community 3 - "ExplainSuspect"
Cohesion: 0.05
Nodes (61): SuspectContribution, Backend determinístico, CLI-first e local-first, Explicabilidade, Faultmap, Infraestrutura existente e evolução controlada, Interface Detector, Módulo detection (+53 more)

### Community 4 - "graph.go"
Cohesion: 0.07
Nodes (57): TraceInvestigation, TraceSignalReader, edgeAccumulator, nodeAccumulator, NodeKind, BlameTrace(), hasRelation(), TestBlameTraceCarregaUmaVezEConstroiGrafo() (+49 more)

### Community 5 - "IngestFunc"
Cohesion: 0.07
Nodes (52): mapOTLPEncoding(), authenticate(), NewHandler(), TestHandlerEntregaCommitEDeploymentCoerentes(), TestHandlerProtegeRotasEValidaConfiguracao(), validateConfig(), writeJSON(), delayFromFile() (+44 more)

### Community 6 - "historicoComUmIncidente"
Cohesion: 0.16
Nodes (25): TestAuditoriaNaoEscreveNaSaidaDoProtocolo(), TestAuditoriaNaoRegistraOsArgumentos(), TestArgumentosComTipoErradoViramErroDeTool(), TestContextoCanceladoEncerraASessao(), TestErroDoRepositorioViraErroDeToolENaoDerrubaOServidor(), TestIncidenteInexistenteExplicaOQueAconteceu(), TestLinhaGiganteNaoDerrubaASessao(), TestLoteJSONRPCRecebeRespostaEmVezDeSilencio() (+17 more)

### Community 7 - "attributes.go"
Cohesion: 0.09
Nodes (40): databaseTargetsInWindow(), sortedKeys(), safeRetryIdentity(), signalsForChange(), databaseDetail(), displayDatabaseSystem(), displaySeverity(), duration() (+32 more)

### Community 8 - "payment/handler_test.go"
Cohesion: 0.17
Nodes (19): chronicFailure(), NewHandler(), executePayment(), mustHandler(), TestHandlerConverteFalhaDoBancoSemVazarDetalhes(), TestHandlerErroCrônicoÉDeterminísticoPorPedido(), TestHandlerForcaStatusSemPersistir(), TestHandlerPersistePagamento() (+11 more)

### Community 9 - "NewSignalRepository"
Cohesion: 0.13
Nodes (29): database/sql.Rows, NewRetentionRepository(), openRetentionDatabase(), signalIDs(), TestRetentionRepositoryPreservaSnapshotsDeIncidentes(), TestRetentionRepositoryRejeitaLimiteInválido(), TestRetentionRepositoryRemoveSomenteSinaisAnterioresAoCorte(), TestRetentionRepositoryRespeitaLimiteDoLote() (+21 more)

### Community 10 - "NewHandler"
Cohesion: 0.09
Nodes (29): Config, Handler, Request, roundTripperFunc, Serviço checkout-service, Serviço load-generator, NewHandler(), TestHandlerEncaminhaPagamentoComSucesso() (+21 more)

### Community 11 - "DetectSchemaChangeProximity"
Cohesion: 0.14
Nodes (34): Consultar o grafo antes de mudar, Grafo de conhecimento em graphify-out, graphify affected — travessia reversa, Limites do grafo, Pontuação pela ponta pessimista do intervalo, Antes de mudar código, consulte o grafo, schemaChangeCandidate, containsLimitation() (+26 more)

### Community 12 - "Finding"
Cohesion: 0.18
Nodes (25): Evidence, Input, Detectores aceitam as duas convenções HTTP e ignoram spans internos, Detector database_timeout, Detector error_rate_delta, Detector latency_delta, databaseTimeoutSummary(), DetectDatabaseTimeout() (+17 more)

### Community 13 - "Snapshot"
Cohesion: 0.08
Nodes (35): ChangeSource, changeSourceFake, ChangeWriter, changeWriterFake, Ingestão do GitHub limitada a uma página, sem N+1, Commit, ImportRequest, Snapshot (+27 more)

### Community 14 - "testing.T"
Cohesion: 0.16
Nodes (32): main(), newRootCommand(), assertTableCount(), preparePersistedIncident(), reserveTCPAddress(), TestBlameTraceCommandExplicaFluxoHTTPPostgreSQL(), TestDiagnoseIncidentCommandDetectaRetryStorm(), TestDiagnoseIncidentCommandExplicaAmostraRepresentativa() (+24 more)

### Community 15 - "Signal"
Cohesion: 0.10
Nodes (37): signalStoreFake, traceReaderFake, propagationSummary, serviceLink, Detector dependency_failure, Detector trace_break, SignalType, DetectDependencyFailure() (+29 more)

### Community 16 - "Integração GitHub"
Cohesion: 0.25
Nodes (8): Módulo integrations, Índice composto (trace_id, timestamp, id), Integração GitHub, Integração PostgreSQL, Uma página por execução na coleta GitHub, Pool *sql.DB único por banco, Proibição de consultas N+1, Regras de implementação para código e banco

### Community 17 - "detectors_test.go"
Cohesion: 0.15
Nodes (30): Detector database_http_trace_correlation, DetectTraceCorrelation(), Run(), signalsByTrace(), assertFinding(), contains(), databaseTimeoutSignals(), httpSignals() (+22 more)

### Community 18 - "Faultmap MVP — Arquitetura"
Cohesion: 0.16
Nodes (29): Agents, Applications (Go / Python / Node), CLI Output, Detection Engine, Diagnostic Score, Faultmap MVP — Arquitetura, Entradas, Evidence Graph (+21 more)

### Community 19 - "time.Time"
Cohesion: 0.11
Nodes (14): diagnosisReaderFake, leitorDeSchemaQueFalha, retentionRemoverStub, scopedSignalReaderFake, scopeReaderFake, scopeReaderNiveis, signalReaderFake, time.Time (+6 more)

### Community 20 - "openDiagnosisRepository"
Cohesion: 0.15
Nodes (27): DiagnosisRepository, assertIntPointerEqual(), assertTimePointerEqual(), testDiagnosisAt(), TestDiagnosisRepositoryGetRespectsCanceledContext(), TestDiagnosisRepositoryGetRestoresCompleteSnapshot(), TestDiagnosisRepositoryGetReturnsTypedNotFound(), TestDiagnosisRepositoryGetSupportsLegacySnapshot() (+19 more)

### Community 21 - "diagnose_scope.go"
Cohesion: 0.18
Nodes (22): ScopedDeploymentReader, ScopedDiagnosisRequest, ScopeDiscovery, ScopedSchemaChangeReader, ScopedSignalReader, ScopeReader, A investigação compara serviços descobertos pelos traces, applyDependencyTieBreak() (+14 more)

### Community 22 - "encoding/json.RawMessage"
Cohesion: 0.26
Nodes (16): encoding/json.RawMessage, GetIncident(), IncidentHistoryReader, decodeArguments(), describeFindings(), marshalIndented(), toolDefinitions(), toolError() (+8 more)

### Community 23 - "Diagnosis"
Cohesion: 0.25
Nodes (11): DiagnosisStore, diagnosisStoreFake, DiagnosisID(), Diagnosis, PersistDiagnosis(), TestDiagnosisIDEDeterministicoEmUTC(), TestPersistDiagnosisNaoSalvaIncidenteSemSinais(), TestPersistDiagnosisPreservaCausaDoStore() (+3 more)

### Community 24 - "otlp_json.go"
Cohesion: 0.23
Nodes (19): exceptionAttributes(), normalizeAttributes(), normalizeExportRequest(), normalizeSpan(), parseUnixNano(), resourceSignalAttributes(), scalarJSONValue(), serviceName() (+11 more)

### Community 25 - "DiagnoseIncidentInScope"
Cohesion: 0.32
Nodes (20): DiagnoseIncidentInScope(), escopoSpanDeBanco(), sinalComVersao(), TestDiagnoseScopeAcusaOCommitImplantado(), TestDiagnoseScopeAcusaSchemaQuandoHaSintoma(), TestDiagnoseScopeAlcançaSegundoSalto(), TestDiagnoseScopeConsultaDeploymentsEmLote(), TestDiagnoseScopeCorrelacionaMudancaDeSchema() (+12 more)

### Community 26 - "database/sql.Tx"
Cohesion: 0.26
Nodes (10): database/sql.NullInt64, database/sql.NullTime, database/sql.Tx, DiagnosisRepository, nullIntPointer(), nullTimePointer(), readPersistedFindings(), readPersistedIncident() (+2 more)

### Community 27 - "detectDeploymentProximity"
Cohesion: 0.15
Nodes (21): Mudança de schema como sinal de incidente, deploymentCandidate, Configuração E2E com GitHub, Mock GitHub no loopback do container, Matriz E2E automatizada, Override timeout-after-deploy (SHA como SERVICE_VERSION), Cenário: timeout depois de uma mudança, commitLabel() (+13 more)

### Community 28 - "run-e2e.sh"
Cohesion: 0.16
Nodes (12): activate_scenario(), assert_contains(), assert_json(), cleanup(), compose(), diagnose(), first_trace_id(), run_scenario() (+4 more)

### Community 29 - "DetectDatabaseError"
Cohesion: 0.19
Nodes (20): Telemetria de instrumentação real como base de teste, Detector database_error, attributeValueOrEmpty(), databaseFailureTypeSuffix(), databaseNonTimeoutFailures(), DetectDatabaseError(), isClientCancellation(), bancoComFalhas() (+12 more)

### Community 30 - "time.Duration"
Cohesion: 0.18
Nodes (15): TestPostgresRepositoryAtrasoOcupaConexaoDoPool(), TestPostgresRepositoryIntegracao(), NewPostgresRepository(), recordDatabaseSpanError(), mustPostgresRepository(), TestNewPostgresRepositoryRejeitaPoolAusente(), TestPostgresRepositoryAtrasoRespeitaContexto(), TestPostgresRepositoryCreateUsaConsultaParametrizada() (+7 more)

### Community 31 - "DetectRetryStorm"
Cohesion: 0.14
Nodes (23): retryCandidate, retryOperationStats, Detector retry_storm, Sistema de demonstração demo-shop, Primeira demonstração obrigatória, Roadmap de dez marcos, Cenário: tempestade de retries, Lacuna: detector de atraso de consumidor (+15 more)

### Community 32 - "openSchemaRepository"
Cohesion: 0.27
Nodes (16): TestListSchemaChangesLimitaAConsulta(), TestSaveSnapshotAceitaBaseVaziaDesdeOInicio(), TestSaveSnapshotFalhaComBancoFechado(), TestSaveSnapshotFalhaComColetaAnteriorCorrompida(), TestSaveSnapshotRecusaColetaGigante(), TestSaveSnapshotRecusaColetaVaziaContraCatalogoPovoado(), TestSaveSnapshotRejeitaColetaSemIdentidade(), TestSaveSnapshotRespeitaContextoCancelado() (+8 more)

### Community 33 - "timeline.go"
Cohesion: 0.10
Nodes (29): Workflow de CI, Matriz E2E fora do gate automático, Binários reproduzíveis com checksums, Workflow de Release, Cenário migracao-inofensiva, Matriz E2E, Modo difícil, make verify (+21 more)

### Community 34 - "A coleta de schema guarda identificadores, não expressões"
Cohesion: 0.21
Nodes (17): Changelog do Faultmap, Piloto cego, Servidor MCP somente leitura, Teto relativo para evidência de apoio, PrivacyConfig, Detectores estruturais reutilizam o peso graph_proximity, error_rate_delta ignora variação de amostragem, Política de privacidade aplicada na ingestão (+9 more)

### Community 35 - "DetectVersionRegression"
Cohesion: 0.28
Nodes (14): versionStats, Detector version_regression, comparableVersionStats(), DetectVersionRegression(), TestVersionRegressionComparaDuasVersõesNaMesmaJanela(), TestVersionRegressionDetectaLatênciaPiorEmUmaVersão(), TestVersionRegressionIgnoraDiferençaDentroDoRuído(), TestVersionRegressionSilenciaComUmaÚnicaVersão() (+6 more)

### Community 36 - "otlp_protobuf.go"
Cohesion: 0.22
Nodes (15): go.opentelemetry.io/proto/otlp/collector/trace/v1.ExportTraceServiceRequest, go.opentelemetry.io/proto/otlp/common/v1.ArrayValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValueList, go.opentelemetry.io/proto/otlp/trace/v1.Span, go.opentelemetry.io/proto/otlp/trace/v1.Span_Event, marshalProtoValue(), protobufAnyValue() (+7 more)

### Community 37 - "New"
Cohesion: 0.24
Nodes (13): Identificadores de catálogo ordenáveis por tempo, encode(), extract(), New(), TestAlfabetoNaoTemCaracteresAmbiguos(), TestClassificabilidade(), TestClassificabilidadeDistingueMilissegundos(), TestDeterminismoPreservaIdempotencia() (+5 more)

### Community 38 - "CLI faultmap"
Cohesion: 0.25
Nodes (8): Módulo reporting, Artefatos gerados em faultmap-out, Comando faultmap blame trace, CLI faultmap, Comando faultmap export graph, Comando faultmap export report, Comando faultmap init, Comando faultmap incident list

### Community 39 - "Demo Shop"
Cohesion: 0.23
Nodes (12): Serviço payment-service, Serviço postgres da demo, Demo Shop, Override database-slow (DB_DELAY 750ms), generate-traffic.sh script, Cenário: banco lento, Override payment-500 (FORCE_HTTP_STATUS 500), Cenário: pagamento retorna HTTP 500 (+4 more)

### Community 40 - "run-hard-mode.sh"
Cohesion: 0.28
Nodes (14): apply_harmless_migration(), assert_contains(), assert_no_finding(), cleanup(), collect_schema(), compose(), diagnose(), generate_burst_traffic() (+6 more)

### Community 41 - "json/report.go"
Cohesion: 0.13
Nodes (30): RenderIncidentSummary(), summarySnapshot(), TestRenderIncidentSummaryNaoDuplicaRelatorioCompleto(), TestRenderIncidentSummaryProduzBytesIdenticosEmDuasExecucoes(), TestRenderIncidentSummaryResumeIdentificacaoJanelasEContagens(), TestRenderIncidentSummarySemMetadadosCompletosOmiteBaseline(), TestRenderIncidentSummarySemSuspeitosOmiteSuspeitoPrincipal(), RenderRanking() (+22 more)

### Community 42 - "NewTimeWindow"
Cohesion: 0.26
Nodes (10): InvestigationWindow, TimeWindow, NewInvestigationWindow(), NewTimeWindow(), TestNewInvestigationWindowAllowsBaselineToMeetIncidentBoundary(), TestNewInvestigationWindowFromIncidentCalculatesContiguousBaseline(), TestNewInvestigationWindowFromIncidentRejectsInvalidInputs(), TestNewInvestigationWindowRejectsOverlappingOrInvalidWindows() (+2 more)

### Community 43 - "database/sql.DB"
Cohesion: 0.40
Nodes (13): database/sql.DB, assertIndexColumns(), assertIndexExists(), assertMigrationVersion(), assertTableExists(), createVersionOneSchema(), migrationRowCount(), openMigrationTestDatabase() (+5 more)

### Community 44 - "ParseOTLPJSON"
Cohesion: 0.22
Nodes (14): encoding/json.Decoder, go.opentelemetry.io/proto/otlp/common/v1.AnyValue, ParseOTLPJSON(), rejectTrailingJSON(), assertSignalEqual(), intAnyValue(), stringAnyValue(), TestParseOTLPJSONHonorsCancelledContext() (+6 more)

### Community 45 - "run"
Cohesion: 0.23
Nodes (10): NewHTTPServer(), RunHTTPServer(), SignalContext(), TestNewHTTPServerConfiguraLimitesEHealth(), TestRunHTTPServerEncerraComContexto(), loadConfig(), main(), run() (+2 more)

### Community 46 - "faultmap_app.py"
Cohesion: 0.19
Nodes (10): _current_fault(), _fault_file(), _FaultInjector, _install_database_delay(), _parse_fault(), Ponto de entrada que instrumenta o DuckDB antes de carregar a aplicação. Fica…, Atrasa cada consulta quando a falha ativa for db_slow. O patch é no execute da…, Arquivo consultado a cada requisição para saber se a falha está ligada. Ler de… (+2 more)

### Community 47 - "ParseOTLPTraces"
Cohesion: 0.24
Nodes (11): io.Reader, classifyOTLPError(), OTLPEncoding, contextError(), TestParseOTLPTracesJSONAceitaEnumsNumericosOficiais(), TestParseOTLPTracesRejeitaCodificacaoDesconhecida(), ParseOTLPLogs(), ParseOTLPTraces() (+3 more)

### Community 48 - "IngestTelemetry"
Cohesion: 0.38
Nodes (10): IngestionResult, SignalStore, contextError(), IngestLogs(), IngestTelemetry(), IngestTelemetryFile(), TestIngestTelemetryFileNormalizaEPersiste(), TestIngestTelemetryPreservaClassificacaoDePayloadInvalido() (+2 more)

### Community 49 - "acceptance_test.go"
Cohesion: 0.42
Nodes (11): readRequiredFile(), requireContains(), TestDemoPublicaPortasSomenteNoLoopback(), TestDemoUsaPortaDeHostDedicada(), TestLoadGeneratorRecebeIdentidadeOTel(), TestReadmesDocumentamExecucaoDaDemo(), TestRunnerE2EDeclaraMatrizLimitesELimpeza(), TestScenariosDocumentamContratoReproduzivel() (+3 more)

### Community 50 - "DetectLogCorrelation"
Cohesion: 0.36
Nodes (10): DetectLogCorrelation(), errorLogs(), filterLogSignals(), logsDeErro(), requisicoes(), TestLogCorrelationExigeCorrelaçãoComTrace(), TestLogCorrelationIgnoraRuídoConstante(), TestLogCorrelationLigaErrosDeLogAoTraceQueFalhou() (+2 more)

### Community 51 - "schema_mcp_integration_test.go"
Cohesion: 0.40
Nodes (10): executar(), gravarColetasDeCatalogo(), gravarTelemetriaDeBanco(), sessaoMCP(), TestIngestSchemaCommandNaoGravaCredencialNoWorkspace(), TestIngestSchemaCommandValidaEntradaAntesDeAbrirConexao(), TestMCPCommandFalaOProtocoloDePontaAPonta(), TestMCPCommandNaoPoluiStdout() (+2 more)

### Community 52 - "Environment"
Cohesion: 0.29
Nodes (5): loadConfig(), main(), run(), Environment, config

### Community 53 - "Janelas de investigação (baseline e incidente)"
Cohesion: 0.40
Nodes (5): InvestigationWindow, Janelas de investigação (baseline e incidente), Metadados legados anuláveis, Estratégia expand-and-contract, Baseline mínima de 30 sinais por serviço

### Community 54 - "ApplyRetention"
Cohesion: 0.21
Nodes (14): podadorDeCatalogoFake, RetentionRequest, RetentionResult, SchemaCatalogPruner, SignalRetentionRemover, ApplyRetention(), pruneSchemaCatalogs(), TestApplyRetentionCalculaCorteEEncerraQuandoNãoHáMaisSinais() (+6 more)

### Community 55 - "NewEnvironment"
Cohesion: 0.33
Nodes (7): mapLookup(), TestLoadConfigConvertePoolPostgres(), TestLoadConfigRejeitaIdleMaiorQueOpen(), Lookup, NewEnvironment(), TestEnvironmentRejeitaConfiguracaoInsegura(), TestEnvironmentValidaValoresObrigatorios()

### Community 56 - "DetectDatabaseLatencyDelta"
Cohesion: 0.38
Nodes (9): DetectDatabaseLatencyDelta(), exceedsDatabaseLatencyNoise(), succeededDatabaseSignals(), bancoComLatência(), TestDatabaseLatencyDeltaAcusaBancoQueDegradouSemFalhar(), TestDatabaseLatencyDeltaExigeDuasJanelas(), TestDatabaseLatencyDeltaIgnoraOperaçõesQueFalharam(), TestDatabaseLatencyDeltaIgnoraOscilaçãoNormal() (+1 more)

### Community 57 - "Deployment"
Cohesion: 0.18
Nodes (11): scopedDeploymentReaderFake, Deployment, deploymentsForService(), NewChangeRepository(), changeSnapshot(), deploymentAt(), openChangeRepository(), TestChangeRepositoryListDeploymentsFiltersOrdersAndLimits() (+3 more)

### Community 58 - "run"
Cohesion: 0.39
Nodes (7): config, loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCheckout(), TestLoadConfigRejeitaRetryIlimitado()

### Community 59 - "run"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteGitHubMock(), TestLoadConfigExigeSHA(), config

### Community 60 - "run"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCargaLimitada(), TestLoadConfigRejeitaConcorrenciaExcessiva(), config

### Community 61 - "Setup"
Cohesion: 0.36
Nodes (6): Setup(), TestConfigValidateExigeIdentidadeCompleta(), TestDisabledMantemShutdownSeguro(), TestTraceEndpointAcrescentaCaminhoOTLP(), traceEndpoint(), Config

### Community 62 - "InitializeProject"
Cohesion: 0.36
Nodes (7): ensureContext(), InitializeProject(), assertDirectoryExists(), assertFileExists(), TestInitializeProjectCreatesLocalWorkspace(), TestInitializeProjectDoesNotOverwriteExistingConfiguration(), TestInitializeProjectHonorsCancelledContext()

### Community 63 - "filterDatabaseSignals"
Cohesion: 0.27
Nodes (12): databaseSystem(), databaseSystems(), filterDatabaseSignals(), carregarFixtureReal(), separarPorFalha(), serviçosDaFixture(), TestDetectorDeBancoDisparaComTelemetriaReal(), TestDetectoresNovosNãoAcusamTelemetriaRealSaudável() (+4 more)

### Community 64 - "PersistedDiagnosis"
Cohesion: 0.21
Nodes (9): incidentHistoryReaderFake, IncidentSummary, PersistedDiagnosis, ListIncidents(), TestGetIncidentValidaIDEPropagaAusencia(), TestListIncidentsPreservaOrdemDoRepositorio(), TestListIncidentsValidaLimiteAntesDoRepositorio(), TestPersistedDiagnosisMetadataComplete() (+1 more)

### Community 65 - "context.Context"
Cohesion: 0.20
Nodes (6): Retenção apaga telemetria e preserva snapshots, context.Context, RetentionRepository, rollbackRetentionTransaction(), RetentionRepository, ScopeRepository

### Community 66 - "ParseOTLPLogsJSON"
Cohesion: 0.27
Nodes (12): logRecordID(), logSeverity(), normalizeLogRecord(), ParseOTLPLogsJSON(), TestParseOTLPLogsNuncaArmazenaOTextoDaMensagem(), TestParseOTLPLogsPreservaSeveridadeECorrelação(), TestParseOTLPLogsRejeitaEnvelopeInválido(), TestParseOTLPLogsÉIdempotentePorRegistro() (+4 more)

### Community 67 - "ListSignals"
Cohesion: 0.43
Nodes (5): SignalReader, ListSignals(), TestListSignalsEncaminhaConsulta(), TestListSignalsRespeitaContextoCancelado(), TestListSignalsValidaEntradaAntesDeConsultar()

### Community 68 - "exigirSinaisCoerentes"
Cohesion: 0.67
Nodes (6): testing.F, exigirErroClassificado(), exigirSinaisCoerentes(), FuzzParseOTLPLogsJSONNãoDevolveOCorpo(), FuzzParseOTLPTracesJSON(), FuzzParseOTLPTracesProtobuf()

### Community 69 - "Receiver OTLP HTTP POST /v1/traces"
Cohesion: 0.33
Nodes (6): Receiver OTLP HTTP POST /v1/traces, Comando faultmap serve, Receiver OTLP sem autenticação nem TLS, Segurança e privacidade, Pipeline de traces do Collector da demo, Defeito: blocked_attributes substituía os padrões

### Community 70 - "Grafo de evidências"
Cohesion: 0.40
Nodes (5): Módulo evidence, EvidenceEdge, EvidenceNode, Fallback de parentesco de spans, Grafo de evidências

### Community 71 - "Render"
Cohesion: 0.70
Nodes (4): Render(), reportDiagnosis(), TestRenderProduzContratoVersionadoEDeterministico(), TestRenderRepresentaBaselineLegadaComoNull()

### Community 72 - "rodar-ranking.sh"
Cohesion: 0.83
Nodes (3): limpar(), rodar-ranking.sh script, trafego()

### Community 86 - "io.Writer"
Cohesion: 0.23
Nodes (9): io.Writer, newAuditor(), RenderScopeSummary(), formatIncidentTime(), RenderIncidentList(), RenderPersistedDiagnosis(), TestRenderIncidentListOrdenaEExibeResumo(), TestRenderPersistedDiagnosisExplicaMetadataLegada() (+1 more)

### Community 87 - "server.go"
Cohesion: 0.30
Nodes (10): bufio.Reader, encoding/json.Encoder, dispatch(), handle(), protocolError(), readLine(), Options, request (+2 more)

### Community 88 - "RenderDiagnosis"
Cohesion: 0.36
Nodes (9): RenderDiagnosis(), renderDiagnosisForTest(), TestRenderDiagnosisApresentaRegrasDeFormaHumanaEAuditavel(), TestRenderDiagnosisApresentaRetryStormEmPortugues(), TestRenderDiagnosisConsolidaLimitacoesRepetidas(), TestRenderDiagnosisDistingueCommitDeServiço(), TestRenderDiagnosisDizOQueAquelePadrãoCostumaSignificar(), TestRenderDiagnosisMantemLimitacaoEspecificaNaHipotese() (+1 more)

### Community 89 - "Piloto cego"
Cohesion: 0.28
Nodes (9): Critérios de aceite do MVP, Métricas de sucesso, Collector do piloto, Exportadores separados para traces e logs, Papéis de operador e investigador, Piloto cego, Sequência recomendada de incidentes, Registro do investigador (+1 more)

### Community 90 - "diagnosis_repository.go"
Cohesion: 0.39
Nodes (7): findingID(), DiagnosisRepository, prepareDiagnosis(), rollbackDiagnosisTransaction(), subjectIdentifier(), preparedDiagnosis, preparedFinding

### Community 91 - "repositorioComRetencao"
Cohesion: 0.50
Nodes (8): coletaDe(), repositorioComRetencao(), TestColetaContraLinhaDeBaseEsvaziadaFalhaEmVezDeInventarMigracao(), TestPruneAvancaEmLotes(), TestPruneEhIdempotente(), TestPruneLiberaOCatalogoAntigoSemApagarAsMudancas(), TestPrunePreservaAColetaMaisRecenteDeCadaBase(), RetentionRepository

### Community 92 - "Snapshot imutável de diagnóstico"
Cohesion: 0.29
Nodes (7): Módulo storage, Persistência SQLite, Snapshot imutável de diagnóstico, Comando faultmap diagnose incident, Comando faultmap incident show, SQLite com um escritor por vez, Flag --until em RFC 3339 estrito

### Community 93 - "Fixtures OpenTelemetry"
Cohesion: 0.33
Nodes (6): Atributos OpenTelemetry prioritários, Testes obrigatórios, StriderEdge (aplicação FastAPI + DuckDB de terceiros), Convenção antiga de atributos (db.system, http.status_code), Telemetria real de instrumentação de terceiros, Fixtures OpenTelemetry

### Community 94 - "NewPolicy"
Cohesion: 0.53
Nodes (5): NewPolicy(), TestPolicyIgnoraDiferençaDeCaixaEEspaços(), TestPolicyRemoveAtributosBloqueados(), TestPolicySemLimiteNãoTrunca(), TestPolicyTruncaAtributosLongos()

### Community 95 - "ADR 0015 — A retenção libera o catálogo e preserva as mudanças"
Cohesion: 0.40
Nodes (4): ADR 0015 — A retenção libera o catálogo e preserva as mudanças, Consequências, Contexto, Decisão

### Community 96 - "Ingestão OTLP"
Cohesion: 0.40
Nodes (5): Módulo telemetry, Allowlist de atributos de Resource, Ingestão OTLP, Signal, Detector deployment_proximity

### Community 97 - "postgres-demo"
Cohesion: 0.40
Nodes (4): DATABASE_URI, uvx, postgres-demo, postgres-mcp

## Ambiguous Edges - Review These
- `database-slow/generate-traffic.sh` → `Cenário: banco lento`  [AMBIGUOUS]
  examples/demo-shop/scenarios/database-slow/README.md · relation: calls
- `server.go` → `MCP`  [AMBIGUOUS]
  docs/images/faultmap-mvp-architecture.png · relation: references
- `Detection Engine` → `PostgreSQL (DB signals / stats)`  [AMBIGUOUS]
  docs/images/faultmap-mvp-architecture.png · relation: shares_data_with
- `Evidence Graph` → `PostgreSQL (DB signals / stats)`  [AMBIGUOUS]
  docs/images/faultmap-mvp-architecture.png · relation: shares_data_with

## Knowledge Gaps
- **55 isolated node(s):** `uvx`, `postgres-mcp`, `DATABASE_URI`, `Request`, `results` (+50 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **13 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `database-slow/generate-traffic.sh` and `Cenário: banco lento`?**
  _Edge tagged AMBIGUOUS (relation: calls) - confidence is low._
- **What is the exact relationship between `server.go` and `MCP`?**
  _Edge tagged AMBIGUOUS (relation: references) - confidence is low._
- **What is the exact relationship between `Detection Engine` and `PostgreSQL (DB signals / stats)`?**
  _Edge tagged AMBIGUOUS (relation: shares_data_with) - confidence is low._
- **What is the exact relationship between `Evidence Graph` and `PostgreSQL (DB signals / stats)`?**
  _Edge tagged AMBIGUOUS (relation: shares_data_with) - confidence is low._
- **Why does `Signal` connect `Signal` to `Rank`, `graph.go`, `attributes.go`, `NewSignalRepository`, `DetectSchemaChangeProximity`, `Finding`, `detectors_test.go`, `time.Time`, `diagnose_scope.go`, `otlp_json.go`, `DiagnoseIncidentInScope`, `detectDeploymentProximity`, `DetectDatabaseError`, `DetectRetryStorm`, `A coleta de schema guarda identificadores, não expressões`, `DetectVersionRegression`, `ParseOTLPJSON`, `ParseOTLPTraces`, `DetectLogCorrelation`, `DetectDatabaseLatencyDelta`, `filterDatabaseSignals`, `ParseOTLPLogsJSON`, `ListSignals`, `exigirSinaisCoerentes`?**
  _High betweenness centrality (0.085) - this node is a cross-community bridge._
- **Why does `Finding` connect `Finding` to `Rank`, `ExplainSuspect`, `DetectSchemaChangeProximity`, `Signal`, `detectors_test.go`, `time.Time`, `openDiagnosisRepository`, `diagnose_scope.go`, `encoding/json.RawMessage`, `Diagnosis`, `database/sql.Tx`, `detectDeploymentProximity`, `DetectDatabaseError`, `DetectRetryStorm`, `timeline.go`, `DetectVersionRegression`, `json/report.go`, `DetectLogCorrelation`, `DetectDatabaseLatencyDelta`, `PersistedDiagnosis`, `RenderDiagnosis`, `diagnosis_repository.go`?**
  _High betweenness centrality (0.057) - this node is a cross-community bridge._
- **Why does `RenderRanking()` connect `json/report.go` to `PersistedDiagnosis`, `graph.go`, `CLI faultmap`, `time.Time`, `io.Writer`?**
  _High betweenness centrality (0.028) - this node is a cross-community bridge._