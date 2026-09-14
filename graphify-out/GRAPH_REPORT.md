# Graph Report - Faultmap  (2026-09-13)

## Corpus Check
- 235 files · ~302,032 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1808 nodes · 5189 edges · 91 communities (78 shown, 13 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 753 edges (avg confidence: 0.83)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d8d93c10`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- SchemaSnapshot
- Migrate
- Rank
- Suspect
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
- DetectTraceBreak
- Integração GitHub
- detectors_test.go
- Faultmap MVP — Arquitetura
- time.Time
- openDiagnosisRepository
- DiagnoseIncidentInScope
- encoding/json.RawMessage
- Diagnosis
- otlp_json.go
- NewInvestigationWindowFromIncident
- context.Context
- detectDeploymentProximity
- run-e2e.sh
- DetectDatabaseError
- time.Duration
- DetectRetryStorm
- openSchemaRepository
- Signal
- Faultmap
- DetectVersionRegression
- otlp_protobuf.go
- New
- CLI faultmap
- run
- run-hard-mode.sh
- PersistedDiagnosis
- NewTimeWindow
- database/sql.DB
- ParseOTLPTraces
- run
- faultmap_app.py
- CommonCauses
- IngestTelemetry
- acceptance_test.go
- DetectLogCorrelation
- schema_mcp_integration_test.go
- Environment
- clienteComMock
- ApplyRetention
- NewEnvironment
- DetectDatabaseLatencyDelta
- Deployment
- run
- testhelpers_test.go
- run
- Setup
- InitializeProject
- filterDatabaseSignals
- commitSHAFromEvidence
- ParseOTLPLogsJSON
- ListSignals
- exigirSinaisCoerentes
- Ingestão OTLP
- Grafo de evidências
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
- RenderDiagnosis
- Piloto cego
- repositorioComRetencao
- Fixtures OpenTelemetry
- ADR 0015 — A retenção libera o catálogo e preserva as mudanças
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
- `Defeito no checkout, não na dependência` --references--> `Rank()`  [INFERRED]
  examples/demo-shop/scenarios/retry-storm/compose.yaml → internal/ranking/ranking.go
- `Pool *sql.DB único por banco` --semantically_similar_to--> `Cenário: pool pequeno`  [INFERRED] [semantically similar]
  docs/mvp/05-entrega-e-diretrizes.md → examples/demo-shop/scenarios/small-pool/README.md
- `A lista de bloqueios do YAML soma aos padrões` --rationale_for--> `privacyPolicyFrom()`  [INFERRED]
  docs/adr/0012-bloqueios-de-privacidade-somam-em-vez-de-substituir.md → cmd/faultmap/root.go
- `faultmap diagnose incident` --references--> `newDiagnoseIncidentCommand()`  [INFERRED]
  README.md → cmd/faultmap/root.go
- `Retenção apaga telemetria e preserva snapshots` --rationale_for--> `ApplyRetention()`  [INFERRED]
  docs/adr/0003-retencao-preserva-snapshots-de-diagnostico.md → internal/application/apply_retention.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Fluxo de ingestão OTLP da demo até o SQLite** — examples_demo_shop_compose_checkout_service, examples_demo_shop_compose_payment_service, examples_demo_shop_otel_collector_pipeline_de_traces, docs_mvp_02_dominio_dados_e_telemetria_receiver_otlp_http, docs_mvp_02_dominio_dados_e_telemetria_persistencia_sqlite [EXTRACTED 1.00]
- **Protocolo do piloto cego** — examples_pilot_readme_piloto_cego, examples_pilot_readme_operador_e_investigador, examples_pilot_collector_collector_do_piloto, examples_pilot_registro_registro_do_investigador, examples_pilot_resultado_resultado_do_piloto_cego [EXTRACTED 1.00]
- **Cegueira contra instrumentação real** — docs_adr_0006_detectores_aceitam_duas_convencoes_http_duas_convencoes_http, docs_adr_0007_telemetria_real_como_base_de_teste_telemetria_real_como_base_de_teste, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_semconv_attributes, readme_compatibilidade_com_instrumentacao_real, _claude_skills_consultar_grafo_skill_consultar_o_grafo_antes_de_mudar [INFERRED 0.85]
- **Cenários que exercitam evidência de banco** — examples_demo_shop_scenarios_database_slow_readme_cenario_banco_lento, examples_demo_shop_scenarios_small_pool_readme_cenario_pool_pequeno, examples_demo_shop_scenarios_table_lock_readme_cenario_lock_na_tabela_de_pagamentos, internal_detection_detectors_detectdatabasetimeout, internal_detection_detectors_detectlatencydelta [INFERRED 0.85]
- **Orçamento de peso por classe no ranking** — docs_adr_0001_ranking_reutiliza_peso_graph_proximity_reuso_do_peso_graph_proximity, docs_adr_0010_teto_por_classe_de_peso_no_ranking_teto_por_classe_de_peso, changelog_teto_relativo_para_evidencia_de_apoio, internal_ranking_ranking_weightclassforrule, internal_ranking_ranking_weightforclass, internal_ranking_ranking_rank [INFERRED 0.85]
- **Privacidade aplicada antes do disco** — docs_adr_0008_politica_de_privacidade_aplicada_na_ingestao_politica_aplicada_na_ingestao, docs_adr_0011_logs_guardam_apenas_metadados_logs_sem_texto_da_mensagem, docs_adr_0012_bloqueios_de_privacidade_somam_em_vez_de_substituir_uniao_de_bloqueios, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_privacy_policy, readme_privacidade [INFERRED 0.85]

## Communities (91 total, 13 thin omitted)

### Community 0 - "SchemaSnapshot"
Cohesion: 0.06
Nodes (77): escritorDeSchemaFake, fonteDeSchemaFake, SchemaSource, SchemaWriter, Mudança de schema como sinal de incidente, A coleta de schema guarda identificadores, não expressões, SchemaChangeKind, SchemaObject (+69 more)

### Community 1 - "Migrate"
Cohesion: 0.07
Nodes (81): diagnosisEnd(), githubImportEnd(), newBlameCommand(), newBlameTraceCommand(), newDiagnoseCommand(), newDiagnoseIncidentCommand(), newExplainCommand(), newExplainSuspectCommand() (+73 more)

### Community 2 - "Rank"
Cohesion: 0.05
Nodes (66): Consultar o grafo antes de mudar, Grafo de conhecimento em graphify-out, graphify affected — travessia reversa, Limites do grafo, Changelog do Faultmap, Piloto cego, Servidor MCP somente leitura, Teto relativo para evidência de apoio (+58 more)

### Community 3 - "Suspect"
Cohesion: 0.08
Nodes (43): SuspectContribution, strings.Builder, ExplainSuspect(), findingsForService(), findSuspect(), SuspectExplanation, groupContributions(), normalizeFinding() (+35 more)

### Community 4 - "graph.go"
Cohesion: 0.09
Nodes (45): TraceInvestigation, TraceSignalReader, edgeAccumulator, nodeAccumulator, NodeKind, BlameTrace(), hasRelation(), TestBlameTraceCarregaUmaVezEConstroiGrafo() (+37 more)

### Community 5 - "IngestFunc"
Cohesion: 0.07
Nodes (51): authenticate(), NewHandler(), TestHandlerEntregaCommitEDeploymentCoerentes(), TestHandlerProtegeRotasEValidaConfiguracao(), validateConfig(), writeJSON(), delayFromFile(), NewHandler() (+43 more)

### Community 6 - "historicoComUmIncidente"
Cohesion: 0.08
Nodes (45): rejectionReporter(), bufio.Reader, encoding/json.Encoder, io.Writer, newAuditor(), TestAuditoriaNaoEscreveNaSaidaDoProtocolo(), TestAuditoriaNaoRegistraOsArgumentos(), TestArgumentosComTipoErradoViramErroDeTool() (+37 more)

### Community 7 - "attributes.go"
Cohesion: 0.08
Nodes (41): databaseTargetsInWindow(), sortedKeys(), safeRetryIdentity(), databaseTargetsQueriedBy(), isHTTPSignal(), databaseDetail(), displayDatabaseSystem(), displaySeverity() (+33 more)

### Community 8 - "payment/handler_test.go"
Cohesion: 0.17
Nodes (19): chronicFailure(), NewHandler(), executePayment(), mustHandler(), TestHandlerConverteFalhaDoBancoSemVazarDetalhes(), TestHandlerErroCrônicoÉDeterminísticoPorPedido(), TestHandlerForcaStatusSemPersistir(), TestHandlerPersistePagamento() (+11 more)

### Community 9 - "NewSignalRepository"
Cohesion: 0.09
Nodes (37): Retenção apaga telemetria e preserva snapshots, Especificação do MVP (índice normativo), Monólito modular em Go, database/sql.Rows, RetentionRepository, NewRetentionRepository(), rollbackRetentionTransaction(), openRetentionDatabase() (+29 more)

### Community 10 - "NewHandler"
Cohesion: 0.09
Nodes (29): Config, Handler, Request, roundTripperFunc, Serviço checkout-service, Serviço load-generator, NewHandler(), TestHandlerEncaminhaPagamentoComSucesso() (+21 more)

### Community 11 - "DetectSchemaChangeProximity"
Cohesion: 0.10
Nodes (41): Workflow de CI, Matriz E2E fora do gate automático, Binários reproduzíveis com checksums, Workflow de Release, scopedSchemaReaderFake, Cenário migracao-inofensiva, Matriz E2E, Modo difícil (+33 more)

### Community 12 - "Finding"
Cohesion: 0.11
Nodes (40): Evidence, Input, Detectores aceitam as duas convenções HTTP e ignoram spans internos, Detector database_http_trace_correlation, Detector database_timeout, Detector latency_delta, Serviço payment-service, Serviço postgres da demo (+32 more)

### Community 13 - "Snapshot"
Cohesion: 0.08
Nodes (35): ChangeSource, changeSourceFake, ChangeWriter, changeWriterFake, Ingestão do GitHub limitada a uma página, sem N+1, Commit, ImportRequest, Snapshot (+27 more)

### Community 14 - "testing.T"
Cohesion: 0.13
Nodes (35): main(), newRootCommand(), assertTableCount(), preparePersistedIncident(), reserveTCPAddress(), TestBlameTraceCommandExplicaFluxoHTTPPostgreSQL(), TestDiagnoseIncidentCommandDetectaRetryStorm(), TestDiagnoseIncidentCommandExplicaAmostraRepresentativa() (+27 more)

### Community 15 - "DetectTraceBreak"
Cohesion: 0.12
Nodes (31): propagationSummary, serviceLink, Detector dependency_failure, Detector trace_break, DetectDependencyFailure(), failingAncestorServices(), indexSpansByID(), isFailedSpan() (+23 more)

### Community 16 - "Integração GitHub"
Cohesion: 0.25
Nodes (8): Módulo integrations, Índice composto (trace_id, timestamp, id), Integração GitHub, Integração PostgreSQL, Uma página por execução na coleta GitHub, Pool *sql.DB único por banco, Proibição de consultas N+1, Regras de implementação para código e banco

### Community 17 - "detectors_test.go"
Cohesion: 0.15
Nodes (30): error_rate_delta ignora variação de amostragem, Detector error_rate_delta, DetectErrorRateDelta(), exceedsSamplingNoise(), Run(), assertFinding(), contains(), databaseTimeoutSignals() (+22 more)

### Community 18 - "Faultmap MVP — Arquitetura"
Cohesion: 0.16
Nodes (29): Agents, Applications (Go / Python / Node), CLI Output, Detection Engine, Diagnostic Score, Faultmap MVP — Arquitetura, Entradas, Evidence Graph (+21 more)

### Community 19 - "time.Time"
Cohesion: 0.13
Nodes (8): leitorDeSchemaQueFalha, retentionRemoverStub, scopedSignalReaderFake, scopeReaderFake, scopeReaderNiveis, signalReaderFake, time.Time, ScopeRepository

### Community 20 - "openDiagnosisRepository"
Cohesion: 0.15
Nodes (27): DiagnosisRepository, assertIntPointerEqual(), assertTimePointerEqual(), testDiagnosisAt(), TestDiagnosisRepositoryGetRespectsCanceledContext(), TestDiagnosisRepositoryGetRestoresCompleteSnapshot(), TestDiagnosisRepositoryGetReturnsTypedNotFound(), TestDiagnosisRepositoryGetSupportsLegacySnapshot() (+19 more)

### Community 21 - "DiagnoseIncidentInScope"
Cohesion: 0.21
Nodes (24): ScopedDeploymentReader, ScopedDiagnosisRequest, ScopeDiscovery, ScopedSchemaChangeReader, ScopedSignalReader, ScopeReader, A investigação compara serviços descobertos pelos traces, applyDependencyTieBreak() (+16 more)

### Community 22 - "encoding/json.RawMessage"
Cohesion: 0.27
Nodes (15): encoding/json.RawMessage, IncidentHistoryReader, decodeArguments(), describeFindings(), marshalIndented(), toolDefinitions(), toolError(), callTool() (+7 more)

### Community 23 - "Diagnosis"
Cohesion: 0.15
Nodes (18): DiagnosisStore, diagnosisStoreFake, DiagnosisID(), Diagnosis, PersistDiagnosis(), TestDiagnosisIDEDeterministicoEmUTC(), TestPersistDiagnosisNaoSalvaIncidenteSemSinais(), TestPersistDiagnosisPreservaCausaDoStore() (+10 more)

### Community 24 - "otlp_json.go"
Cohesion: 0.23
Nodes (19): exceptionAttributes(), normalizeAttributes(), normalizeExportRequest(), normalizeSpan(), parseUnixNano(), resourceSignalAttributes(), scalarJSONValue(), serviceName() (+11 more)

### Community 25 - "NewInvestigationWindowFromIncident"
Cohesion: 0.23
Nodes (22): cadeiaSignal(), escopoHTTPSignal(), escopoSpanDeBanco(), sinalComVersao(), TestDiagnoseScopeAcusaOCommitImplantado(), TestDiagnoseScopeAcusaSchemaQuandoHaSintoma(), TestDiagnoseScopeAlcançaSegundoSalto(), TestDiagnoseScopeConsultaDeploymentsEmLote() (+14 more)

### Community 26 - "context.Context"
Cohesion: 0.17
Nodes (13): context.Context, database/sql.NullInt64, database/sql.NullTime, database/sql.Tx, DiagnosisRepository, nullIntPointer(), nullTimePointer(), readPersistedFindings() (+5 more)

### Community 27 - "detectDeploymentProximity"
Cohesion: 0.25
Nodes (13): deploymentCandidate, commitLabel(), deploymentSummary(), detectDeploymentProximity(), DetectDeploymentProximityFindings(), observedVersions(), sameStringSet(), sortedSetValues() (+5 more)

### Community 28 - "run-e2e.sh"
Cohesion: 0.16
Nodes (12): activate_scenario(), assert_contains(), assert_json(), cleanup(), compose(), diagnose(), first_trace_id(), run_scenario() (+4 more)

### Community 29 - "DetectDatabaseError"
Cohesion: 0.32
Nodes (12): Detector database_error, DetectDatabaseError(), bancoComFalhas(), databaseErrorSignals(), TestDatabaseErrorAcusaFalhaVerdadeira(), TestDatabaseErrorDetectaCrescimentoDeFalhasDeConexão(), TestDatabaseErrorIgnoraCancelamentoDeInstrumentaçãoReal(), TestDatabaseErrorIgnoraCancelamentoDoCliente() (+4 more)

### Community 30 - "time.Duration"
Cohesion: 0.18
Nodes (15): TestPostgresRepositoryAtrasoOcupaConexaoDoPool(), TestPostgresRepositoryIntegracao(), NewPostgresRepository(), recordDatabaseSpanError(), mustPostgresRepository(), TestNewPostgresRepositoryRejeitaPoolAusente(), TestPostgresRepositoryAtrasoRespeitaContexto(), TestPostgresRepositoryCreateUsaConsultaParametrizada() (+7 more)

### Community 31 - "DetectRetryStorm"
Cohesion: 0.13
Nodes (25): retryCandidate, retryOperationStats, Detector retry_storm, Critérios de aceite do MVP, Métricas de sucesso, Sistema de demonstração demo-shop, Primeira demonstração obrigatória, Roadmap de dez marcos (+17 more)

### Community 32 - "openSchemaRepository"
Cohesion: 0.27
Nodes (16): TestListSchemaChangesLimitaAConsulta(), TestSaveSnapshotAceitaBaseVaziaDesdeOInicio(), TestSaveSnapshotFalhaComBancoFechado(), TestSaveSnapshotFalhaComColetaAnteriorCorrompida(), TestSaveSnapshotRecusaColetaGigante(), TestSaveSnapshotRecusaColetaVaziaContraCatalogoPovoado(), TestSaveSnapshotRejeitaColetaSemIdentidade(), TestSaveSnapshotRespeitaContextoCancelado() (+8 more)

### Community 33 - "Signal"
Cohesion: 0.18
Nodes (19): traceReaderFake, Telemetria de instrumentação real como base de teste, SignalType, math/rand.Rand, attributeValueOrEmpty(), databaseFailureTypeSuffix(), databaseNonTimeoutFailures(), isClientCancellation() (+11 more)

### Community 34 - "Faultmap"
Cohesion: 0.15
Nodes (13): Backend determinístico, CLI-first e local-first, Explicabilidade, Faultmap, Infraestrutura existente e evolução controlada, Interface Detector, Módulo detection, Monólito modular em Go (+5 more)

### Community 35 - "DetectVersionRegression"
Cohesion: 0.27
Nodes (14): versionStats, Detector version_regression, errorRate(), comparableVersionStats(), DetectVersionRegression(), TestVersionRegressionComparaDuasVersõesNaMesmaJanela(), TestVersionRegressionDetectaLatênciaPiorEmUmaVersão(), TestVersionRegressionIgnoraDiferençaDentroDoRuído() (+6 more)

### Community 36 - "otlp_protobuf.go"
Cohesion: 0.22
Nodes (15): go.opentelemetry.io/proto/otlp/collector/trace/v1.ExportTraceServiceRequest, go.opentelemetry.io/proto/otlp/common/v1.ArrayValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValueList, go.opentelemetry.io/proto/otlp/trace/v1.Span, go.opentelemetry.io/proto/otlp/trace/v1.Span_Event, marshalProtoValue(), protobufAnyValue() (+7 more)

### Community 37 - "New"
Cohesion: 0.24
Nodes (13): Identificadores de catálogo ordenáveis por tempo, encode(), extract(), New(), TestAlfabetoNaoTemCaracteresAmbiguos(), TestClassificabilidade(), TestClassificabilidadeDistingueMilissegundos(), TestDeterminismoPreservaIdempotencia() (+5 more)

### Community 38 - "CLI faultmap"
Cohesion: 0.14
Nodes (15): Módulo reporting, Módulo storage, Persistência SQLite, Snapshot imutável de diagnóstico, Artefatos gerados em faultmap-out, Comando faultmap blame trace, CLI faultmap, Comando faultmap diagnose incident (+7 more)

### Community 39 - "run"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConvertePoolPostgres(), TestLoadConfigRejeitaIdleMaiorQueOpen(), config

### Community 40 - "run-hard-mode.sh"
Cohesion: 0.28
Nodes (14): apply_harmless_migration(), assert_contains(), assert_no_finding(), cleanup(), collect_schema(), compose(), diagnose(), generate_burst_traffic() (+6 more)

### Community 41 - "PersistedDiagnosis"
Cohesion: 0.05
Nodes (69): incidentHistoryReaderFake, timeline.json ancora findings na janela do incidente, IncidentSummary, PersistedDiagnosis, ListIncidents(), TestGetIncidentValidaIDEPropagaAusencia(), TestListIncidentsPreservaOrdemDoRepositorio(), TestListIncidentsValidaLimiteAntesDoRepositorio() (+61 more)

### Community 42 - "NewTimeWindow"
Cohesion: 0.26
Nodes (10): InvestigationWindow, TimeWindow, NewInvestigationWindow(), NewTimeWindow(), TestNewInvestigationWindowAllowsBaselineToMeetIncidentBoundary(), TestNewInvestigationWindowFromIncidentCalculatesContiguousBaseline(), TestNewInvestigationWindowFromIncidentRejectsInvalidInputs(), TestNewInvestigationWindowRejectsOverlappingOrInvalidWindows() (+2 more)

### Community 43 - "database/sql.DB"
Cohesion: 0.40
Nodes (13): database/sql.DB, assertIndexColumns(), assertIndexExists(), assertMigrationVersion(), assertTableExists(), createVersionOneSchema(), migrationRowCount(), openMigrationTestDatabase() (+5 more)

### Community 44 - "ParseOTLPTraces"
Cohesion: 0.16
Nodes (22): go.opentelemetry.io/proto/otlp/common/v1.AnyValue, io.Reader, classifyOTLPError(), OTLPEncoding, contextError(), ParseOTLPJSON(), assertSignalEqual(), intAnyValue() (+14 more)

### Community 45 - "run"
Cohesion: 0.23
Nodes (10): NewHTTPServer(), RunHTTPServer(), SignalContext(), TestNewHTTPServerConfiguraLimitesEHealth(), TestRunHTTPServerEncerraComContexto(), loadConfig(), main(), run() (+2 more)

### Community 46 - "faultmap_app.py"
Cohesion: 0.19
Nodes (10): _current_fault(), _fault_file(), _FaultInjector, _install_database_delay(), _parse_fault(), Ponto de entrada que instrumenta o DuckDB antes de carregar a aplicação. Fica…, Atrasa cada consulta quando a falha ativa for db_slow. O patch é no execute da…, Arquivo consultado a cada requisição para saber se a falha está ligada. Ler de… (+2 more)

### Community 47 - "CommonCauses"
Cohesion: 0.33
Nodes (7): Frases de causas comuns, CommonCauses(), TestCausasComunsCobremCatálogoDeTerceiro(), TestCausasComunsIgnoraRegraDesconhecida(), TestCausasComunsOferecemAlternativas(), TestTodaRegraDizOQueAquelePadrãoCostumaSignificar(), TestSchemaChangeProximityTemCausasComuns()

### Community 48 - "IngestTelemetry"
Cohesion: 0.30
Nodes (11): IngestionResult, SignalStore, signalStoreFake, contextError(), IngestLogs(), IngestTelemetry(), IngestTelemetryFile(), TestIngestTelemetryFileNormalizaEPersiste() (+3 more)

### Community 49 - "acceptance_test.go"
Cohesion: 0.42
Nodes (11): readRequiredFile(), requireContains(), TestDemoPublicaPortasSomenteNoLoopback(), TestDemoUsaPortaDeHostDedicada(), TestLoadGeneratorRecebeIdentidadeOTel(), TestReadmesDocumentamExecucaoDaDemo(), TestRunnerE2EDeclaraMatrizLimitesELimpeza(), TestScenariosDocumentamContratoReproduzivel() (+3 more)

### Community 50 - "DetectLogCorrelation"
Cohesion: 0.36
Nodes (10): DetectLogCorrelation(), errorLogs(), filterLogSignals(), logsDeErro(), requisicoes(), TestLogCorrelationExigeCorrelaçãoComTrace(), TestLogCorrelationIgnoraRuídoConstante(), TestLogCorrelationLigaErrosDeLogAoTraceQueFalhou() (+2 more)

### Community 51 - "schema_mcp_integration_test.go"
Cohesion: 0.31
Nodes (13): executar(), gravarColetasDeCatalogo(), gravarTelemetriaDeBanco(), sessaoMCP(), TestIngestSchemaCommandNaoGravaCredencialNoWorkspace(), TestIngestSchemaCommandValidaEntradaAntesDeAbrirConexao(), TestInitEphemeralAvisaQueNadaEhApagadoSozinho(), TestInitEphemeralCriaWorkspaceUsavelForaDoProjeto() (+5 more)

### Community 52 - "Environment"
Cohesion: 0.29
Nodes (5): loadConfig(), main(), run(), Environment, config

### Community 53 - "clienteComMock"
Cohesion: 0.48
Nodes (6): clienteComMock(), sqlmock.Sqlmock, TestFetchFalhaQuandoOCatalogoDevolveTipoInesperado(), TestFetchPropagaFalhaDeCadaConsulta(), TestFetchPropagaFalhaNoMeioDaLeitura(), TestFetchRespeitaContextoCancelado()

### Community 54 - "ApplyRetention"
Cohesion: 0.21
Nodes (14): podadorDeCatalogoFake, RetentionRequest, RetentionResult, SchemaCatalogPruner, SignalRetentionRemover, ApplyRetention(), pruneSchemaCatalogs(), TestApplyRetentionCalculaCorteEEncerraQuandoNãoHáMaisSinais() (+6 more)

### Community 55 - "NewEnvironment"
Cohesion: 0.33
Nodes (7): mapLookup(), TestLoadConfigConverteGitHubMock(), TestLoadConfigExigeSHA(), Lookup, NewEnvironment(), TestEnvironmentRejeitaConfiguracaoInsegura(), TestEnvironmentValidaValoresObrigatorios()

### Community 56 - "DetectDatabaseLatencyDelta"
Cohesion: 0.38
Nodes (9): DetectDatabaseLatencyDelta(), exceedsDatabaseLatencyNoise(), succeededDatabaseSignals(), bancoComLatência(), TestDatabaseLatencyDeltaAcusaBancoQueDegradouSemFalhar(), TestDatabaseLatencyDeltaExigeDuasJanelas(), TestDatabaseLatencyDeltaIgnoraOperaçõesQueFalharam(), TestDatabaseLatencyDeltaIgnoraOscilaçãoNormal() (+1 more)

### Community 57 - "Deployment"
Cohesion: 0.20
Nodes (10): scopedDeploymentReaderFake, Deployment, NewChangeRepository(), changeSnapshot(), deploymentAt(), openChangeRepository(), TestChangeRepositoryListDeploymentsFiltersOrdersAndLimits(), TestChangeRepositorySaveChangesIsAtomicAndIdempotent() (+2 more)

### Community 58 - "run"
Cohesion: 0.39
Nodes (7): config, loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCheckout(), TestLoadConfigRejeitaRetryIlimitado()

### Community 59 - "testhelpers_test.go"
Cohesion: 0.33
Nodes (4): diagnosisReaderFake, diagnosisDatabaseSignal(), hasContribution(), hasFinding()

### Community 60 - "run"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCargaLimitada(), TestLoadConfigRejeitaConcorrenciaExcessiva(), config

### Community 61 - "Setup"
Cohesion: 0.80
Nodes (3): Setup(), traceEndpoint(), Config

### Community 62 - "InitializeProject"
Cohesion: 0.27
Nodes (11): ensureContext(), EphemeralProjectDir(), InitializeProject(), assertDirectoryExists(), assertFileExists(), TestEphemeralProjectDirCriaForaDoProjeto(), TestEphemeralProjectDirNaoColide(), TestEphemeralProjectDirServeAoInitCompleto() (+3 more)

### Community 63 - "filterDatabaseSignals"
Cohesion: 0.30
Nodes (11): databaseSystem(), databaseSystems(), filterDatabaseSignals(), carregarFixtureReal(), separarPorFalha(), serviçosDaFixture(), TestDetectorDeBancoDisparaComTelemetriaReal(), TestDetectoresNovosNãoAcusamTelemetriaRealSaudável() (+3 more)

### Community 64 - "commitSHAFromEvidence"
Cohesion: 0.40
Nodes (5): Configuração E2E com GitHub, Mock GitHub no loopback do container, Matriz E2E automatizada, Override timeout-after-deploy (SHA como SERVICE_VERSION), commitSHAFromEvidence()

### Community 66 - "ParseOTLPLogsJSON"
Cohesion: 0.23
Nodes (14): encoding/json.Decoder, rejectTrailingJSON(), logRecordID(), logSeverity(), normalizeLogRecord(), ParseOTLPLogsJSON(), TestParseOTLPLogsNuncaArmazenaOTextoDaMensagem(), TestParseOTLPLogsPreservaSeveridadeECorrelação() (+6 more)

### Community 67 - "ListSignals"
Cohesion: 0.43
Nodes (5): SignalReader, ListSignals(), TestListSignalsEncaminhaConsulta(), TestListSignalsRespeitaContextoCancelado(), TestListSignalsValidaEntradaAntesDeConsultar()

### Community 68 - "exigirSinaisCoerentes"
Cohesion: 0.67
Nodes (6): testing.F, exigirErroClassificado(), exigirSinaisCoerentes(), FuzzParseOTLPLogsJSONNãoDevolveOCorpo(), FuzzParseOTLPTracesJSON(), FuzzParseOTLPTracesProtobuf()

### Community 69 - "Ingestão OTLP"
Cohesion: 0.18
Nodes (11): Módulo telemetry, Allowlist de atributos de Resource, Ingestão OTLP, Receiver OTLP HTTP POST /v1/traces, Signal, Detector deployment_proximity, Comando faultmap serve, Receiver OTLP sem autenticação nem TLS (+3 more)

### Community 70 - "Grafo de evidências"
Cohesion: 0.40
Nodes (5): Módulo evidence, EvidenceEdge, EvidenceNode, Fallback de parentesco de spans, Grafo de evidências

### Community 72 - "rodar-ranking.sh"
Cohesion: 0.83
Nodes (3): limpar(), rodar-ranking.sh script, trafego()

### Community 88 - "RenderDiagnosis"
Cohesion: 0.36
Nodes (9): RenderDiagnosis(), renderDiagnosisForTest(), TestRenderDiagnosisApresentaRegrasDeFormaHumanaEAuditavel(), TestRenderDiagnosisApresentaRetryStormEmPortugues(), TestRenderDiagnosisConsolidaLimitacoesRepetidas(), TestRenderDiagnosisDistingueCommitDeServiço(), TestRenderDiagnosisDizOQueAquelePadrãoCostumaSignificar(), TestRenderDiagnosisMantemLimitacaoEspecificaNaHipotese() (+1 more)

### Community 89 - "Piloto cego"
Cohesion: 0.38
Nodes (7): Collector do piloto, Exportadores separados para traces e logs, Papéis de operador e investigador, Piloto cego, Sequência recomendada de incidentes, Registro do investigador, Resultado do piloto cego

### Community 91 - "repositorioComRetencao"
Cohesion: 0.50
Nodes (8): coletaDe(), repositorioComRetencao(), TestColetaContraLinhaDeBaseEsvaziadaFalhaEmVezDeInventarMigracao(), TestPruneAvancaEmLotes(), TestPruneEhIdempotente(), TestPruneLiberaOCatalogoAntigoSemApagarAsMudancas(), TestPrunePreservaAColetaMaisRecenteDeCadaBase(), RetentionRepository

### Community 93 - "Fixtures OpenTelemetry"
Cohesion: 0.18
Nodes (11): Atributos OpenTelemetry prioritários, InvestigationWindow, Janelas de investigação (baseline e incidente), Metadados legados anuláveis, Testes obrigatórios, Estratégia expand-and-contract, Baseline mínima de 30 sinais por serviço, StriderEdge (aplicação FastAPI + DuckDB de terceiros) (+3 more)

### Community 95 - "ADR 0015 — A retenção libera o catálogo e preserva as mudanças"
Cohesion: 0.40
Nodes (4): ADR 0015 — A retenção libera o catálogo e preserva as mudanças, Consequências, Contexto, Decisão

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
- **Why does `Signal` connect `Signal` to `Rank`, `graph.go`, `attributes.go`, `NewSignalRepository`, `DetectSchemaChangeProximity`, `Finding`, `DetectTraceBreak`, `detectors_test.go`, `time.Time`, `DiagnoseIncidentInScope`, `otlp_json.go`, `NewInvestigationWindowFromIncident`, `detectDeploymentProximity`, `DetectDatabaseError`, `DetectRetryStorm`, `DetectVersionRegression`, `ParseOTLPTraces`, `IngestTelemetry`, `DetectLogCorrelation`, `DetectDatabaseLatencyDelta`, `testhelpers_test.go`, `filterDatabaseSignals`, `ParseOTLPLogsJSON`, `ListSignals`, `exigirSinaisCoerentes`?**
  _High betweenness centrality (0.098) - this node is a cross-community bridge._
- **Why does `Finding` connect `Finding` to `Rank`, `Suspect`, `DetectSchemaChangeProximity`, `DetectTraceBreak`, `detectors_test.go`, `openDiagnosisRepository`, `DiagnoseIncidentInScope`, `encoding/json.RawMessage`, `Diagnosis`, `context.Context`, `detectDeploymentProximity`, `DetectDatabaseError`, `DetectRetryStorm`, `DetectVersionRegression`, `PersistedDiagnosis`, `DetectLogCorrelation`, `DetectDatabaseLatencyDelta`, `testhelpers_test.go`, `commitSHAFromEvidence`, `RenderDiagnosis`?**
  _High betweenness centrality (0.033) - this node is a cross-community bridge._
- **Why does `DiagnoseIncidentInScope()` connect `DiagnoseIncidentInScope` to `Migrate`, `Rank`, `NewSignalRepository`, `DetectTraceBreak`, `detectors_test.go`, `Diagnosis`, `NewInvestigationWindowFromIncident`, `context.Context`, `detectDeploymentProximity`?**
  _High betweenness centrality (0.022) - this node is a cross-community bridge._