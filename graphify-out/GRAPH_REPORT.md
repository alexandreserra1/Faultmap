# Graph Report - Faultmap  (2026-09-13)

## Corpus Check
- 252 files · ~320,273 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1979 nodes · 5720 edges · 115 communities (102 shown, 13 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 764 edges (avg confidence: 0.82)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `60642fe0`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- SchemaSnapshot
- Load
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
- detectors.go
- client.go
- newRootCommand
- DetectTraceBreak
- Integração GitHub
- detectors_test.go
- Faultmap MVP — Arquitetura
- time.Time
- testing.T
- DiagnoseIncidentInScope
- GetIncident
- conformidade.go
- otlp_json.go
- NewInvestigationWindowFromIncident
- database/sql.Tx
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
- Write
- NewTimeWindow
- database/sql.DB
- ParseOTLPJSON
- run
- faultmap_app.py
- io.Writer
- IngestTelemetry
- acceptance_test.go
- DetectLogCorrelation
- schema_mcp_integration_test.go
- run
- clienteComMock
- ApplyRetention
- Demo Shop
- DetectDatabaseLatencyDelta
- openChangeRepository
- run
- SchemaChange
- Environment
- Setup
- InitializeProject
- filterDatabaseSignals
- Diff
- Deployment
- ParseOTLPLogsJSON
- ListSignals
- exigirSinaisCoerentes
- Ingestão OTLP
- Grafo de evidências
- context.Context
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
- RenderDiagnosis
- Piloto cego
- postgres/signal_repository.go
- repositorioComRetencao
- ParseOTLPTraces
- Fixtures OpenTelemetry
- coletar
- ADR 0015 — A retenção libera o catálogo e preserva as mudanças
- TestConformidadeIntegracaoPostgres
- postgres-demo
- Finding
- NewHandler
- NewClient
- catalog_integration_test.go
- IngestChanges
- net/http.Handler
- scanSignal
- testSignal
- NewClient
- RetentionRepository
- .SaveSnapshot
- ScopeRepository
- snapshotDeExemplo
- ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada
- Render
- Open

## God Nodes (most connected - your core abstractions)
1. `Signal` - 140 edges
2. `Finding` - 57 edges
3. `newRootCommand()` - 42 edges
4. `Rank()` - 42 edges
5. `DiagnoseIncidentInScope()` - 41 edges
6. `DetectSchemaChangeProximity()` - 37 edges
7. `Load()` - 34 edges
8. `PersistedDiagnosis` - 32 edges
9. `NewInvestigationWindowFromIncident()` - 29 edges
10. `Open()` - 29 edges

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

## Communities (115 total, 13 thin omitted)

### Community 0 - "SchemaSnapshot"
Cohesion: 0.15
Nodes (18): escritorDeSchemaFake, fonteDeSchemaFake, SchemaSource, SchemaSnapshot, SchemaImportResult, SchemaWriter, IngestSchema(), coletaComDoisObjetos() (+10 more)

### Community 1 - "Load"
Cohesion: 0.06
Nodes (88): Conexao, RepositorioDeCatalogo, RepositorioDeDiagnostico, RepositorioDeMudancas, RepositorioDeSinais, diagnosisEnd(), githubImportEnd(), newBlameCommand() (+80 more)

### Community 2 - "Rank"
Cohesion: 0.05
Nodes (70): Consultar o grafo antes de mudar, Grafo de conhecimento em graphify-out, graphify affected — travessia reversa, Limites do grafo, Changelog do Faultmap, Mudança de schema como sinal de incidente, Piloto cego, Servidor MCP somente leitura (+62 more)

### Community 3 - "Suspect"
Cohesion: 0.09
Nodes (35): Frases de causas comuns, strings.Builder, CommonCauses(), TestCausasComunsCobremCatálogoDeTerceiro(), TestCausasComunsIgnoraRegraDesconhecida(), TestCausasComunsOferecemAlternativas(), TestTodaRegraDizOQueAquelePadrãoCostumaSignificar(), SubjectKind (+27 more)

### Community 4 - "graph.go"
Cohesion: 0.09
Nodes (46): TraceInvestigation, edgeAccumulator, nodeAccumulator, NodeKind, BlameTrace(), TraceSignalReader, hasRelation(), TestBlameTraceCarregaUmaVezEConstroiGrafo() (+38 more)

### Community 5 - "IngestFunc"
Cohesion: 0.12
Nodes (34): mapOTLPEncoding(), net/http.ResponseWriter, appendUvarint(), decodePayload(), fallbackContentType(), Encoding, marshalGoogleStatusProtobuf(), NewHandler() (+26 more)

### Community 6 - "historicoComUmIncidente"
Cohesion: 0.16
Nodes (25): TestAuditoriaNaoEscreveNaSaidaDoProtocolo(), TestAuditoriaNaoRegistraOsArgumentos(), TestArgumentosComTipoErradoViramErroDeTool(), TestContextoCanceladoEncerraASessao(), TestErroDoRepositorioViraErroDeToolENaoDerrubaOServidor(), TestIncidenteInexistenteExplicaOQueAconteceu(), TestLinhaGiganteNaoDerrubaASessao(), TestLoteJSONRPCRecebeRespostaEmVezDeSilencio() (+17 more)

### Community 7 - "attributes.go"
Cohesion: 0.08
Nodes (41): databaseTargetsInWindow(), sortedKeys(), safeRetryIdentity(), databaseTargetsQueriedBy(), signalsForChange(), databaseDetail(), displayDatabaseSystem(), displaySeverity() (+33 more)

### Community 8 - "payment/handler_test.go"
Cohesion: 0.09
Nodes (33): chronicFailure(), NewHandler(), executePayment(), mustHandler(), TestHandlerConverteFalhaDoBancoSemVazarDetalhes(), TestHandlerErroCrônicoÉDeterminísticoPorPedido(), TestHandlerForcaStatusSemPersistir(), TestHandlerPersistePagamento() (+25 more)

### Community 9 - "NewSignalRepository"
Cohesion: 0.20
Nodes (20): Retenção apaga telemetria e preserva snapshots, TestConformidadeSQLite(), NewDiagnosisRepository(), RetentionRepository, NewRetentionRepository(), openRetentionDatabase(), signalIDs(), TestRetentionRepositoryPreservaSnapshotsDeIncidentes() (+12 more)

### Community 10 - "NewHandler"
Cohesion: 0.10
Nodes (24): Config, Handler, Request, roundTripperFunc, NewHandler(), TestHandlerEncaminhaPagamentoComSucesso(), TestHandlerFanOutFalhaQuandoUmaChamadaFalha(), TestHandlerFanOutFazChamadasParalelasBemSucedidas() (+16 more)

### Community 11 - "DetectSchemaChangeProximity"
Cohesion: 0.12
Nodes (37): Workflow de CI, Matriz E2E fora do gate automático, Binários reproduzíveis com checksums, Workflow de Release, Cenário migracao-inofensiva, Matriz E2E, Modo difícil, Pontuação pela ponta pessimista do intervalo (+29 more)

### Community 12 - "detectors.go"
Cohesion: 0.15
Nodes (31): Evidence, Input, Detectores aceitam as duas convenções HTTP e ignoram spans internos, Telemetria de instrumentação real como base de teste, Detector database_http_trace_correlation, Detector database_timeout, Detector latency_delta, clamp() (+23 more)

### Community 13 - "client.go"
Cohesion: 0.26
Nodes (10): Ingestão do GitHub limitada a uma página, sem N+1, FetchRequest, Client, commitResponse, deploymentResponse, net/url.URL, isLoopbackHost(), repositoryPath() (+2 more)

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
Cohesion: 0.11
Nodes (13): diagnosisReaderFake, leitorDeSchemaQueFalha, retentionRemoverStub, scopedSignalReaderFake, scopeReaderFake, scopeReaderNiveis, signalReaderFake, time.Time (+5 more)

### Community 20 - "testing.T"
Cohesion: 0.14
Nodes (33): DiagnosisRepository, TestConfigValidateExigeIdentidadeCompleta(), TestDisabledMantemShutdownSeguro(), TestTraceEndpointAcrescentaCaminhoOTLP(), testing.T, TestCommitValidateRejectsIncompleteChange(), TestDeploymentValidateRequiresCorrelationFields(), assertIntPointerEqual() (+25 more)

### Community 21 - "DiagnoseIncidentInScope"
Cohesion: 0.21
Nodes (24): ScopedDiagnosisRequest, ScopeDiscovery, A investigação compara serviços descobertos pelos traces, applyDependencyTieBreak(), containsService(), corroboratedSchemaFindings(), deploymentsForService(), DiagnoseIncidentInScope() (+16 more)

### Community 22 - "GetIncident"
Cohesion: 0.25
Nodes (15): GetIncident(), IncidentHistoryReader, decodeArguments(), describeFindings(), marshalIndented(), toolDefinitions(), toolError(), callTool() (+7 more)

### Community 23 - "conformidade.go"
Cohesion: 0.07
Nodes (58): diagnosisStoreFake, DiagnosisID(), Diagnosis, DiagnosisStore, PersistDiagnosis(), TestDiagnosisIDEDeterministicoEmUTC(), TestPersistDiagnosisNaoSalvaIncidenteSemSinais(), TestPersistDiagnosisPreservaCausaDoStore() (+50 more)

### Community 24 - "otlp_json.go"
Cohesion: 0.23
Nodes (20): encoding/json.RawMessage, exceptionAttributes(), normalizeAttributes(), normalizeExportRequest(), normalizeSpan(), parseUnixNano(), resourceSignalAttributes(), scalarJSONValue() (+12 more)

### Community 25 - "NewInvestigationWindowFromIncident"
Cohesion: 0.26
Nodes (20): cadeiaSignal(), escopoSpanDeBanco(), sinalComVersao(), TestDiagnoseScopeAcusaOCommitImplantado(), TestDiagnoseScopeAcusaSchemaQuandoHaSintoma(), TestDiagnoseScopeAlcançaSegundoSalto(), TestDiagnoseScopeConsultaDeploymentsEmLote(), TestDiagnoseScopeCorrelacionaMudancaDeSchema() (+12 more)

### Community 26 - "database/sql.Tx"
Cohesion: 0.15
Nodes (17): database/sql.NullInt64, database/sql.NullTime, database/sql.Tx, DiagnosisRepository, nullIntPointer(), nullTimePointer(), readPersistedFindings(), readPersistedIncident() (+9 more)

### Community 27 - "detectDeploymentProximity"
Cohesion: 0.17
Nodes (18): deploymentCandidate, Configuração E2E com GitHub, Mock GitHub no loopback do container, Matriz E2E automatizada, Override timeout-after-deploy (SHA como SERVICE_VERSION), commitLabel(), commitSHAFromEvidence(), deploymentSummary() (+10 more)

### Community 28 - "run-e2e.sh"
Cohesion: 0.16
Nodes (12): activate_scenario(), assert_contains(), assert_json(), cleanup(), compose(), diagnose(), first_trace_id(), run_scenario() (+4 more)

### Community 29 - "DetectDatabaseError"
Cohesion: 0.32
Nodes (12): Detector database_error, DetectDatabaseError(), bancoComFalhas(), databaseErrorSignals(), TestDatabaseErrorAcusaFalhaVerdadeira(), TestDatabaseErrorDetectaCrescimentoDeFalhasDeConexão(), TestDatabaseErrorIgnoraCancelamentoDeInstrumentaçãoReal(), TestDatabaseErrorIgnoraCancelamentoDoCliente() (+4 more)

### Community 30 - "time.Duration"
Cohesion: 0.33
Nodes (9): newHTTPServer(), parseServerTimeouts(), rejectionReporter(), serveHTTP(), shutdownHTTPServers(), serverTimeouts, net/http.Server, time.Duration (+1 more)

### Community 31 - "DetectRetryStorm"
Cohesion: 0.19
Nodes (18): retryCandidate, retryOperationStats, Lacuna: detector de atraso de consumidor, Catálogo de falhas do OpenTelemetry Demo, DetectRetryStorm(), retryStats(), bancoNodeRepetido(), databaseRetrySignals() (+10 more)

### Community 32 - "openSchemaRepository"
Cohesion: 0.25
Nodes (17): TestListSchemaChangesLimitaAConsulta(), TestSaveSnapshotAceitaBaseVaziaDesdeOInicio(), TestSaveSnapshotFalhaComBancoFechado(), TestSaveSnapshotFalhaComColetaAnteriorCorrompida(), TestSaveSnapshotRecusaColetaGigante(), TestSaveSnapshotRecusaColetaVaziaContraCatalogoPovoado(), TestSaveSnapshotRejeitaColetaSemIdentidade(), TestSaveSnapshotRespeitaContextoCancelado() (+9 more)

### Community 33 - "Signal"
Cohesion: 0.19
Nodes (16): signalStoreFake, traceReaderFake, SignalType, math/rand.Rand, attributeValueOrEmpty(), databaseFailureTypeSuffix(), databaseNonTimeoutFailures(), isClientCancellation() (+8 more)

### Community 34 - "Faultmap"
Cohesion: 0.15
Nodes (13): Backend determinístico, CLI-first e local-first, Explicabilidade, Faultmap, Infraestrutura existente e evolução controlada, Interface Detector, Módulo detection, Monólito modular em Go (+5 more)

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
Cohesion: 0.14
Nodes (15): Módulo reporting, Módulo storage, Persistência SQLite, Snapshot imutável de diagnóstico, Artefatos gerados em faultmap-out, Comando faultmap blame trace, CLI faultmap, Comando faultmap diagnose incident (+7 more)

### Community 39 - "run"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConvertePoolPostgres(), TestLoadConfigRejeitaIdleMaiorQueOpen(), config

### Community 40 - "run-hard-mode.sh"
Cohesion: 0.28
Nodes (14): apply_harmless_migration(), assert_contains(), assert_no_finding(), cleanup(), collect_schema(), compose(), diagnose(), generate_burst_traffic() (+6 more)

### Community 41 - "Write"
Cohesion: 0.10
Nodes (41): graph(), snapshot(), TestArtefatosConcordamSobreAOrdemDosSuspeitos(), TestWriteExigeDiretorioInformado(), TestWriteFalhaQuandoCaminhoNaoEDiretorio(), TestWriteFalhaQuandoDiretorioNaoExiste(), TestWriteGravaOsCincoArtefatosNoDiretorio(), TestWriteProduzBytesIdenticosEmDuasExecucoes() (+33 more)

### Community 42 - "NewTimeWindow"
Cohesion: 0.26
Nodes (10): InvestigationWindow, TimeWindow, NewInvestigationWindow(), NewTimeWindow(), TestNewInvestigationWindowAllowsBaselineToMeetIncidentBoundary(), TestNewInvestigationWindowFromIncidentCalculatesContiguousBaseline(), TestNewInvestigationWindowFromIncidentRejectsInvalidInputs(), TestNewInvestigationWindowRejectsOverlappingOrInvalidWindows() (+2 more)

### Community 43 - "database/sql.DB"
Cohesion: 0.25
Nodes (18): database/sql.DB, applyMigration(), isMigrationApplied(), Migrate(), rollbackMigration(), assertIndexColumns(), assertIndexExists(), assertMigrationVersion() (+10 more)

### Community 44 - "ParseOTLPJSON"
Cohesion: 0.20
Nodes (15): encoding/json.Decoder, go.opentelemetry.io/proto/otlp/common/v1.AnyValue, ParseOTLPJSON(), rejectTrailingJSON(), assertSignalEqual(), intAnyValue(), stringAnyValue(), TestParseOTLPJSONHonorsCancelledContext() (+7 more)

### Community 45 - "run"
Cohesion: 0.23
Nodes (10): NewHTTPServer(), RunHTTPServer(), SignalContext(), TestNewHTTPServerConfiguraLimitesEHealth(), TestRunHTTPServerEncerraComContexto(), loadConfig(), main(), run() (+2 more)

### Community 46 - "faultmap_app.py"
Cohesion: 0.19
Nodes (10): _current_fault(), _fault_file(), _FaultInjector, _install_database_delay(), _parse_fault(), Ponto de entrada que instrumenta o DuckDB antes de carregar a aplicação. Fica…, Atrasa cada consulta quando a falha ativa for db_slow. O patch é no execute da…, Arquivo consultado a cada requisição para saber se a falha está ligada. Ler de… (+2 more)

### Community 47 - "io.Writer"
Cohesion: 0.09
Nodes (34): timeline.json ancora findings na janela do incidente, bufio.Reader, encoding/json.Encoder, io.Writer, newAuditor(), dispatch(), handle(), protocolError() (+26 more)

### Community 48 - "IngestTelemetry"
Cohesion: 0.33
Nodes (10): IngestionResult, contextError(), SignalStore, IngestLogs(), IngestTelemetry(), IngestTelemetryFile(), TestIngestTelemetryFileNormalizaEPersiste(), TestIngestTelemetryPreservaClassificacaoDePayloadInvalido() (+2 more)

### Community 49 - "acceptance_test.go"
Cohesion: 0.42
Nodes (11): readRequiredFile(), requireContains(), TestDemoPublicaPortasSomenteNoLoopback(), TestDemoUsaPortaDeHostDedicada(), TestLoadGeneratorRecebeIdentidadeOTel(), TestReadmesDocumentamExecucaoDaDemo(), TestRunnerE2EDeclaraMatrizLimitesELimpeza(), TestScenariosDocumentamContratoReproduzivel() (+3 more)

### Community 50 - "DetectLogCorrelation"
Cohesion: 0.32
Nodes (11): exceedsSamplingNoise(), DetectLogCorrelation(), errorLogs(), filterLogSignals(), logsDeErro(), requisicoes(), TestLogCorrelationExigeCorrelaçãoComTrace(), TestLogCorrelationIgnoraRuídoConstante() (+3 more)

### Community 51 - "schema_mcp_integration_test.go"
Cohesion: 0.23
Nodes (17): TestBackendPostgresNaoCriaArquivoLocal(), TestComandosDeLeituraFuncionamComPostgres(), TestRetencaoFuncionaComPostgres(), workspacePostgres(), executar(), gravarColetasDeCatalogo(), gravarTelemetriaDeBanco(), sessaoMCP() (+9 more)

### Community 52 - "run"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteGitHubMock(), TestLoadConfigExigeSHA(), config

### Community 53 - "clienteComMock"
Cohesion: 0.48
Nodes (6): clienteComMock(), sqlmock.Sqlmock, TestFetchFalhaQuandoOCatalogoDevolveTipoInesperado(), TestFetchPropagaFalhaDeCadaConsulta(), TestFetchPropagaFalhaNoMeioDaLeitura(), TestFetchRespeitaContextoCancelado()

### Community 54 - "ApplyRetention"
Cohesion: 0.20
Nodes (15): podadorDeCatalogoFake, RetentionRequest, RetentionResult, RetentionRepository, ApplyRetention(), SchemaCatalogPruner, SignalRetentionRemover, pruneSchemaCatalogs() (+7 more)

### Community 55 - "Demo Shop"
Cohesion: 0.10
Nodes (25): Detector retry_storm, Critérios de aceite do MVP, Métricas de sucesso, Sistema de demonstração demo-shop, Primeira demonstração obrigatória, Roadmap de dez marcos, Serviço checkout-service, Serviço load-generator (+17 more)

### Community 56 - "DetectDatabaseLatencyDelta"
Cohesion: 0.38
Nodes (9): DetectDatabaseLatencyDelta(), exceedsDatabaseLatencyNoise(), succeededDatabaseSignals(), bancoComLatência(), TestDatabaseLatencyDeltaAcusaBancoQueDegradouSemFalhar(), TestDatabaseLatencyDeltaExigeDuasJanelas(), TestDatabaseLatencyDeltaIgnoraOperaçõesQueFalharam(), TestDatabaseLatencyDeltaIgnoraOscilaçãoNormal() (+1 more)

### Community 57 - "openChangeRepository"
Cohesion: 0.42
Nodes (8): ChangeRepository, NewChangeRepository(), changeSnapshot(), deploymentAt(), openChangeRepository(), TestChangeRepositoryListDeploymentsFiltersOrdersAndLimits(), TestChangeRepositorySaveChangesIsAtomicAndIdempotent(), TestChangeRepositoryValidatesBeforeDatabaseAndRespectsContext()

### Community 58 - "run"
Cohesion: 0.39
Nodes (7): config, loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCheckout(), TestLoadConfigRejeitaRetryIlimitado()

### Community 59 - "SchemaChange"
Cohesion: 0.20
Nodes (16): scopedSchemaReaderFake, schemaChangeCandidate, SchemaChange, SchemaChangeKind, SchemaObject, SchemaObjectKind, alteredDetail(), countByKind() (+8 more)

### Community 60 - "Environment"
Cohesion: 0.18
Nodes (12): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCargaLimitada(), TestLoadConfigRejeitaConcorrenciaExcessiva(), Environment, Lookup (+4 more)

### Community 61 - "Setup"
Cohesion: 0.80
Nodes (3): Setup(), traceEndpoint(), Config

### Community 62 - "InitializeProject"
Cohesion: 0.27
Nodes (11): ensureContext(), EphemeralProjectDir(), InitializeProject(), assertDirectoryExists(), assertFileExists(), TestEphemeralProjectDirCriaForaDoProjeto(), TestEphemeralProjectDirNaoColide(), TestEphemeralProjectDirServeAoInitCompleto() (+3 more)

### Community 63 - "filterDatabaseSignals"
Cohesion: 0.30
Nodes (11): databaseSystem(), databaseSystems(), filterDatabaseSignals(), carregarFixtureReal(), separarPorFalha(), serviçosDaFixture(), TestDetectorDeBancoDisparaComTelemetriaReal(), TestDetectoresNovosNãoAcusamTelemetriaRealSaudável() (+3 more)

### Community 64 - "Diff"
Cohesion: 0.32
Nodes (19): CheckCollection(), Diff(), column(), index(), snapshotWith(), TestCheckCollectionExplicaOQueVerificar(), TestCheckCollectionNaoAtrapalhaRemocaoParcial(), TestCheckCollectionRecusaClasseInteiraQueSumiu() (+11 more)

### Community 65 - "Deployment"
Cohesion: 0.16
Nodes (12): scopedDeploymentReaderFake, Deployment, preparedCommit, preparedDeployment, NewChangeRepository(), prepareChanges(), rollbackChangeTransaction(), scanDeployments() (+4 more)

### Community 66 - "ParseOTLPLogsJSON"
Cohesion: 0.27
Nodes (12): logRecordID(), logSeverity(), normalizeLogRecord(), ParseOTLPLogsJSON(), TestParseOTLPLogsNuncaArmazenaOTextoDaMensagem(), TestParseOTLPLogsPreservaSeveridadeECorrelação(), TestParseOTLPLogsRejeitaEnvelopeInválido(), TestParseOTLPLogsÉIdempotentePorRegistro() (+4 more)

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

### Community 71 - "context.Context"
Cohesion: 0.18
Nodes (5): context.Context, rollbackRetentionTransaction(), RetentionRepository, ChangeRepository, ScopeRepository

### Community 72 - "rodar-ranking.sh"
Cohesion: 0.83
Nodes (3): limpar(), rodar-ranking.sh script, trafego()

### Community 86 - "Snapshot"
Cohesion: 0.17
Nodes (12): changeSourceFake, Commit, ImportRequest, Snapshot, preparedCommit, preparedDeployment, prepareChanges(), preparedCommit (+4 more)

### Community 87 - "PersistedDiagnosis"
Cohesion: 0.21
Nodes (9): incidentHistoryReaderFake, IncidentSummary, PersistedDiagnosis, ListIncidents(), TestGetIncidentValidaIDEPropagaAusencia(), TestListIncidentsPreservaOrdemDoRepositorio(), TestListIncidentsValidaLimiteAntesDoRepositorio(), TestPersistedDiagnosisMetadataComplete() (+1 more)

### Community 88 - "RenderDiagnosis"
Cohesion: 0.36
Nodes (9): RenderDiagnosis(), renderDiagnosisForTest(), TestRenderDiagnosisApresentaRegrasDeFormaHumanaEAuditavel(), TestRenderDiagnosisApresentaRetryStormEmPortugues(), TestRenderDiagnosisConsolidaLimitacoesRepetidas(), TestRenderDiagnosisDistingueCommitDeServiço(), TestRenderDiagnosisDizOQueAquelePadrãoCostumaSignificar(), TestRenderDiagnosisMantemLimitacaoEspecificaNaHipotese() (+1 more)

### Community 89 - "Piloto cego"
Cohesion: 0.38
Nodes (7): Collector do piloto, Exportadores separados para traces e logs, Papéis de operador e investigador, Piloto cego, Sequência recomendada de incidentes, Registro do investigador, Resultado do piloto cego

### Community 90 - "postgres/signal_repository.go"
Cohesion: 0.22
Nodes (9): database/sql.Rows, placeholders(), marshalFloatMap(), marshalStringMap(), NewSignalRepository(), rollbackSignalTransaction(), scanSignal(), scanSignals() (+1 more)

### Community 91 - "repositorioComRetencao"
Cohesion: 0.42
Nodes (9): coletaDe(), SchemaRepository, repositorioComRetencao(), TestColetaContraLinhaDeBaseEsvaziadaFalhaEmVezDeInventarMigracao(), TestPruneAvancaEmLotes(), TestPruneEhIdempotente(), TestPruneLiberaOCatalogoAntigoSemApagarAsMudancas(), TestPrunePreservaAColetaMaisRecenteDeCadaBase() (+1 more)

### Community 92 - "ParseOTLPTraces"
Cohesion: 0.27
Nodes (10): io.Reader, classifyOTLPError(), OTLPEncoding, contextError(), TestParseOTLPTracesRejeitaCodificacaoDesconhecida(), ParseOTLPLogs(), ParseOTLPTraces(), parseOTLPProtobuf() (+2 more)

### Community 93 - "Fixtures OpenTelemetry"
Cohesion: 0.18
Nodes (11): Atributos OpenTelemetry prioritários, InvestigationWindow, Janelas de investigação (baseline e incidente), Metadados legados anuláveis, Testes obrigatórios, Estratégia expand-and-contract, Baseline mínima de 30 sinais por serviço, StriderEdge (aplicação FastAPI + DuckDB de terceiros) (+3 more)

### Community 94 - "coletar"
Cohesion: 0.37
Nodes (12): coletar(), esperarCatalogo(), sqlmock.Sqlmock, linhasDeColuna(), linhasVazias(), TestFetchDevolveObjetosEmOrdemEstavel(), TestFetchIgnoraConstraintsInternasDeNotNull(), TestFetchNaoGuardaOTextoDoDefault() (+4 more)

### Community 95 - "ADR 0015 — A retenção libera o catálogo e preserva as mudanças"
Cohesion: 0.40
Nodes (4): ADR 0015 — A retenção libera o catálogo e preserva as mudanças, Consequências, Contexto, Decisão

### Community 96 - "TestConformidadeIntegracaoPostgres"
Cohesion: 0.24
Nodes (10): abrirEmSchemaIsolado(), comSearchPath(), TestConformidadeIntegracaoPostgres(), closeAfterFailure(), Open(), applyMigration(), isMigrationApplied(), Migrate() (+2 more)

### Community 97 - "postgres-demo"
Cohesion: 0.40
Nodes (4): DATABASE_URI, uvx, postgres-demo, postgres-mcp

### Community 99 - "Finding"
Cohesion: 0.42
Nodes (11): SuspectContribution, ExplainSuspect(), findingsForService(), findSuspect(), SuspectExplanation, groupContributions(), normalizeFinding(), remainingFindings() (+3 more)

### Community 100 - "NewHandler"
Cohesion: 0.30
Nodes (10): delayFromFile(), NewHandler(), novoProxy(), TestAtrasoDeArquivoLigaEDesligaSemReiniciar(), TestAtrasoIgnoraConteúdoInválido(), TestGatewayPropagaContextoDeTrace(), TestGatewayRepassaAutorização(), TestGatewayRepassaCorpoEStatus() (+2 more)

### Community 101 - "NewClient"
Cohesion: 0.29
Nodes (7): DigestExpression(), TestDigestExpressionNaoEhReversivelNemVazaTamanho(), Client, isImplicitNotNullConstraint(), NewClient(), qualify(), TestNewClientExigeBaseNomeada()

### Community 102 - "catalog_integration_test.go"
Cohesion: 0.50
Nodes (11): coletarDoServidor(), conectarComoUsuarioCego(), conectarPostgres(), objetosDoSchema(), prepararSchema(), somenteDoSchema(), TestIntegracaoConsultasSaoPostgreSQLValido(), TestIntegracaoDetectaMigracaoReal() (+3 more)

### Community 103 - "IngestChanges"
Cohesion: 0.33
Nodes (8): ChangeSource, changeWriterFake, ChangeImportResult, ChangeWriter, IngestChanges(), TestIngestChangesFetchesAndPersistsOneBoundedSnapshot(), TestIngestChangesStopsBeforePersistenceWhenSourceFails(), validChangesRequest()

### Community 104 - "net/http.Handler"
Cohesion: 0.36
Nodes (8): authenticate(), NewHandler(), TestHandlerEntregaCommitEDeploymentCoerentes(), TestHandlerProtegeRotasEValidaConfiguracao(), validateConfig(), writeJSON(), Config, net/http.Handler

### Community 105 - "scanSignal"
Cohesion: 0.33
Nodes (5): marshalFloatMap(), marshalStringMap(), rollbackSignalTransaction(), scanSignal(), SignalRepository

### Community 106 - "testSignal"
Cohesion: 0.42
Nodes (8): openSignalRepository(), testSignal(), TestSignalRepositoryListByServiceAndWindowUsesStableOrderAndLimit(), TestSignalRepositoryListByTraceIDFiltersOrdersAndLimits(), TestSignalRepositoryListByTraceIDRejectsInvalidInputBeforeDatabaseAccess(), TestSignalRepositoryListByTraceIDRespectsCanceledContext(), TestSignalRepositorySaveInsertsAndDeduplicatesByID(), SignalRepository

### Community 107 - "NewClient"
Cohesion: 0.39
Nodes (7): net/url.Values, NewClient(), assertQueryValue(), TestClientFetchImportsBoundedCommitsAndDeployments(), TestClientFetchPreservesCancellationAndDoesNotLeakToken(), TestClientFetchRejectsInvalidInputBeforeNetwork(), TestNewClientRejectsTokenOverInsecureRemoteURL()

### Community 108 - "RetentionRepository"
Cohesion: 0.36
Nodes (3): NewRetentionRepository(), rollbackRetentionTransaction(), RetentionRepository

### Community 109 - ".SaveSnapshot"
Cohesion: 0.39
Nodes (6): NewSchemaRepository(), normalizeDatabaseNames(), readPreviousSnapshot(), rollbackSchemaTransaction(), validateSnapshot(), SchemaRepository

### Community 110 - "ScopeRepository"
Cohesion: 0.43
Nodes (3): NewScopeRepository(), scanServices(), ScopeRepository

### Community 111 - "snapshotDeExemplo"
Cohesion: 0.60
Nodes (5): snapshotDeExemplo(), TestExplicarSuspeitoAgrupaContribuicoesEvidenciasEProveniencia(), TestExplicarSuspeitoExigeNomeNaoVazio(), TestExplicarSuspeitoIgnoraCaixaEEspacos(), TestExplicarSuspeitoInexistenteDevolveErroDeDominio()

### Community 112 - "ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada"
Cohesion: 0.40
Nodes (4): ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada, Consequências, Contexto, Decisão

### Community 113 - "Render"
Cohesion: 0.70
Nodes (4): Render(), reportDiagnosis(), TestRenderProduzContratoVersionadoEDeterministico(), TestRenderRepresentaBaselineLegadaComoNull()

### Community 114 - "Open"
Cohesion: 0.50
Nodes (3): closeAfterFailure(), Open(), TestOpenConfiguresSQLiteForFaultmap()

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
- **61 isolated node(s):** `uvx`, `postgres-mcp`, `DATABASE_URI`, `Request`, `results` (+56 more)
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
- **Why does `Signal` connect `Signal` to `graph.go`, `attributes.go`, `NewSignalRepository`, `DetectSchemaChangeProximity`, `detectors.go`, `DetectTraceBreak`, `detectors_test.go`, `time.Time`, `DiagnoseIncidentInScope`, `conformidade.go`, `otlp_json.go`, `NewInvestigationWindowFromIncident`, `detectDeploymentProximity`, `DetectDatabaseError`, `DetectRetryStorm`, `DetectVersionRegression`, `ParseOTLPJSON`, `IngestTelemetry`, `DetectLogCorrelation`, `DetectDatabaseLatencyDelta`, `filterDatabaseSignals`, `ParseOTLPLogsJSON`, `ListSignals`, `exigirSinaisCoerentes`, `postgres/signal_repository.go`, `ParseOTLPTraces`, `scanSignal`, `testSignal`?**
  _High betweenness centrality (0.097) - this node is a cross-community bridge._
- **Why does `Finding` connect `Finding` to `Rank`, `Suspect`, `DetectSchemaChangeProximity`, `detectors.go`, `DetectTraceBreak`, `detectors_test.go`, `time.Time`, `testing.T`, `DiagnoseIncidentInScope`, `GetIncident`, `conformidade.go`, `database/sql.Tx`, `detectDeploymentProximity`, `DetectDatabaseError`, `DetectRetryStorm`, `DetectVersionRegression`, `Write`, `io.Writer`, `DetectLogCorrelation`, `DetectDatabaseLatencyDelta`, `PersistedDiagnosis`, `RenderDiagnosis`?**
  _High betweenness centrality (0.058) - this node is a cross-community bridge._
- **Why does `DetectSchemaChangeProximity()` connect `DetectSchemaChangeProximity` to `SchemaSnapshot`, `Rank`, `Finding`, `attributes.go`, `detectors.go`, `time.Time`, `DiagnoseIncidentInScope`, `SchemaChange`?**
  _High betweenness centrality (0.029) - this node is a cross-community bridge._