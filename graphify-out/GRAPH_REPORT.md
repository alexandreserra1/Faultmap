# Graph Report - Faultmap  (2026-09-22)

## Corpus Check
- 262 files · ~333,495 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2051 nodes · 5875 edges · 112 communities (97 shown, 15 thin omitted)
- Extraction: 86% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 792 edges (avg confidence: 0.82)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `630bafbd`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- SchemaSnapshot
- Load
- Rank
- SchemaChange
- graph.go
- IngestFunc
- historicoComUmIncidente
- attributes.go
- payment/handler_test.go
- NewSignalRepository
- NewHandler
- DetectSchemaChangeProximity
- Finding
- io.Writer
- newRootCommand
- DetectTraceBreak
- Integração GitHub
- detectors_test.go
- Faultmap MVP — Arquitetura
- time.Time
- openDiagnosisRepository
- diagnose_scope.go
- encoding/json.RawMessage
- conformidade.go
- otlp_json.go
- DiagnoseIncidentInScope
- database/sql.Tx
- Input
- run-e2e.sh
- Signal
- postgres/diagnosis_repository.go
- DetectRetryStorm
- Open
- NewPolicy
- Faultmap
- DetectVersionRegression
- otlp_protobuf.go
- testing.T
- CLI faultmap
- .SaveSnapshot
- run-hard-mode.sh
- json/report.go
- InvestigationWindow
- database/sql.DB
- ParseOTLPJSON
- medir-ruido-do-banco.py
- faultmap_app.py
- timeline.go
- context.Context
- acceptance_test.go
- DetectLogCorrelation
- schema_mcp_integration_test.go
- Render
- clienteComMock
- ApplyRetention
- Demo Shop
- DetectDatabaseLatencyDelta
- openChangeRepository
- sortear-incidente-test.sh
- sqlite/diagnosis_repository.go
- Diff
- medir-ruido-do-banco.sh
- InitializeProject
- Environment
- Deployment
- ParseOTLPLogsJSON
- RenderPersistedDiagnosis
- exigirSinaisCoerentes
- Ingestão OTLP
- Grafo de evidências
- Write
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
- Snapshot
- PersistedDiagnosis
- Piloto cego
- postgres/signal_repository.go
- repositorioComRetencao
- deploy-inofensivo
- coletar
- ADR 0015 — A retenção libera o catálogo e preserva as mudanças
- postgres-demo
- ExplainSuspect
- Sorteio cego na demo-shop
- RenderRanking
- Retenção apaga telemetria e preserva snapshots
- RenderIncidentSummary
- testSignal
- Migrate
- TestConformidadeIntegracaoPostgres
- sqlite/change_repository.go
- catalog_integration_test.go
- NewClient
- ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada
- RenderDiagnosis
- scanSignal
- A coleta de schema guarda identificadores, não expressões
- sortear-incidente.sh
- Fixtures OpenTelemetry

## God Nodes (most connected - your core abstractions)
1. `Signal` - 143 edges
2. `Finding` - 59 edges
3. `Rank()` - 45 edges
4. `newRootCommand()` - 42 edges
5. `DiagnoseIncidentInScope()` - 41 edges
6. `DetectSchemaChangeProximity()` - 37 edges
7. `Load()` - 34 edges
8. `PersistedDiagnosis` - 32 edges
9. `NewInvestigationWindowFromIncident()` - 31 edges
10. `Open()` - 29 edges

## Surprising Connections (you probably didn't know these)
- `Cenário migracao-inofensiva` --references--> `DetectSchemaChangeProximity()`  [INFERRED]
  CHANGELOG.md → internal/detection/schema_change_proximity.go
- `Defeito no checkout, não na dependência` --references--> `Rank()`  [INFERRED]
  examples/demo-shop/scenarios/retry-storm/compose.yaml → internal/ranking/ranking.go
- `Ranking auditável de suspeitos` --references--> `Rank()`  [INFERRED]
  README.md → internal/ranking/ranking.go
- `Mapeamento de regras para classes de peso` --rationale_for--> `weightClassForRule()`  [INFERRED]
  docs/mvp/03-diagnostico-integracoes-e-saidas.md → internal/ranking/ranking.go
- `Pool *sql.DB único por banco` --semantically_similar_to--> `Cenário: pool pequeno`  [INFERRED] [semantically similar]
  docs/mvp/05-entrega-e-diretrizes.md → examples/demo-shop/scenarios/small-pool/README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Fluxo de ingestão OTLP da demo até o SQLite** — examples_demo_shop_compose_checkout_service, examples_demo_shop_compose_payment_service, examples_demo_shop_otel_collector_pipeline_de_traces, docs_mvp_02_dominio_dados_e_telemetria_receiver_otlp_http, docs_mvp_02_dominio_dados_e_telemetria_persistencia_sqlite [EXTRACTED 1.00]
