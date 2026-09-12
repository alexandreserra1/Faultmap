# Graph Report - Faultmap  (2026-09-12)

## Corpus Check
- 242 files · ~296,426 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1565 nodes · 4665 edges · 83 communities (75 shown, 8 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 552 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Ingestão de Catálogo
- Comandos da CLI
- Grafo de Evidências do Trace
- Receptor OTLP e Mock GitHub
- Explicação de Suspeito
- Mudanças do GitHub
- Vínculo Banco-Serviço
- Ranking e Pesos
- Propagação entre Serviços
- Retenção de Telemetria
- Bootstrap e Testes da CLI
- Relatório JSON de Incidente
- Serviços da Demo Shop
- Detectores de Banco
- Snapshots de Diagnóstico
- Falhas do Servidor MCP
- Dublês de Leitura
- Suíte dos Detectores
- Grupo 18
- Grupo 19
- Grupo 20
- Grupo 21
- Grupo 22
- Grupo 23
- Grupo 24
- Grupo 25
- Grupo 26
- Grupo 27
- Grupo 28
- Grupo 29
- Grupo 30
- Grupo 31
- Grupo 32
- Grupo 33
- Grupo 34
- Grupo 35
- Grupo 36
- Grupo 37
- Grupo 38
- Grupo 39
- Grupo 40
- Grupo 41
- Grupo 42
- Grupo 43
- Grupo 44
- Grupo 45
- Grupo 46
- Grupo 47
- Grupo 48
- Grupo 49
- Grupo 50
- Grupo 51
- Grupo 52
- Grupo 53
- Grupo 54
- Grupo 55
- Grupo 56
- Grupo 57
- Grupo 58
- Grupo 59
- Grupo 60
- Grupo 61
- Grupo 62
- Grupo 63
- Grupo 64
- Grupo 65
- Grupo 66
- Grupo 67
- Grupo 68
- Grupo 69
- Grupo 70
- Grupo 71
- Grupo 72
- Grupo 73
- Grupo 74
- Grupo 75
- Grupo 76
- Grupo 77
- Grupo 78
- Grupo 79
- Grupo 80
- Grupo 81
- Grupo 82

## God Nodes (most connected - your core abstractions)
1. `Signal` - 131 edges
2. `Finding` - 54 edges
3. `newRootCommand()` - 42 edges
4. `DiagnoseIncidentInScope()` - 39 edges
5. `Migrate()` - 34 edges
6. `Load()` - 32 edges
7. `Rank()` - 32 edges
8. `Open()` - 31 edges
9. `PersistedDiagnosis` - 30 edges
10. `DetectSchemaChangeProximity()` - 28 edges

## Surprising Connections (you probably didn't know these)
- `newRetentionApplyCommand()` --calls--> `ApplyRetention()`  [EXTRACTED]
  cmd/faultmap/root.go → internal/application/apply_retention.go
- `newRetentionApplyCommand()` --calls--> `NewRetentionRepository()`  [EXTRACTED]
  cmd/faultmap/root.go → internal/storage/sqlite/retention_repository.go
- `newIncidentListCommand()` --calls--> `ListIncidents()`  [EXTRACTED]
  cmd/faultmap/root.go → internal/application/incident_history.go
- `newIncidentListCommand()` --calls--> `RenderIncidentList()`  [EXTRACTED]
  cmd/faultmap/root.go → internal/reporting/terminal/incident_renderer.go
- `newIncidentShowCommand()` --calls--> `GetIncident()`  [EXTRACTED]
  cmd/faultmap/root.go → internal/application/incident_history.go

## Import Cycles
- None detected.

## Communities (83 total, 8 thin omitted)

### Community 0 - "Ingestão de Catálogo"
Cohesion: 0.06
Nodes (68): escritorDeSchemaFake, fonteDeSchemaFake, SchemaSource, SchemaWriter, scopedSchemaReaderFake, schemaChangeCandidate, SchemaChange, SchemaChangeKind (+60 more)

### Community 1 - "Comandos da CLI"
Cohesion: 0.08
Nodes (74): diagnosisEnd(), githubImportEnd(), newBlameCommand(), newBlameTraceCommand(), newDiagnoseCommand(), newDiagnoseIncidentCommand(), newExplainCommand(), newExplainSuspectCommand() (+66 more)

### Community 2 - "Grafo de Evidências do Trace"
Cohesion: 0.08
Nodes (56): TraceInvestigation, TraceSignalReader, edgeAccumulator, nodeAccumulator, NodeKind, BlameTrace(), hasRelation(), TestBlameTraceCarregaUmaVezEConstroiGrafo() (+48 more)

### Community 3 - "Receptor OTLP e Mock GitHub"
Cohesion: 0.07
Nodes (52): mapOTLPEncoding(), authenticate(), NewHandler(), TestHandlerEntregaCommitEDeploymentCoerentes(), TestHandlerProtegeRotasEValidaConfiguracao(), validateConfig(), writeJSON(), delayFromFile() (+44 more)

### Community 4 - "Explicação de Suspeito"
Cohesion: 0.08
Nodes (44): SuspectContribution, strings.Builder, applyDependencyTieBreak(), ExplainSuspect(), findingsForService(), findSuspect(), SuspectExplanation, groupContributions() (+36 more)

### Community 5 - "Mudanças do GitHub"
Cohesion: 0.08
Nodes (34): ChangeSource, changeSourceFake, ChangeWriter, changeWriterFake, Commit, ImportRequest, Snapshot, FetchRequest (+26 more)

### Community 6 - "Vínculo Banco-Serviço"
Cohesion: 0.09
Nodes (40): databaseTargetsInWindow(), safeRetryIdentity(), databaseTargetsQueriedBy(), signalsForChange(), isHTTPSignal(), databaseDetail(), displayDatabaseSystem(), displaySeverity() (+32 more)

### Community 7 - "Ranking e Pesos"
Cohesion: 0.09
Nodes (39): math/rand.Rand, clamp(), contributionReason(), Config, ScoreContribution, Weights, Rank(), sortedSet() (+31 more)

### Community 8 - "Propagação entre Serviços"
Cohesion: 0.11
Nodes (34): signalStoreFake, traceReaderFake, propagationSummary, serviceLink, SignalType, DetectDependencyFailure(), failingAncestorServices(), indexSpansByID() (+26 more)

### Community 9 - "Retenção de Telemetria"
Cohesion: 0.11
Nodes (31): database/sql.Rows, NewRetentionRepository(), rollbackRetentionTransaction(), openRetentionDatabase(), signalIDs(), TestRetentionRepositoryPreservaSnapshotsDeIncidentes(), TestRetentionRepositoryRejeitaLimiteInválido(), TestRetentionRepositoryRemoveSomenteSinaisAnterioresAoCorte() (+23 more)

### Community 10 - "Bootstrap e Testes da CLI"
Cohesion: 0.13
Nodes (35): main(), newRootCommand(), assertTableCount(), preparePersistedIncident(), reserveTCPAddress(), TestBlameTraceCommandExplicaFluxoHTTPPostgreSQL(), TestDiagnoseIncidentCommandDetectaRetryStorm(), TestDiagnoseIncidentCommandExplicaAmostraRepresentativa() (+27 more)

### Community 11 - "Relatório JSON de Incidente"
Cohesion: 0.12
Nodes (32): RenderIncidentSummary(), summarySnapshot(), TestRenderIncidentSummaryNaoDuplicaRelatorioCompleto(), TestRenderIncidentSummaryProduzBytesIdenticosEmDuasExecucoes(), TestRenderIncidentSummaryResumeIdentificacaoJanelasEContagens(), TestRenderIncidentSummarySemMetadadosCompletosOmiteBaseline(), TestRenderIncidentSummarySemSuspeitosOmiteSuspeitoPrincipal(), RenderRanking() (+24 more)

### Community 12 - "Serviços da Demo Shop"
Cohesion: 0.10
Nodes (24): Config, Handler, Request, roundTripperFunc, NewHandler(), TestHandlerEncaminhaPagamentoComSucesso(), TestHandlerFanOutFalhaQuandoUmaChamadaFalha(), TestHandlerFanOutFazChamadasParalelasBemSucedidas() (+16 more)

### Community 13 - "Detectores de Banco"
Cohesion: 0.20
Nodes (28): Evidence, Input, databaseSystem(), databaseSystems(), databaseTimeoutSummary(), DetectDatabaseTimeout(), DetectErrorRateDelta(), DetectLatencyDelta() (+20 more)

### Community 14 - "Snapshots de Diagnóstico"
Cohesion: 0.15
Nodes (27): DiagnosisRepository, assertIntPointerEqual(), assertTimePointerEqual(), testDiagnosisAt(), TestDiagnosisRepositoryGetRespectsCanceledContext(), TestDiagnosisRepositoryGetRestoresCompleteSnapshot(), TestDiagnosisRepositoryGetReturnsTypedNotFound(), TestDiagnosisRepositoryGetSupportsLegacySnapshot() (+19 more)

### Community 15 - "Falhas do Servidor MCP"
Cohesion: 0.16
Nodes (24): TestAuditoriaNaoEscreveNaSaidaDoProtocolo(), TestAuditoriaNaoRegistraOsArgumentos(), TestArgumentosComTipoErradoViramErroDeTool(), TestContextoCanceladoEncerraASessao(), TestErroDoRepositorioViraErroDeToolENaoDerrubaOServidor(), TestIncidenteInexistenteExplicaOQueAconteceu(), TestLinhaGiganteNaoDerrubaASessao(), TestLoteJSONRPCRecebeRespostaEmVezDeSilencio() (+16 more)

### Community 16 - "Dublês de Leitura"
Cohesion: 0.11
Nodes (14): diagnosisReaderFake, leitorDeSchemaQueFalha, retentionRemoverStub, scopedSignalReaderFake, scopeReaderFake, scopeReaderNiveis, signalReaderFake, time.Time (+6 more)

### Community 17 - "Suíte dos Detectores"
Cohesion: 0.18
Nodes (25): Run(), assertFinding(), contains(), databaseTimeoutSignals(), httpSignals(), internalHTTPSendSignals(), legacyHTTPSignals(), TestDatabaseTimeoutIgnoraErroGenerico() (+17 more)

### Community 18 - "Grupo 18"
Cohesion: 0.15
Nodes (18): DiagnosisStore, diagnosisStoreFake, DiagnosisID(), Diagnosis, PersistDiagnosis(), TestDiagnosisIDEDeterministicoEmUTC(), TestPersistDiagnosisNaoSalvaIncidenteSemSinais(), TestPersistDiagnosisPreservaCausaDoStore() (+10 more)

### Community 19 - "Grupo 19"
Cohesion: 0.17
Nodes (19): chronicFailure(), NewHandler(), executePayment(), mustHandler(), TestHandlerConverteFalhaDoBancoSemVazarDetalhes(), TestHandlerErroCrônicoÉDeterminísticoPorPedido(), TestHandlerForcaStatusSemPersistir(), TestHandlerPersistePagamento() (+11 more)

### Community 20 - "Grupo 20"
Cohesion: 0.28
Nodes (21): containsLimitation(), DetectSchemaChangeProximity(), entradaComBanco(), mudancaDeSchema(), spansDeBanco(), spansDeBancoComoAInstrumentacaoReal(), TestDetectSchemaChangeProximityAcusaServicoQueFalaComABase(), TestDetectSchemaChangeProximityDecaiComADistancia() (+13 more)

### Community 21 - "Grupo 21"
Cohesion: 0.23
Nodes (19): exceptionAttributes(), normalizeAttributes(), normalizeExportRequest(), normalizeSpan(), parseUnixNano(), resourceSignalAttributes(), scalarJSONValue(), serviceName() (+11 more)

### Community 22 - "Grupo 22"
Cohesion: 0.19
Nodes (19): ScopedDeploymentReader, ScopedDiagnosisRequest, ScopeDiscovery, ScopedSchemaChangeReader, ScopedSignalReader, ScopeReader, containsService(), corroboratedSchemaFindings() (+11 more)

### Community 23 - "Grupo 23"
Cohesion: 0.32
Nodes (20): DiagnoseIncidentInScope(), escopoSpanDeBanco(), sinalComVersao(), TestDiagnoseScopeAcusaOCommitImplantado(), TestDiagnoseScopeAcusaSchemaQuandoHaSintoma(), TestDiagnoseScopeAlcançaSegundoSalto(), TestDiagnoseScopeConsultaDeploymentsEmLote(), TestDiagnoseScopeCorrelacionaMudancaDeSchema() (+12 more)

### Community 24 - "Grupo 24"
Cohesion: 0.16
Nodes (12): activate_scenario(), assert_contains(), assert_json(), cleanup(), compose(), diagnose(), first_trace_id(), run_scenario() (+4 more)

### Community 25 - "Grupo 25"
Cohesion: 0.21
Nodes (18): attributeValueOrEmpty(), databaseFailureTypeSuffix(), databaseNonTimeoutFailures(), DetectDatabaseError(), isClientCancellation(), bancoComFalhas(), databaseErrorSignals(), TestDatabaseErrorAcusaFalhaVerdadeira() (+10 more)

### Community 26 - "Grupo 26"
Cohesion: 0.18
Nodes (11): scopedDeploymentReaderFake, Deployment, deploymentsForService(), NewChangeRepository(), changeSnapshot(), deploymentAt(), openChangeRepository(), TestChangeRepositoryListDeploymentsFiltersOrdersAndLimits() (+3 more)

### Community 27 - "Grupo 27"
Cohesion: 0.23
Nodes (16): retryCandidate, retryOperationStats, DetectRetryStorm(), retryStats(), bancoNodeRepetido(), databaseRetrySignals(), retrySignals(), serverRetrySignals() (+8 more)

### Community 28 - "Grupo 28"
Cohesion: 0.18
Nodes (14): TestPostgresRepositoryAtrasoOcupaConexaoDoPool(), TestPostgresRepositoryIntegracao(), NewPostgresRepository(), recordDatabaseSpanError(), mustPostgresRepository(), TestNewPostgresRepositoryRejeitaPoolAusente(), TestPostgresRepositoryAtrasoRespeitaContexto(), TestPostgresRepositoryCreateUsaConsultaParametrizada() (+6 more)

### Community 29 - "Grupo 29"
Cohesion: 0.21
Nodes (15): deploymentCandidate, commitLabel(), commitSHAFromEvidence(), deploymentSummary(), detectDeploymentProximity(), DetectDeploymentProximityFindings(), observedVersions(), sameStringSet() (+7 more)

### Community 30 - "Grupo 30"
Cohesion: 0.28
Nodes (14): encoding/json.RawMessage, GetIncident(), IncidentHistoryReader, decodeArguments(), describeFindings(), marshalIndented(), toolError(), callTool() (+6 more)

### Community 31 - "Grupo 31"
Cohesion: 0.25
Nodes (13): GitHubConfig, InvestigationConfig, PrivacyConfig, RankingConfig, RankingWeights, StorageConfig, contextError(), Config (+5 more)

### Community 32 - "Grupo 32"
Cohesion: 0.22
Nodes (15): go.opentelemetry.io/proto/otlp/collector/trace/v1.ExportTraceServiceRequest, go.opentelemetry.io/proto/otlp/common/v1.ArrayValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValueList, go.opentelemetry.io/proto/otlp/trace/v1.Span, go.opentelemetry.io/proto/otlp/trace/v1.Span_Event, marshalProtoValue(), protobufAnyValue() (+7 more)

### Community 33 - "Grupo 33"
Cohesion: 0.24
Nodes (14): collectChangeIDs(), collectSignalIDs(), copyIntPointer(), findingSummary(), newDocument(), Render(), sortedStrings(), completeDiagnosis() (+6 more)

### Community 34 - "Grupo 34"
Cohesion: 0.31
Nodes (13): versionStats, comparableVersionStats(), DetectVersionRegression(), TestVersionRegressionComparaDuasVersõesNaMesmaJanela(), TestVersionRegressionDetectaLatênciaPiorEmUmaVersão(), TestVersionRegressionIgnoraDiferençaDentroDoRuído(), TestVersionRegressionSilenciaComUmaÚnicaVersão(), TestVersionRegressionSilenciaComVolumeInsuficienteNoCanário() (+5 more)

### Community 35 - "Grupo 35"
Cohesion: 0.28
Nodes (14): apply_harmless_migration(), assert_contains(), assert_no_finding(), cleanup(), collect_schema(), compose(), diagnose(), generate_burst_traffic() (+6 more)

### Community 36 - "Grupo 36"
Cohesion: 0.31
Nodes (14): NewClient(), coletar(), esperarCatalogo(), sqlmock.Sqlmock, linhasDeColuna(), linhasVazias(), TestFetchDevolveObjetosEmOrdemEstavel(), TestFetchIgnoraConstraintsInternasDeNotNull() (+6 more)

### Community 37 - "Grupo 37"
Cohesion: 0.21
Nodes (10): rejectionReporter(), io.Writer, newAuditor(), RenderScopeSummary(), formatIncidentTime(), RenderIncidentList(), RenderPersistedDiagnosis(), TestRenderIncidentListOrdenaEExibeResumo() (+2 more)

### Community 38 - "Grupo 38"
Cohesion: 0.26
Nodes (10): InvestigationWindow, TimeWindow, NewInvestigationWindow(), NewTimeWindow(), TestNewInvestigationWindowAllowsBaselineToMeetIncidentBoundary(), TestNewInvestigationWindowFromIncidentCalculatesContiguousBaseline(), TestNewInvestigationWindowFromIncidentRejectsInvalidInputs(), TestNewInvestigationWindowRejectsOverlappingOrInvalidWindows() (+2 more)

### Community 39 - "Grupo 39"
Cohesion: 0.25
Nodes (10): mapLookup(), TestLoadConfigConverteCheckout(), TestLoadConfigRejeitaRetryIlimitado(), mapLookup(), TestLoadConfigConvertePoolPostgres(), TestLoadConfigRejeitaIdleMaiorQueOpen(), Lookup, NewEnvironment() (+2 more)

### Community 40 - "Grupo 40"
Cohesion: 0.29
Nodes (12): bufio.Reader, encoding/json.Encoder, toolDefinitions(), dispatch(), handle(), protocolError(), readLine(), Serve() (+4 more)

### Community 41 - "Grupo 41"
Cohesion: 0.40
Nodes (13): database/sql.DB, assertIndexColumns(), assertIndexExists(), assertMigrationVersion(), assertTableExists(), createVersionOneSchema(), migrationRowCount(), openMigrationTestDatabase() (+5 more)

### Community 42 - "Grupo 42"
Cohesion: 0.24
Nodes (13): go.opentelemetry.io/proto/otlp/common/v1.AnyValue, ParseOTLPJSON(), assertSignalEqual(), intAnyValue(), stringAnyValue(), TestParseOTLPJSONHonorsCancelledContext(), TestParseOTLPJSONNormalizesResourceSpans(), TestParseOTLPJSONPreservaExceçãoESuprimeStacktrace() (+5 more)

### Community 43 - "Grupo 43"
Cohesion: 0.25
Nodes (12): encode(), extract(), New(), TestAlfabetoNaoTemCaracteresAmbiguos(), TestClassificabilidade(), TestClassificabilidadeDistingueMilissegundos(), TestDeterminismoPreservaIdempotencia(), TestInstanteZeroNaoQuebra() (+4 more)

### Community 44 - "Grupo 44"
Cohesion: 0.33
Nodes (10): IngestionResult, SignalStore, contextError(), IngestLogs(), IngestTelemetry(), IngestTelemetryFile(), TestIngestTelemetryFileNormalizaEPersiste(), TestIngestTelemetryPreservaClassificacaoDePayloadInvalido() (+2 more)

### Community 45 - "Grupo 45"
Cohesion: 0.19
Nodes (10): _current_fault(), _fault_file(), _FaultInjector, _install_database_delay(), _parse_fault(), Ponto de entrada que instrumenta o DuckDB antes de carregar a aplicação. Fica…, Atrasa cada consulta quando a falha ativa for db_slow. O patch é no execute da…, Arquivo consultado a cada requisição para saber se a falha está ligada. Ler de… (+2 more)

### Community 46 - "Grupo 46"
Cohesion: 0.27
Nodes (10): io.Reader, classifyOTLPError(), OTLPEncoding, contextError(), TestParseOTLPTracesRejeitaCodificacaoDesconhecida(), ParseOTLPLogs(), ParseOTLPTraces(), parseOTLPProtobuf() (+2 more)

### Community 47 - "Grupo 47"
Cohesion: 0.42
Nodes (11): readRequiredFile(), requireContains(), TestDemoPublicaPortasSomenteNoLoopback(), TestDemoUsaPortaDeHostDedicada(), TestLoadGeneratorRecebeIdentidadeOTel(), TestReadmesDocumentamExecucaoDaDemo(), TestRunnerE2EDeclaraMatrizLimitesELimpeza(), TestScenariosDocumentamContratoReproduzivel() (+3 more)

### Community 48 - "Grupo 48"
Cohesion: 0.23
Nodes (9): NewHTTPServer(), SignalContext(), TestNewHTTPServerConfiguraLimitesEHealth(), TestRunHTTPServerEncerraComContexto(), loadConfig(), main(), run(), context.CancelFunc (+1 more)

### Community 49 - "Grupo 49"
Cohesion: 0.27
Nodes (6): context.Context, applyMigration(), isMigrationApplied(), rollbackMigration(), migration, ScopeRepository

### Community 50 - "Grupo 50"
Cohesion: 0.36
Nodes (10): DetectLogCorrelation(), errorLogs(), filterLogSignals(), logsDeErro(), requisicoes(), TestLogCorrelationExigeCorrelaçãoComTrace(), TestLogCorrelationIgnoraRuídoConstante(), TestLogCorrelationLigaErrosDeLogAoTraceQueFalhou() (+2 more)

### Community 51 - "Grupo 51"
Cohesion: 0.29
Nodes (5): incidentHistoryReaderFake, IncidentSummary, PersistedDiagnosis, DiagnosisRepository, historicoFake

### Community 52 - "Grupo 52"
Cohesion: 0.29
Nodes (5): config, loadConfig(), main(), run(), Environment

### Community 53 - "Grupo 53"
Cohesion: 0.33
Nodes (9): database/sql.NullInt64, database/sql.NullTime, database/sql.Tx, nullIntPointer(), nullTimePointer(), readPersistedFindings(), readPersistedIncident(), readPersistedRanking() (+1 more)

### Community 54 - "Grupo 54"
Cohesion: 0.38
Nodes (9): DetectDatabaseLatencyDelta(), exceedsDatabaseLatencyNoise(), succeededDatabaseSignals(), bancoComLatência(), TestDatabaseLatencyDeltaAcusaBancoQueDegradouSemFalhar(), TestDatabaseLatencyDeltaExigeDuasJanelas(), TestDatabaseLatencyDeltaIgnoraOperaçõesQueFalharam(), TestDatabaseLatencyDeltaIgnoraOscilaçãoNormal() (+1 more)

### Community 55 - "Grupo 55"
Cohesion: 0.33
Nodes (8): RetentionRequest, RetentionResult, SignalRetentionRemover, ApplyRetention(), TestApplyRetentionCalculaCorteEEncerraQuandoNãoHáMaisSinais(), TestApplyRetentionPropagaFalhaSemMascararProgresso(), TestApplyRetentionRespeitaTetoDeLotes(), TestApplyRetentionValidaEntrada()

### Community 56 - "Grupo 56"
Cohesion: 0.44
Nodes (9): executar(), gravarTelemetriaDeBanco(), sessaoMCP(), TestIngestSchemaCommandNaoGravaCredencialNoWorkspace(), TestIngestSchemaCommandValidaEntradaAntesDeAbrirConexao(), TestMCPCommandFalaOProtocoloDePontaAPonta(), TestMCPCommandNaoPoluiStdout(), TestPipelineDeSchemaChegaAoDiagnosticoEAoMCP() (+1 more)

### Community 57 - "Grupo 57"
Cohesion: 0.36
Nodes (9): RenderDiagnosis(), renderDiagnosisForTest(), TestRenderDiagnosisApresentaRegrasDeFormaHumanaEAuditavel(), TestRenderDiagnosisApresentaRetryStormEmPortugues(), TestRenderDiagnosisConsolidaLimitacoesRepetidas(), TestRenderDiagnosisDistingueCommitDeServiço(), TestRenderDiagnosisDizOQueAquelePadrãoCostumaSignificar(), TestRenderDiagnosisMantemLimitacaoEspecificaNaHipotese() (+1 more)

### Community 58 - "Grupo 58"
Cohesion: 0.44
Nodes (8): newHTTPServer(), parseServerTimeouts(), serveHTTP(), shutdownHTTPServers(), RunHTTPServer(), serverTimeouts, net/http.Server, time.Duration

### Community 59 - "Grupo 59"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteGitHubMock(), TestLoadConfigExigeSHA(), config

### Community 60 - "Grupo 60"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCargaLimitada(), TestLoadConfigRejeitaConcorrenciaExcessiva(), config

### Community 61 - "Grupo 61"
Cohesion: 0.36
Nodes (7): ensureContext(), InitializeProject(), assertDirectoryExists(), assertFileExists(), TestInitializeProjectCreatesLocalWorkspace(), TestInitializeProjectDoesNotOverwriteExistingConfiguration(), TestInitializeProjectHonorsCancelledContext()

### Community 62 - "Grupo 62"
Cohesion: 0.42
Nodes (8): carregarFixtureReal(), separarPorFalha(), serviçosDaFixture(), TestDetectorDeBancoDisparaComTelemetriaReal(), TestDetectoresNovosNãoAcusamTelemetriaRealSaudável(), TestFalhaRealDeBancoÉReconhecida(), TestTelemetriaRealDeBancoÉEnxergada(), TestTelemetriaRealHTTPÉEnxergada()

### Community 63 - "Grupo 63"
Cohesion: 0.36
Nodes (7): encoding/json.Decoder, rejectTrailingJSON(), ParseOTLPLogsJSON(), TestParseOTLPLogsNuncaArmazenaOTextoDaMensagem(), TestParseOTLPLogsPreservaSeveridadeECorrelação(), TestParseOTLPLogsRejeitaEnvelopeInválido(), TestParseOTLPLogsÉIdempotentePorRegistro()

### Community 64 - "Grupo 64"
Cohesion: 0.36
Nodes (6): CommonCauses(), TestCausasComunsCobremCatálogoDeTerceiro(), TestCausasComunsIgnoraRegraDesconhecida(), TestCausasComunsOferecemAlternativas(), TestTodaRegraDizOQueAquelePadrãoCostumaSignificar(), TestSchemaChangeProximityTemCausasComuns()

### Community 65 - "Grupo 65"
Cohesion: 0.54
Nodes (7): logRecordID(), logSeverity(), normalizeLogRecord(), exportLogsServiceRequest, logRecord, resourceLog, scopeLog

### Community 66 - "Grupo 66"
Cohesion: 0.43
Nodes (5): SignalReader, ListSignals(), TestListSignalsEncaminhaConsulta(), TestListSignalsRespeitaContextoCancelado(), TestListSignalsValidaEntradaAntesDeConsultar()

### Community 67 - "Grupo 67"
Cohesion: 0.67
Nodes (6): testing.F, exigirErroClassificado(), exigirSinaisCoerentes(), FuzzParseOTLPLogsJSONNãoDevolveOCorpo(), FuzzParseOTLPTracesJSON(), FuzzParseOTLPTracesProtobuf()

### Community 68 - "Grupo 68"
Cohesion: 0.48
Nodes (6): clienteComMock(), sqlmock.Sqlmock, TestFetchFalhaQuandoOCatalogoDevolveTipoInesperado(), TestFetchPropagaFalhaDeCadaConsulta(), TestFetchPropagaFalhaNoMeioDaLeitura(), TestFetchRespeitaContextoCancelado()

### Community 69 - "Grupo 69"
Cohesion: 0.43
Nodes (5): NewPolicy(), TestPolicyIgnoraDiferençaDeCaixaEEspaços(), TestPolicyRemoveAtributosBloqueados(), TestPolicySemLimiteNãoTrunca(), TestPolicyTruncaAtributosLongos()

### Community 70 - "Grupo 70"
Cohesion: 0.40
Nodes (5): ListIncidents(), TestGetIncidentValidaIDEPropagaAusencia(), TestListIncidentsPreservaOrdemDoRepositorio(), TestListIncidentsValidaLimiteAntesDoRepositorio(), TestPersistedDiagnosisMetadataComplete()

### Community 71 - "Grupo 71"
Cohesion: 0.70
Nodes (4): loadConfig(), main(), run(), config

### Community 72 - "Grupo 72"
Cohesion: 0.80
Nodes (3): Setup(), traceEndpoint(), Config

### Community 73 - "Grupo 73"
Cohesion: 0.70
Nodes (4): Render(), reportDiagnosis(), TestRenderProduzContratoVersionadoEDeterministico(), TestRenderRepresentaBaselineLegadaComoNull()

### Community 74 - "Grupo 74"
Cohesion: 0.83
Nodes (3): limpar(), rodar-ranking.sh script, trafego()

## Knowledge Gaps
- **10 isolated node(s):** `Request`, `results`, `generate-traffic.sh script`, `generate-traffic.sh script`, `generate-traffic.sh script` (+5 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **8 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Signal` connect `Propagação entre Serviços` to `Grafo de Evidências do Trace`, `Vínculo Banco-Serviço`, `Ranking e Pesos`, `Retenção de Telemetria`, `Detectores de Banco`, `Dublês de Leitura`, `Suíte dos Detectores`, `Grupo 20`, `Grupo 21`, `Grupo 22`, `Grupo 23`, `Grupo 25`, `Grupo 27`, `Grupo 29`, `Grupo 34`, `Grupo 42`, `Grupo 44`, `Grupo 46`, `Grupo 50`, `Grupo 54`, `Grupo 62`, `Grupo 63`, `Grupo 65`, `Grupo 66`, `Grupo 67`?**
  _High betweenness centrality (0.121) - this node is a cross-community bridge._
- **Why does `parseOTLPProtobuf()` connect `Grupo 46` to `Grupo 32`, `Grupo 49`, `Grupo 21`, `Propagação entre Serviços`?**
  _High betweenness centrality (0.030) - this node is a cross-community bridge._
- **Why does `Finding` connect `Detectores de Banco` to `Explicação de Suspeito`, `Ranking e Pesos`, `Propagação entre Serviços`, `Relatório JSON de Incidente`, `Snapshots de Diagnóstico`, `Dublês de Leitura`, `Suíte dos Detectores`, `Grupo 18`, `Grupo 20`, `Grupo 22`, `Grupo 25`, `Grupo 27`, `Grupo 29`, `Grupo 30`, `Grupo 33`, `Grupo 34`, `Grupo 50`, `Grupo 51`, `Grupo 53`, `Grupo 54`, `Grupo 57`?**
  _High betweenness centrality (0.029) - this node is a cross-community bridge._
- **What connects `Request`, `results`, `generate-traffic.sh script` to the rest of the system?**
  _10 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Ingestão de Catálogo` be split into smaller, more focused modules?**
  _Cohesion score 0.06265389876880985 - nodes in this community are weakly interconnected._
- **Should `Comandos da CLI` be split into smaller, more focused modules?**
  _Cohesion score 0.07753164556962025 - nodes in this community are weakly interconnected._
- **Should `Grafo de Evidências do Trace` be split into smaller, more focused modules?**
  _Cohesion score 0.07539682539682539 - nodes in this community are weakly interconnected._