- **Protocolo do piloto cego** — examples_pilot_readme_piloto_cego, examples_pilot_readme_operador_e_investigador, examples_pilot_collector_collector_do_piloto, examples_pilot_registro_registro_do_investigador, examples_pilot_resultado_resultado_do_piloto_cego [EXTRACTED 1.00]
- **Cegueira contra instrumentação real** — docs_adr_0006_detectores_aceitam_duas_convencoes_http_duas_convencoes_http, docs_adr_0007_telemetria_real_como_base_de_teste_telemetria_real_como_base_de_teste, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_semconv_attributes, readme_compatibilidade_com_instrumentacao_real, _claude_skills_consultar_grafo_skill_consultar_o_grafo_antes_de_mudar [INFERRED 0.85]
- **Cenários que exercitam evidência de banco** — examples_demo_shop_scenarios_database_slow_readme_cenario_banco_lento, examples_demo_shop_scenarios_small_pool_readme_cenario_pool_pequeno, examples_demo_shop_scenarios_table_lock_readme_cenario_lock_na_tabela_de_pagamentos, internal_detection_detectors_detectdatabasetimeout, internal_detection_detectors_detectlatencydelta [INFERRED 0.85]
- **Orçamento de peso por classe no ranking** — docs_adr_0001_ranking_reutiliza_peso_graph_proximity_reuso_do_peso_graph_proximity, docs_adr_0010_teto_por_classe_de_peso_no_ranking_teto_por_classe_de_peso, changelog_teto_relativo_para_evidencia_de_apoio, internal_ranking_ranking_weightclassforrule, internal_ranking_ranking_weightforclass, internal_ranking_ranking_rank [INFERRED 0.85]
- **Privacidade aplicada antes do disco** — docs_adr_0008_politica_de_privacidade_aplicada_na_ingestao_politica_aplicada_na_ingestao, docs_adr_0011_logs_guardam_apenas_metadados_logs_sem_texto_da_mensagem, docs_adr_0012_bloqueios_de_privacidade_somam_em_vez_de_substituir_uniao_de_bloqueios, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_privacy_policy, readme_privacidade [INFERRED 0.85]

## Communities (112 total, 15 thin omitted)

### Community 0 - "SchemaSnapshot"
Cohesion: 0.15
Nodes (18): escritorDeSchemaFake, fonteDeSchemaFake, SchemaSource, SchemaSnapshot, SchemaImportResult, SchemaWriter, IngestSchema(), coletaComDoisObjetos() (+10 more)

### Community 1 - "Load"
Cohesion: 0.06
Nodes (99): Conexao, RepositorioDeCatalogo, RepositorioDeDiagnostico, RepositorioDeMudancas, diagnosisEnd(), githubImportEnd(), newBlameCommand(), newBlameTraceCommand() (+91 more)

### Community 2 - "Rank"
Cohesion: 0.08
Nodes (46): Módulo ranking, ScoreContribution, Suspect, Ranking de suspeitos, Desempate pela evidência de banco, math/rand.Rand, clamp(), contributionReason() (+38 more)

### Community 3 - "SchemaChange"
Cohesion: 0.20
Nodes (16): scopedSchemaReaderFake, schemaChangeCandidate, SchemaChange, SchemaChangeKind, SchemaObject, SchemaObjectKind, alteredDetail(), countByKind() (+8 more)

### Community 4 - "graph.go"
Cohesion: 0.07
Nodes (54): TraceInvestigation, RepositorioDeSinais, edgeAccumulator, nodeAccumulator, NodeKind, BlameTrace(), TraceSignalReader, hasRelation() (+46 more)

### Community 5 - "IngestFunc"
Cohesion: 0.07
Nodes (51): authenticate(), NewHandler(), TestHandlerEntregaCommitEDeploymentCoerentes(), TestHandlerProtegeRotasEValidaConfiguracao(), validateConfig(), writeJSON(), delayFromFile(), NewHandler() (+43 more)

### Community 6 - "historicoComUmIncidente"
Cohesion: 0.16
Nodes (25): TestAuditoriaNaoEscreveNaSaidaDoProtocolo(), TestAuditoriaNaoRegistraOsArgumentos(), TestArgumentosComTipoErradoViramErroDeTool(), TestContextoCanceladoEncerraASessao(), TestErroDoRepositorioViraErroDeToolENaoDerrubaOServidor(), TestIncidenteInexistenteExplicaOQueAconteceu(), TestLinhaGiganteNaoDerrubaASessao(), TestLoteJSONRPCRecebeRespostaEmVezDeSilencio() (+17 more)

### Community 7 - "attributes.go"
Cohesion: 0.07
Nodes (45): Consultar o grafo antes de mudar, Grafo de conhecimento em graphify-out, graphify affected — travessia reversa, Limites do grafo, databaseTargetsInWindow(), sortedKeys(), safeRetryIdentity(), databaseTargetsQueriedBy() (+37 more)

### Community 8 - "payment/handler_test.go"
Cohesion: 0.09
Nodes (33): chronicFailure(), NewHandler(), executePayment(), mustHandler(), TestHandlerConverteFalhaDoBancoSemVazarDetalhes(), TestHandlerErroCrônicoÉDeterminísticoPorPedido(), TestHandlerForcaStatusSemPersistir(), TestHandlerPersistePagamento() (+25 more)

### Community 9 - "NewSignalRepository"
Cohesion: 0.27
Nodes (17): TestConformidadeSQLite(), NewRetentionRepository(), openRetentionDatabase(), signalIDs(), TestRetentionRepositoryPreservaSnapshotsDeIncidentes(), TestRetentionRepositoryRejeitaLimiteInválido(), TestRetentionRepositoryRemoveSomenteSinaisAnterioresAoCorte(), TestRetentionRepositoryRespeitaLimiteDoLote() (+9 more)

### Community 10 - "NewHandler"
Cohesion: 0.06
Nodes (46): Config, Handler, Request, roundTripperFunc, Ingestão do GitHub limitada a uma página, sem N+1, Serviço checkout-service, Serviço load-generator, NewHandler() (+38 more)

### Community 11 - "DetectSchemaChangeProximity"
Cohesion: 0.24
Nodes (23): Pontuação pela ponta pessimista do intervalo, containsLimitation(), DetectSchemaChangeProximity(), entradaComBanco(), mudancaDeSchema(), spansDeBanco(), spansDeBancoComoAInstrumentacaoReal(), TestDetectSchemaChangeProximityAcusaServicoQueFalaComABase() (+15 more)

### Community 12 - "Finding"
Cohesion: 0.15
Nodes (29): Evidence, Detector database_http_trace_correlation, Detector database_timeout, Detector latency_delta, clamp(), databaseTimeoutSummary(), DetectDatabaseTimeout(), DetectLatencyDelta() (+21 more)

### Community 13 - "io.Writer"
Cohesion: 0.18
Nodes (15): rejectionReporter(), bufio.Reader, encoding/json.Encoder, io.Writer, newAuditor(), dispatch(), handle(), protocolError() (+7 more)

### Community 14 - "newRootCommand"
Cohesion: 0.13
Nodes (29): main(), newRootCommand(), assertTableCount(), preparePersistedIncident(), reserveTCPAddress(), TestBlameTraceCommandExplicaFluxoHTTPPostgreSQL(), TestDiagnoseIncidentCommandDetectaRetryStorm(), TestDiagnoseIncidentCommandExplicaAmostraRepresentativa() (+21 more)

### Community 15 - "DetectTraceBreak"
Cohesion: 0.12
Nodes (31): propagationSummary, serviceLink, Detector dependency_failure, Detector trace_break, DetectDependencyFailure(), failingAncestorServices(), indexSpansByID(), isFailedSpan() (+23 more)

### Community 16 - "Integração GitHub"
Cohesion: 0.25
Nodes (8): Módulo integrations, Índice composto (trace_id, timestamp, id), Integração GitHub, Integração PostgreSQL, Uma página por execução na coleta GitHub, Pool *sql.DB único por banco, Proibição de consultas N+1, Regras de implementação para código e banco

### Community 17 - "detectors_test.go"
Cohesion: 0.17
Nodes (28): Detector error_rate_delta, DetectErrorRateDelta(), Run(), assertFinding(), contains(), databaseTimeoutSignals(), httpSignals(), internalHTTPSendSignals() (+20 more)

### Community 18 - "Faultmap MVP — Arquitetura"
Cohesion: 0.16
Nodes (29): Agents, Applications (Go / Python / Node), CLI Output, Detection Engine, Diagnostic Score, Faultmap MVP — Arquitetura, Entradas, Evidence Graph (+21 more)

### Community 19 - "time.Time"
Cohesion: 0.09
Nodes (16): diagnosisReaderFake, leitorDeSchemaQueFalha, retentionRemoverStub, scopedSignalReaderFake, scopeReaderFake, scopeReaderNiveis, signalReaderFake, time.Time (+8 more)

### Community 20 - "openDiagnosisRepository"
Cohesion: 0.15
Nodes (27): DiagnosisRepository, assertIntPointerEqual(), assertTimePointerEqual(), testDiagnosisAt(), TestDiagnosisRepositoryGetRespectsCanceledContext(), TestDiagnosisRepositoryGetRestoresCompleteSnapshot(), TestDiagnosisRepositoryGetReturnsTypedNotFound(), TestDiagnosisRepositoryGetSupportsLegacySnapshot() (+19 more)

### Community 21 - "diagnose_scope.go"
Cohesion: 0.18
Nodes (22): ScopedDiagnosisRequest, ScopeDiscovery, A investigação compara serviços descobertos pelos traces, applyDependencyTieBreak(), containsService(), corroboratedProximityFindings(), deploymentsForService(), expandScopeByTraces() (+14 more)

### Community 22 - "encoding/json.RawMessage"
Cohesion: 0.27
Nodes (15): encoding/json.RawMessage, IncidentHistoryReader, decodeArguments(), describeFindings(), marshalIndented(), toolDefinitions(), toolError(), callTool() (+7 more)

### Community 23 - "conformidade.go"
Cohesion: 0.11
Nodes (41): diagnosisStoreFake, DiagnosisID(), Diagnosis, DiagnosisStore, PersistDiagnosis(), TestDiagnosisIDEDeterministicoEmUTC(), TestPersistDiagnosisNaoSalvaIncidenteSemSinais(), TestPersistDiagnosisPreservaCausaDoStore() (+33 more)

### Community 24 - "otlp_json.go"
Cohesion: 0.23
Nodes (19): exceptionAttributes(), normalizeAttributes(), normalizeExportRequest(), normalizeSpan(), parseUnixNano(), resourceSignalAttributes(), scalarJSONValue(), serviceName() (+11 more)

### Community 25 - "DiagnoseIncidentInScope"
Cohesion: 0.29
Nodes (23): DiagnoseIncidentInScope(), escopoHTTPSignal(), escopoSpanDeBanco(), sinalComVersao(), TestDiagnoseScopeAcusaOCommitImplantado(), TestDiagnoseScopeAcusaSchemaQuandoHaSintoma(), TestDiagnoseScopeAlcançaSegundoSalto(), TestDiagnoseScopeApresentaDeployQuandoHaSintoma() (+15 more)

### Community 26 - "database/sql.Tx"
Cohesion: 0.16
Nodes (16): database/sql.NullInt64, database/sql.NullTime, database/sql.Tx, DiagnosisRepository, nullIntPointer(), nullTimePointer(), readPersistedFindings(), readPersistedIncident() (+8 more)

### Community 27 - "Input"
Cohesion: 0.13
Nodes (24): deploymentCandidate, Input, Módulo platform, Mapeamento de regras para classes de peso, Pesos configuráveis do ranking, Configuração inicial faultmap.yaml, Configuração E2E com GitHub, Mock GitHub no loopback do container (+16 more)

### Community 28 - "run-e2e.sh"
Cohesion: 0.16
Nodes (12): activate_scenario(), assert_contains(), assert_json(), cleanup(), compose(), diagnose(), first_trace_id(), run_scenario() (+4 more)

### Community 29 - "Signal"
Cohesion: 0.13
Nodes (33): traceReaderFake, Detector database_error, attributeValueOrEmpty(), databaseFailureTypeSuffix(), databaseNonTimeoutFailures(), DetectDatabaseError(), isClientCancellation(), bancoComFalhas() (+25 more)

### Community 30 - "postgres/diagnosis_repository.go"
Cohesion: 0.29
Nodes (9): findingID(), DiagnosisRepository, preparedFinding, NewDiagnosisRepository(), prepareDiagnosis(), rollbackDiagnosisTransaction(), subjectIdentifier(), preparedDiagnosis (+1 more)

### Community 31 - "DetectRetryStorm"
Cohesion: 0.23
Nodes (16): retryCandidate, retryOperationStats, DetectRetryStorm(), retryStats(), bancoNodeRepetido(), databaseRetrySignals(), retrySignals(), serverRetrySignals() (+8 more)

### Community 32 - "Open"
Cohesion: 0.18
Nodes (20): closeAfterFailure(), Open(), TestOpenConfiguresSQLiteForFaultmap(), TestListSchemaChangesLimitaAConsulta(), TestSaveSnapshotAceitaBaseVaziaDesdeOInicio(), TestSaveSnapshotFalhaComBancoFechado(), TestSaveSnapshotFalhaComColetaAnteriorCorrompida(), TestSaveSnapshotRecusaColetaGigante() (+12 more)

### Community 33 - "NewPolicy"
Cohesion: 0.53
Nodes (5): NewPolicy(), TestPolicyIgnoraDiferençaDeCaixaEEspaços(), TestPolicyRemoveAtributosBloqueados(), TestPolicySemLimiteNãoTrunca(), TestPolicyTruncaAtributosLongos()

### Community 34 - "Faultmap"
Cohesion: 0.15
Nodes (13): Backend determinístico, CLI-first e local-first, Explicabilidade, Faultmap, Infraestrutura existente e evolução controlada, Interface Detector, Módulo detection, Monólito modular em Go (+5 more)

### Community 35 - "DetectVersionRegression"
Cohesion: 0.28
Nodes (14): versionStats, Detector version_regression, comparableVersionStats(), DetectVersionRegression(), TestVersionRegressionComparaDuasVersõesNaMesmaJanela(), TestVersionRegressionDetectaLatênciaPiorEmUmaVersão(), TestVersionRegressionIgnoraDiferençaDentroDoRuído(), TestVersionRegressionSilenciaComUmaÚnicaVersão() (+6 more)

### Community 36 - "otlp_protobuf.go"
Cohesion: 0.22
Nodes (15): go.opentelemetry.io/proto/otlp/collector/trace/v1.ExportTraceServiceRequest, go.opentelemetry.io/proto/otlp/common/v1.ArrayValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValueList, go.opentelemetry.io/proto/otlp/trace/v1.Span, go.opentelemetry.io/proto/otlp/trace/v1.Span_Event, marshalProtoValue(), protobufAnyValue() (+7 more)

### Community 37 - "testing.T"
Cohesion: 0.17
Nodes (19): Identificadores de catálogo ordenáveis por tempo, TestConfigValidateExigeIdentidadeCompleta(), TestDisabledMantemShutdownSeguro(), TestTraceEndpointAcrescentaCaminhoOTLP(), testing.T, TestCommitValidateRejectsIncompleteChange(), TestDeploymentValidateRequiresCorrelationFields(), encode() (+11 more)

### Community 38 - "CLI faultmap"
Cohesion: 0.14
Nodes (15): Módulo reporting, Módulo storage, Persistência SQLite, Snapshot imutável de diagnóstico, Artefatos gerados em faultmap-out, Comando faultmap blame trace, CLI faultmap, Comando faultmap diagnose incident (+7 more)

### Community 39 - ".SaveSnapshot"
Cohesion: 0.39
Nodes (6): NewSchemaRepository(), normalizeDatabaseNames(), readPreviousSnapshot(), rollbackSchemaTransaction(), validateSnapshot(), SchemaRepository

### Community 40 - "run-hard-mode.sh"
Cohesion: 0.26
Nodes (15): apply_harmless_migration(), assert_contains(), assert_no_finding(), cleanup(), collect_schema(), compose(), diagnose(), generate_burst_traffic() (+7 more)

### Community 41 - "json/report.go"
Cohesion: 0.29
Nodes (14): copyIntPointer(), newReport(), orderedSuspects(), reportFinding(), reportSuspect(), reportTime(), sortedStrings(), contribution (+6 more)

### Community 42 - "InvestigationWindow"
Cohesion: 0.26
Nodes (10): InvestigationWindow, TimeWindow, NewInvestigationWindow(), NewTimeWindow(), TestNewInvestigationWindowAllowsBaselineToMeetIncidentBoundary(), TestNewInvestigationWindowFromIncidentCalculatesContiguousBaseline(), TestNewInvestigationWindowFromIncidentRejectsInvalidInputs(), TestNewInvestigationWindowRejectsOverlappingOrInvalidWindows() (+2 more)

### Community 43 - "database/sql.DB"
Cohesion: 0.25
Nodes (18): database/sql.DB, applyMigration(), isMigrationApplied(), Migrate(), rollbackMigration(), assertIndexColumns(), assertIndexExists(), assertMigrationVersion() (+10 more)

### Community 44 - "ParseOTLPJSON"
Cohesion: 0.20
Nodes (15): encoding/json.Decoder, go.opentelemetry.io/proto/otlp/common/v1.AnyValue, ParseOTLPJSON(), rejectTrailingJSON(), assertSignalEqual(), intAnyValue(), stringAnyValue(), TestParseOTLPJSONHonorsCancelledContext() (+7 more)

### Community 45 - "medir-ruido-do-banco.py"
Cohesion: 0.48
Nodes (6): instante(), janelas(), main(), operações_de_banco(), percentil(), Só as que concluíram: é o que o detector mede.

### Community 46 - "faultmap_app.py"
Cohesion: 0.19
Nodes (10): _current_fault(), _fault_file(), _FaultInjector, _install_database_delay(), _parse_fault(), Ponto de entrada que instrumenta o DuckDB antes de carregar a aplicação. Fica…, Atrasa cada consulta quando a falha ativa for db_slow. O patch é no execute da…, Arquivo consultado a cada requisição para saber se a falha está ligada. Ler de… (+2 more)

### Community 47 - "timeline.go"
Cohesion: 0.10
Nodes (29): Workflow de CI, Matriz E2E fora do gate automático, Binários reproduzíveis com checksums, Workflow de Release, Cenário migracao-inofensiva, Matriz E2E, Modo difícil, make verify (+21 more)

### Community 48 - "context.Context"
Cohesion: 0.11
Nodes (24): IngestionResult, signalStoreFake, context.Context, io.Reader, contextError(), SignalStore, IngestLogs(), IngestTelemetry() (+16 more)

### Community 49 - "acceptance_test.go"
Cohesion: 0.42
Nodes (11): readRequiredFile(), requireContains(), TestDemoPublicaPortasSomenteNoLoopback(), TestDemoUsaPortaDeHostDedicada(), TestLoadGeneratorRecebeIdentidadeOTel(), TestReadmesDocumentamExecucaoDaDemo(), TestRunnerE2EDeclaraMatrizLimitesELimpeza(), TestScenariosDocumentamContratoReproduzivel() (+3 more)

### Community 50 - "DetectLogCorrelation"
Cohesion: 0.40
Nodes (9): DetectLogCorrelation(), errorLogs(), filterLogSignals(), logsDeErro(), requisicoes(), TestLogCorrelationExigeCorrelaçãoComTrace(), TestLogCorrelationIgnoraRuídoConstante(), TestLogCorrelationLigaErrosDeLogAoTraceQueFalhou() (+1 more)

### Community 51 - "schema_mcp_integration_test.go"
Cohesion: 0.23
Nodes (17): TestBackendPostgresNaoCriaArquivoLocal(), TestComandosDeLeituraFuncionamComPostgres(), TestRetencaoFuncionaComPostgres(), workspacePostgres(), executar(), gravarColetasDeCatalogo(), gravarTelemetriaDeBanco(), sessaoMCP() (+9 more)

### Community 52 - "Render"
Cohesion: 0.70
Nodes (4): Render(), reportDiagnosis(), TestRenderProduzContratoVersionadoEDeterministico(), TestRenderRepresentaBaselineLegadaComoNull()

### Community 53 - "clienteComMock"
Cohesion: 0.48
Nodes (6): clienteComMock(), sqlmock.Sqlmock, TestFetchFalhaQuandoOCatalogoDevolveTipoInesperado(), TestFetchPropagaFalhaDeCadaConsulta(), TestFetchPropagaFalhaNoMeioDaLeitura(), TestFetchRespeitaContextoCancelado()

### Community 54 - "ApplyRetention"
Cohesion: 0.20
Nodes (15): podadorDeCatalogoFake, RetentionRequest, RetentionResult, RetentionRepository, ApplyRetention(), SchemaCatalogPruner, SignalRetentionRemover, pruneSchemaCatalogs() (+7 more)

### Community 55 - "Demo Shop"
Cohesion: 0.14
Nodes (19): Detector retry_storm, Critérios de aceite do MVP, Métricas de sucesso, Sistema de demonstração demo-shop, Primeira demonstração obrigatória, Roadmap de dez marcos, Serviço payment-service, Serviço postgres da demo (+11 more)

### Community 56 - "DetectDatabaseLatencyDelta"
Cohesion: 0.19
Nodes (22): DetectDatabaseLatencyDelta(), exceedsDatabaseLatencyNoise(), bancoComLatência(), TestDatabaseLatencyDeltaAcusaBancoQueDegradouSemFalhar(), TestDatabaseLatencyDeltaExigeDuasJanelas(), TestDatabaseLatencyDeltaIgnoraOperaçõesQueFalharam(), TestDatabaseLatencyDeltaIgnoraOscilaçãoNormal(), TestDatabaseLatencyDeltaMedeAsOperaçõesQueConcluíram() (+14 more)

### Community 57 - "openChangeRepository"
Cohesion: 0.42
Nodes (8): ChangeRepository, NewChangeRepository(), changeSnapshot(), deploymentAt(), openChangeRepository(), TestChangeRepositoryListDeploymentsFiltersOrdersAndLimits(), TestChangeRepositorySaveChangesIsAtomicAndIdempotent(), TestChangeRepositoryValidatesBeforeDatabaseAndRespectsContext()

### Community 59 - "sqlite/diagnosis_repository.go"
Cohesion: 0.29
Nodes (9): findingID(), preparedFinding, DiagnosisRepository, NewDiagnosisRepository(), prepareDiagnosis(), rollbackDiagnosisTransaction(), subjectIdentifier(), preparedDiagnosis (+1 more)

### Community 60 - "Diff"
Cohesion: 0.32
Nodes (19): CheckCollection(), Diff(), column(), index(), snapshotWith(), TestCheckCollectionExplicaOQueVerificar(), TestCheckCollectionNaoAtrapalhaRemocaoParcial(), TestCheckCollectionRecusaClasseInteiraQueSumiu() (+11 more)

### Community 62 - "InitializeProject"
Cohesion: 0.27
Nodes (11): ensureContext(), EphemeralProjectDir(), InitializeProject(), assertDirectoryExists(), assertFileExists(), TestEphemeralProjectDirCriaForaDoProjeto(), TestEphemeralProjectDirNaoColide(), TestEphemeralProjectDirServeAoInitCompleto() (+3 more)

### Community 64 - "Environment"
Cohesion: 0.06
Nodes (46): config, loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCheckout(), TestLoadConfigRejeitaRetryIlimitado(), loadConfig() (+38 more)

### Community 65 - "Deployment"
Cohesion: 0.15
Nodes (13): scopedDeploymentReaderFake, Deployment, preparedCommit, preparedDeployment, NewChangeRepository(), prepareChanges(), rollbackChangeTransaction(), scanDeployments() (+5 more)

### Community 66 - "ParseOTLPLogsJSON"
Cohesion: 0.27
Nodes (12): logRecordID(), logSeverity(), normalizeLogRecord(), ParseOTLPLogsJSON(), TestParseOTLPLogsNuncaArmazenaOTextoDaMensagem(), TestParseOTLPLogsPreservaSeveridadeECorrelação(), TestParseOTLPLogsRejeitaEnvelopeInválido(), TestParseOTLPLogsÉIdempotentePorRegistro() (+4 more)

### Community 67 - "RenderPersistedDiagnosis"
Cohesion: 0.43
Nodes (5): formatIncidentTime(), RenderIncidentList(), RenderPersistedDiagnosis(), TestRenderIncidentListOrdenaEExibeResumo(), TestRenderPersistedDiagnosisExplicaMetadataLegada()

### Community 68 - "exigirSinaisCoerentes"
Cohesion: 0.67
Nodes (6): testing.F, exigirErroClassificado(), exigirSinaisCoerentes(), FuzzParseOTLPLogsJSONNãoDevolveOCorpo(), FuzzParseOTLPTracesJSON(), FuzzParseOTLPTracesProtobuf()

### Community 69 - "Ingestão OTLP"
Cohesion: 0.18
Nodes (11): Módulo telemetry, Allowlist de atributos de Resource, Ingestão OTLP, Receiver OTLP HTTP POST /v1/traces, Signal, Detector deployment_proximity, Comando faultmap serve, Receiver OTLP sem autenticação nem TLS (+3 more)

### Community 70 - "Grafo de evidências"
Cohesion: 0.40
Nodes (5): Módulo evidence, EvidenceEdge, EvidenceNode, Fallback de parentesco de spans, Grafo de evidências

### Community 71 - "Write"
Cohesion: 0.37
Nodes (11): graph(), snapshot(), TestArtefatosConcordamSobreAOrdemDosSuspeitos(), TestWriteExigeDiretorioInformado(), TestWriteFalhaQuandoCaminhoNaoEDiretorio(), TestWriteFalhaQuandoDiretorioNaoExiste(), TestWriteGravaOsCincoArtefatosNoDiretorio(), TestWriteProduzBytesIdenticosEmDuasExecucoes() (+3 more)

### Community 72 - "rodar-ranking.sh"
Cohesion: 0.83
Nodes (3): limpar(), rodar-ranking.sh script, trafego()

### Community 86 - "Snapshot"
Cohesion: 0.17
Nodes (14): ChangeSource, changeSourceFake, changeWriterFake, Commit, ImportRequest, Snapshot, ChangeImportResult, ChangeWriter (+6 more)

### Community 87 - "PersistedDiagnosis"
Cohesion: 0.21
Nodes (9): incidentHistoryReaderFake, IncidentSummary, PersistedDiagnosis, ListIncidents(), TestGetIncidentValidaIDEPropagaAusencia(), TestListIncidentsPreservaOrdemDoRepositorio(), TestListIncidentsValidaLimiteAntesDoRepositorio(), TestPersistedDiagnosisMetadataComplete() (+1 more)

### Community 89 - "Piloto cego"
Cohesion: 0.38
Nodes (7): Collector do piloto, Exportadores separados para traces e logs, Papéis de operador e investigador, Piloto cego, Sequência recomendada de incidentes, Registro do investigador, Resultado do piloto cego

### Community 90 - "postgres/signal_repository.go"
Cohesion: 0.22
Nodes (9): SignalType, database/sql.Rows, marshalFloatMap(), marshalStringMap(), NewSignalRepository(), rollbackSignalTransaction(), scanSignal(), scanSignals() (+1 more)

### Community 91 - "repositorioComRetencao"
Cohesion: 0.42
Nodes (9): coletaDe(), SchemaRepository, repositorioComRetencao(), TestColetaContraLinhaDeBaseEsvaziadaFalhaEmVezDeInventarMigracao(), TestPruneAvancaEmLotes(), TestPruneEhIdempotente(), TestPruneLiberaOCatalogoAntigoSemApagarAsMudancas(), TestPrunePreservaAColetaMaisRecenteDeCadaBase() (+1 more)

### Community 92 - "deploy-inofensivo"
Cohesion: 0.50
Nodes (3): deploy-inofensivo, O que ele verifica, Por que ele existe

### Community 94 - "coletar"
Cohesion: 0.37
Nodes (12): coletar(), esperarCatalogo(), sqlmock.Sqlmock, linhasDeColuna(), linhasVazias(), TestFetchDevolveObjetosEmOrdemEstavel(), TestFetchIgnoraConstraintsInternasDeNotNull(), TestFetchNaoGuardaOTextoDoDefault() (+4 more)

### Community 95 - "ADR 0015 — A retenção libera o catálogo e preserva as mudanças"
Cohesion: 0.40
Nodes (4): ADR 0015 — A retenção libera o catálogo e preserva as mudanças, Consequências, Contexto, Decisão

### Community 97 - "postgres-demo"
Cohesion: 0.40
Nodes (4): DATABASE_URI, uvx, postgres-demo, postgres-mcp

### Community 99 - "ExplainSuspect"
Cohesion: 0.07
Nodes (50): SuspectContribution, Frases de causas comuns, Lacuna: detector de atraso de consumidor, Catálogo de falhas do OpenTelemetry Demo, strings.Builder, ExplainSuspect(), findingsForService(), findSuspect() (+42 more)

### Community 100 - "Sorteio cego na demo-shop"
Cohesion: 0.12
Nodes (15): ADR 0017 — A cauda do banco é uma pergunta própria, não outro percentil, Contexto, Decisão, O que se aceita em troca, Por que não mudar o percentil, Como repetir, O achado sobre o produto, O mecanismo existe, mas não foi confirmado como a causa (+7 more)

### Community 101 - "RenderRanking"
Cohesion: 0.38
Nodes (8): RenderRanking(), rankingSnapshot(), TestRankingPreservaAOrdemDoSnapshot(), TestRenderRankingNormalizaInstanteParaUTC(), TestRenderRankingPreservaOrdemDoSnapshotEOrdenaContribuicoesPorRegra(), TestRenderRankingProduzBytesIdenticosEmDuasExecucoes(), TestRenderRankingSemSuspeitosEscreveListaVaziaENaoNula(), rankingDocument

### Community 102 - "Retenção apaga telemetria e preserva snapshots"
Cohesion: 0.47
Nodes (3): Retenção apaga telemetria e preserva snapshots, RetentionRepository, rollbackRetentionTransaction()

### Community 104 - "RenderIncidentSummary"
Cohesion: 0.47
Nodes (8): RenderIncidentSummary(), summarySnapshot(), TestRenderIncidentSummaryNaoDuplicaRelatorioCompleto(), TestRenderIncidentSummaryProduzBytesIdenticosEmDuasExecucoes(), TestRenderIncidentSummaryResumeIdentificacaoJanelasEContagens(), TestRenderIncidentSummarySemMetadadosCompletosOmiteBaseline(), TestRenderIncidentSummarySemSuspeitosOmiteSuspeitoPrincipal(), summaryDocument

### Community 106 - "testSignal"
Cohesion: 0.42
Nodes (8): openSignalRepository(), testSignal(), TestSignalRepositoryListByServiceAndWindowUsesStableOrderAndLimit(), TestSignalRepositoryListByTraceIDFiltersOrdersAndLimits(), TestSignalRepositoryListByTraceIDRejectsInvalidInputBeforeDatabaseAccess(), TestSignalRepositoryListByTraceIDRespectsCanceledContext(), TestSignalRepositorySaveInsertsAndDeduplicatesByID(), SignalRepository

### Community 107 - "Migrate"
Cohesion: 0.39
Nodes (6): applyMigration(), TestMigrateIntegracaoSuportaProcessosSimultaneos(), isMigrationApplied(), Migrate(), rollbackMigration(), migration

### Community 108 - "TestConformidadeIntegracaoPostgres"
Cohesion: 0.19
Nodes (8): abrirEmSchemaIsolado(), comSearchPath(), TestConformidadeIntegracaoPostgres(), closeAfterFailure(), Open(), NewRetentionRepository(), rollbackRetentionTransaction(), RetentionRepository

### Community 109 - "sqlite/change_repository.go"
Cohesion: 0.19
Nodes (8): preparedCommit, preparedDeployment, prepareChanges(), rollbackChangeTransaction(), ChangeRepository, deploymentMetadata, preparedChanges, preparedDeployment

### Community 110 - "catalog_integration_test.go"
Cohesion: 0.50
Nodes (11): coletarDoServidor(), conectarComoUsuarioCego(), conectarPostgres(), objetosDoSchema(), prepararSchema(), somenteDoSchema(), TestIntegracaoConsultasSaoPostgreSQLValido(), TestIntegracaoDetectaMigracaoReal() (+3 more)

### Community 111 - "NewClient"
Cohesion: 0.29
Nodes (7): DigestExpression(), TestDigestExpressionNaoEhReversivelNemVazaTamanho(), Client, isImplicitNotNullConstraint(), NewClient(), qualify(), TestNewClientExigeBaseNomeada()

### Community 112 - "ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada"
Cohesion: 0.33
Nodes (5): Adendo — a divergência de concorrência passou a ser coberta, ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada, Consequências, Contexto, Decisão

### Community 113 - "RenderDiagnosis"
Cohesion: 0.36
Nodes (9): RenderDiagnosis(), renderDiagnosisForTest(), TestRenderDiagnosisApresentaRegrasDeFormaHumanaEAuditavel(), TestRenderDiagnosisApresentaRetryStormEmPortugues(), TestRenderDiagnosisConsolidaLimitacoesRepetidas(), TestRenderDiagnosisDistingueCommitDeServiço(), TestRenderDiagnosisDizOQueAquelePadrãoCostumaSignificar(), TestRenderDiagnosisMantemLimitacaoEspecificaNaHipotese() (+1 more)

### Community 114 - "scanSignal"
Cohesion: 0.33
Nodes (5): marshalFloatMap(), marshalStringMap(), rollbackSignalTransaction(), scanSignal(), SignalRepository

### Community 115 - "A coleta de schema guarda identificadores, não expressões"
Cohesion: 0.17
Nodes (23): Changelog do Faultmap, Mudança de schema como sinal de incidente, Piloto cego, Servidor MCP somente leitura, Teto relativo para evidência de apoio, Antes de mudar código, consulte o grafo, PrivacyConfig, Detectores estruturais reutilizam o peso graph_proximity (+15 more)

### Community 117 - "sortear-incidente.sh"
Cohesion: 0.42
Nodes (7): base(), carga(), diagnosticar(), falha(), revelar(), sortear-incidente.sh script, sortear()

### Community 118 - "Fixtures OpenTelemetry"
Cohesion: 0.18
Nodes (11): Atributos OpenTelemetry prioritários, InvestigationWindow, Janelas de investigação (baseline e incidente), Metadados legados anuláveis, Testes obrigatórios, Estratégia expand-and-contract, Baseline mínima de 30 sinais por serviço, StriderEdge (aplicação FastAPI + DuckDB de terceiros) (+3 more)

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
- **76 isolated node(s):** `uvx`, `postgres-mcp`, `DATABASE_URI`, `Request`, `results` (+71 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **15 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

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
- **Why does `Signal` connect `Signal` to `Rank`, `graph.go`, `attributes.go`, `NewSignalRepository`, `DetectSchemaChangeProximity`, `Finding`, `DetectTraceBreak`, `detectors_test.go`, `time.Time`, `diagnose_scope.go`, `conformidade.go`, `otlp_json.go`, `DiagnoseIncidentInScope`, `Input`, `DetectRetryStorm`, `DetectVersionRegression`, `ParseOTLPJSON`, `context.Context`, `DetectLogCorrelation`, `DetectDatabaseLatencyDelta`, `ParseOTLPLogsJSON`, `exigirSinaisCoerentes`, `postgres/signal_repository.go`, `testSignal`, `scanSignal`?**
  _High betweenness centrality (0.101) - this node is a cross-community bridge._
- **Why does `Finding` connect `Finding` to `Rank`, `DetectSchemaChangeProximity`, `DetectTraceBreak`, `detectors_test.go`, `time.Time`, `openDiagnosisRepository`, `diagnose_scope.go`, `encoding/json.RawMessage`, `conformidade.go`, `database/sql.Tx`, `Input`, `Signal`, `postgres/diagnosis_repository.go`, `DetectRetryStorm`, `DetectVersionRegression`, `json/report.go`, `timeline.go`, `DetectLogCorrelation`, `DetectDatabaseLatencyDelta`, `sqlite/diagnosis_repository.go`, `PersistedDiagnosis`, `ExplainSuspect`, `RenderDiagnosis`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Why does `Load()` connect `Load` to `context.Context`?**
  _High betweenness centrality (0.032) - this node is a cross-community bridge._