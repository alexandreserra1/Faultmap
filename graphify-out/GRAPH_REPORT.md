# Graph Report - Faultmap  (2026-09-13)

## Corpus Check
- 244 files · ~297,308 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1770 nodes · 5100 edges · 86 communities (73 shown, 13 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 741 edges (avg confidence: 0.83)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Ingestão de Catálogo
- Comandos da CLI
- Grafo de Conhecimento e Changelog
- Explicação de Suspeito e Causas Comuns
- Grafo de Evidências do Trace
- Receptor OTLP e Mock GitHub
- Servidor MCP
- Vínculo Banco-Serviço
- Serviços da Demo Shop
- Retenção de Telemetria
- Ingestão do GitHub e Carga
- CI, Release e Cenários de Teste
- Detectores de Banco e Convenções
- Mudanças do GitHub
- Bootstrap e Testes da CLI
- Propagação entre Serviços
- Princípios do Produto
- Taxa de Erro e Ruído Amostral
- Diagrama de Arquitetura
- Relatório JSON de Incidente
- Snapshots de Diagnóstico
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
- Grupo 83
- Grupo 84
- Grupo 85

## God Nodes (most connected - your core abstractions)
1. `Signal` - 131 edges
2. `Finding` - 54 edges
3. `newRootCommand()` - 42 edges
4. `Rank()` - 42 edges
5. `DiagnoseIncidentInScope()` - 41 edges
6. `DetectSchemaChangeProximity()` - 37 edges
7. `Migrate()` - 34 edges
8. `Load()` - 32 edges
9. `Open()` - 31 edges
10. `PersistedDiagnosis` - 30 edges

## Surprising Connections (you probably didn't know these)
- `Defeito no checkout, não na dependência` --references--> `Rank()`  [INFERRED]
  examples/demo-shop/scenarios/retry-storm/compose.yaml → internal/ranking/ranking.go
- `Pool *sql.DB único por banco` --semantically_similar_to--> `Cenário: pool pequeno`  [INFERRED] [semantically similar]
  docs/mvp/05-entrega-e-diretrizes.md → examples/demo-shop/scenarios/small-pool/README.md
- `A lista de bloqueios do YAML soma aos padrões` --rationale_for--> `privacyPolicyFrom()`  [INFERRED]
  docs/adr/0012-bloqueios-de-privacidade-somam-em-vez-de-substituir.md → cmd/faultmap/root.go
- `Retenção apaga telemetria e preserva snapshots` --rationale_for--> `ApplyRetention()`  [INFERRED]
  docs/adr/0003-retencao-preserva-snapshots-de-diagnostico.md → internal/application/apply_retention.go
- `Retenção apaga telemetria e preserva snapshots` --references--> `BlameTrace()`  [INFERRED]
  docs/adr/0003-retencao-preserva-snapshots-de-diagnostico.md → internal/application/blame_trace.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Privacidade aplicada antes do disco** — docs_adr_0008_politica_de_privacidade_aplicada_na_ingestao_politica_aplicada_na_ingestao, docs_adr_0011_logs_guardam_apenas_metadados_logs_sem_texto_da_mensagem, docs_adr_0012_bloqueios_de_privacidade_somam_em_vez_de_substituir_uniao_de_bloqueios, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_privacy_policy, readme_privacidade [INFERRED 0.85]
- **Orçamento de peso por classe no ranking** — docs_adr_0001_ranking_reutiliza_peso_graph_proximity_reuso_do_peso_graph_proximity, docs_adr_0010_teto_por_classe_de_peso_no_ranking_teto_por_classe_de_peso, changelog_teto_relativo_para_evidencia_de_apoio, internal_ranking_ranking_weightclassforrule, internal_ranking_ranking_weightforclass, internal_ranking_ranking_rank [INFERRED 0.85]
- **Cegueira contra instrumentação real** — docs_adr_0006_detectores_aceitam_duas_convencoes_http_duas_convencoes_http, docs_adr_0007_telemetria_real_como_base_de_teste_telemetria_real_como_base_de_teste, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_semconv_attributes, readme_compatibilidade_com_instrumentacao_real, _claude_skills_consultar_grafo_skill_consultar_o_grafo_antes_de_mudar [INFERRED 0.85]
- **Fluxo de ingestão OTLP da demo até o SQLite** — examples_demo_shop_compose_checkout_service, examples_demo_shop_compose_payment_service, examples_demo_shop_otel_collector_pipeline_de_traces, docs_mvp_02_dominio_dados_e_telemetria_receiver_otlp_http, docs_mvp_02_dominio_dados_e_telemetria_persistencia_sqlite [EXTRACTED 1.00]
- **Cenários que exercitam evidência de banco** — examples_demo_shop_scenarios_database_slow_readme_cenario_banco_lento, examples_demo_shop_scenarios_small_pool_readme_cenario_pool_pequeno, examples_demo_shop_scenarios_table_lock_readme_cenario_lock_na_tabela_de_pagamentos, internal_detection_detectors_detectdatabasetimeout, internal_detection_detectors_detectlatencydelta [INFERRED 0.85]
- **Protocolo do piloto cego** — examples_pilot_readme_piloto_cego, examples_pilot_readme_operador_e_investigador, examples_pilot_collector_collector_do_piloto, examples_pilot_registro_registro_do_investigador, examples_pilot_resultado_resultado_do_piloto_cego [EXTRACTED 1.00]

## Communities (86 total, 13 thin omitted)

### Community 0 - "Ingestão de Catálogo"
Cohesion: 0.05
Nodes (85): escritorDeSchemaFake, fonteDeSchemaFake, SchemaSource, SchemaWriter, scopedSchemaReaderFake, Mudança de schema como sinal de incidente, A coleta de schema guarda identificadores, não expressões, SchemaChange (+77 more)

### Community 1 - "Comandos da CLI"
Cohesion: 0.05
Nodes (94): diagnosisEnd(), githubImportEnd(), newBlameCommand(), newBlameTraceCommand(), newDiagnoseCommand(), newDiagnoseIncidentCommand(), newExplainCommand(), newExplainSuspectCommand() (+86 more)

### Community 2 - "Grafo de Conhecimento e Changelog"
Cohesion: 0.05
Nodes (64): Consultar o grafo antes de mudar, Grafo de conhecimento em graphify-out, graphify affected — travessia reversa, Limites do grafo, Changelog do Faultmap, Piloto cego, Servidor MCP somente leitura, Teto relativo para evidência de apoio (+56 more)

### Community 3 - "Explicação de Suspeito e Causas Comuns"
Cohesion: 0.06
Nodes (61): SuspectContribution, Frases de causas comuns, Lacuna: detector de atraso de consumidor, Catálogo de falhas do OpenTelemetry Demo, strings.Builder, ExplainSuspect(), findingsForService(), findSuspect() (+53 more)

### Community 4 - "Grafo de Evidências do Trace"
Cohesion: 0.08
Nodes (56): TraceInvestigation, TraceSignalReader, edgeAccumulator, nodeAccumulator, NodeKind, BlameTrace(), hasRelation(), TestBlameTraceCarregaUmaVezEConstroiGrafo() (+48 more)

### Community 5 - "Receptor OTLP e Mock GitHub"
Cohesion: 0.07
Nodes (52): mapOTLPEncoding(), authenticate(), NewHandler(), TestHandlerEntregaCommitEDeploymentCoerentes(), TestHandlerProtegeRotasEValidaConfiguracao(), validateConfig(), writeJSON(), delayFromFile() (+44 more)

### Community 6 - "Servidor MCP"
Cohesion: 0.08
Nodes (44): bufio.Reader, encoding/json.Encoder, io.Writer, newAuditor(), TestAuditoriaNaoEscreveNaSaidaDoProtocolo(), TestAuditoriaNaoRegistraOsArgumentos(), TestArgumentosComTipoErradoViramErroDeTool(), TestContextoCanceladoEncerraASessao() (+36 more)

### Community 7 - "Vínculo Banco-Serviço"
Cohesion: 0.08
Nodes (42): databaseTargetsInWindow(), sortedKeys(), safeRetryIdentity(), databaseTargetsQueriedBy(), signalsForChange(), isHTTPSignal(), databaseDetail(), displayDatabaseSystem() (+34 more)

### Community 8 - "Serviços da Demo Shop"
Cohesion: 0.08
Nodes (34): Config, Handler, Request, roundTripperFunc, NewHandler(), TestHandlerEncaminhaPagamentoComSucesso(), TestHandlerFanOutFalhaQuandoUmaChamadaFalha(), TestHandlerFanOutFazChamadasParalelasBemSucedidas() (+26 more)

### Community 9 - "Retenção de Telemetria"
Cohesion: 0.11
Nodes (32): Retenção apaga telemetria e preserva snapshots, database/sql.Rows, NewRetentionRepository(), rollbackRetentionTransaction(), openRetentionDatabase(), signalIDs(), TestRetentionRepositoryPreservaSnapshotsDeIncidentes(), TestRetentionRepositoryRejeitaLimiteInválido() (+24 more)

### Community 10 - "Ingestão do GitHub e Carga"
Cohesion: 0.09
Nodes (31): Ingestão do GitHub limitada a uma página, sem N+1, Serviço checkout-service, Serviço load-generator, New(), TestGeneratorEnviaQuantidadeLimitada(), TestGeneratorLimitaConcorrencia(), TestGeneratorRejeitaLimitesPerigosos(), TestGeneratorRespeitaCancelamento() (+23 more)

### Community 11 - "CI, Release e Cenários de Teste"
Cohesion: 0.12
Nodes (38): Workflow de CI, Matriz E2E fora do gate automático, Binários reproduzíveis com checksums, Workflow de Release, Cenário migracao-inofensiva, Matriz E2E, Modo difícil, Pontuação pela ponta pessimista do intervalo (+30 more)

### Community 12 - "Detectores de Banco e Convenções"
Cohesion: 0.13
Nodes (37): Evidence, Input, Detectores aceitam as duas convenções HTTP e ignoram spans internos, Telemetria de instrumentação real como base de teste, Detector database_http_trace_correlation, Detector database_timeout, Detector latency_delta, generate-traffic.sh script (+29 more)

### Community 13 - "Mudanças do GitHub"
Cohesion: 0.10
Nodes (21): ChangeSource, changeSourceFake, ChangeWriter, changeWriterFake, scopedDeploymentReaderFake, Commit, Deployment, ImportRequest (+13 more)

### Community 14 - "Bootstrap e Testes da CLI"
Cohesion: 0.16
Nodes (32): main(), newRootCommand(), assertTableCount(), preparePersistedIncident(), reserveTCPAddress(), TestBlameTraceCommandExplicaFluxoHTTPPostgreSQL(), TestDiagnoseIncidentCommandDetectaRetryStorm(), TestDiagnoseIncidentCommandExplicaAmostraRepresentativa() (+24 more)

### Community 15 - "Propagação entre Serviços"
Cohesion: 0.12
Nodes (31): propagationSummary, serviceLink, Detector dependency_failure, Detector trace_break, DetectDependencyFailure(), failingAncestorServices(), indexSpansByID(), isFailedSpan() (+23 more)

### Community 16 - "Princípios do Produto"
Cohesion: 0.06
Nodes (33): Backend determinístico, CLI-first e local-first, Explicabilidade, Faultmap, Infraestrutura existente e evolução controlada, Interface Detector, Módulo detection, Módulo integrations (+25 more)

### Community 17 - "Taxa de Erro e Ruído Amostral"
Cohesion: 0.15
Nodes (30): error_rate_delta ignora variação de amostragem, Detector error_rate_delta, DetectErrorRateDelta(), exceedsSamplingNoise(), Run(), assertFinding(), contains(), databaseTimeoutSignals() (+22 more)

### Community 18 - "Diagrama de Arquitetura"
Cohesion: 0.16
Nodes (29): Agents, Applications (Go / Python / Node), CLI Output, Detection Engine, Diagnostic Score, Faultmap MVP — Arquitetura, Entradas, Evidence Graph (+21 more)

### Community 19 - "Relatório JSON de Incidente"
Cohesion: 0.10
Nodes (12): diagnosisReaderFake, leitorDeSchemaQueFalha, retentionRemoverStub, scopedSignalReaderFake, scopeReaderFake, scopeReaderNiveis, signalReaderFake, time.Time (+4 more)

### Community 20 - "Snapshots de Diagnóstico"
Cohesion: 0.15
Nodes (27): DiagnosisRepository, assertIntPointerEqual(), assertTimePointerEqual(), testDiagnosisAt(), TestDiagnosisRepositoryGetRespectsCanceledContext(), TestDiagnosisRepositoryGetRestoresCompleteSnapshot(), TestDiagnosisRepositoryGetReturnsTypedNotFound(), TestDiagnosisRepositoryGetSupportsLegacySnapshot() (+19 more)

### Community 21 - "Grupo 21"
Cohesion: 0.21
Nodes (24): ScopedDeploymentReader, ScopedDiagnosisRequest, ScopeDiscovery, ScopedSchemaChangeReader, ScopedSignalReader, ScopeReader, A investigação compara serviços descobertos pelos traces, applyDependencyTieBreak() (+16 more)

### Community 22 - "Grupo 22"
Cohesion: 0.18
Nodes (21): context.Context, GetIncident(), IncidentHistoryReader, ListIncidents(), TestGetIncidentValidaIDEPropagaAusencia(), TestListIncidentsPreservaOrdemDoRepositorio(), TestListIncidentsValidaLimiteAntesDoRepositorio(), TestPersistedDiagnosisMetadataComplete() (+13 more)

### Community 23 - "Grupo 23"
Cohesion: 0.15
Nodes (18): DiagnosisStore, diagnosisStoreFake, DiagnosisID(), Diagnosis, PersistDiagnosis(), TestDiagnosisIDEDeterministicoEmUTC(), TestPersistDiagnosisNaoSalvaIncidenteSemSinais(), TestPersistDiagnosisPreservaCausaDoStore() (+10 more)

### Community 24 - "Grupo 24"
Cohesion: 0.23
Nodes (20): encoding/json.RawMessage, exceptionAttributes(), normalizeAttributes(), normalizeExportRequest(), normalizeSpan(), parseUnixNano(), resourceSignalAttributes(), scalarJSONValue() (+12 more)

### Community 25 - "Grupo 25"
Cohesion: 0.23
Nodes (22): cadeiaSignal(), escopoHTTPSignal(), escopoSpanDeBanco(), sinalComVersao(), TestDiagnoseScopeAcusaOCommitImplantado(), TestDiagnoseScopeAcusaSchemaQuandoHaSintoma(), TestDiagnoseScopeAlcançaSegundoSalto(), TestDiagnoseScopeConsultaDeploymentsEmLote() (+14 more)

### Community 26 - "Grupo 26"
Cohesion: 0.16
Nodes (14): incidentHistoryReaderFake, database/sql.NullInt64, database/sql.NullTime, database/sql.Tx, IncidentSummary, PersistedDiagnosis, DiagnosisRepository, nullIntPointer() (+6 more)

### Community 27 - "Grupo 27"
Cohesion: 0.16
Nodes (20): deploymentCandidate, Configuração E2E com GitHub, Mock GitHub no loopback do container, Matriz E2E automatizada, Override timeout-after-deploy (SHA como SERVICE_VERSION), Cenário: timeout depois de uma mudança, commitLabel(), commitSHAFromEvidence() (+12 more)

### Community 28 - "Grupo 28"
Cohesion: 0.16
Nodes (12): activate_scenario(), assert_contains(), assert_json(), cleanup(), compose(), diagnose(), first_trace_id(), run_scenario() (+4 more)

### Community 29 - "Grupo 29"
Cohesion: 0.21
Nodes (17): Detector database_error, attributeValueOrEmpty(), databaseFailureTypeSuffix(), databaseNonTimeoutFailures(), DetectDatabaseError(), isClientCancellation(), bancoComFalhas(), databaseErrorSignals() (+9 more)

### Community 30 - "Grupo 30"
Cohesion: 0.18
Nodes (15): TestPostgresRepositoryAtrasoOcupaConexaoDoPool(), TestPostgresRepositoryIntegracao(), NewPostgresRepository(), recordDatabaseSpanError(), mustPostgresRepository(), TestNewPostgresRepositoryRejeitaPoolAusente(), TestPostgresRepositoryAtrasoRespeitaContexto(), TestPostgresRepositoryCreateUsaConsultaParametrizada() (+7 more)

### Community 31 - "Grupo 31"
Cohesion: 0.23
Nodes (16): retryCandidate, retryOperationStats, DetectRetryStorm(), retryStats(), bancoNodeRepetido(), databaseRetrySignals(), retrySignals(), serverRetrySignals() (+8 more)

### Community 32 - "Grupo 32"
Cohesion: 0.27
Nodes (16): TestListSchemaChangesLimitaAConsulta(), TestSaveSnapshotAceitaBaseVaziaDesdeOInicio(), TestSaveSnapshotFalhaComBancoFechado(), TestSaveSnapshotFalhaComColetaAnteriorCorrompida(), TestSaveSnapshotRecusaColetaGigante(), TestSaveSnapshotRecusaColetaVaziaContraCatalogoPovoado(), TestSaveSnapshotRejeitaColetaSemIdentidade(), TestSaveSnapshotRespeitaContextoCancelado() (+8 more)

### Community 33 - "Grupo 33"
Cohesion: 0.24
Nodes (15): timeline.json ancora findings na janela do incidente, collectChangeIDs(), collectSignalIDs(), copyIntPointer(), findingSummary(), newDocument(), Render(), sortedStrings() (+7 more)

### Community 34 - "Grupo 34"
Cohesion: 0.23
Nodes (12): signalStoreFake, traceReaderFake, SignalType, math/rand.Rand, signalsForService(), consultaDeBanco(), diagnosticar(), embaralhar() (+4 more)

### Community 35 - "Grupo 35"
Cohesion: 0.28
Nodes (14): versionStats, Detector version_regression, comparableVersionStats(), DetectVersionRegression(), TestVersionRegressionComparaDuasVersõesNaMesmaJanela(), TestVersionRegressionDetectaLatênciaPiorEmUmaVersão(), TestVersionRegressionIgnoraDiferençaDentroDoRuído(), TestVersionRegressionSilenciaComUmaÚnicaVersão() (+6 more)

### Community 36 - "Grupo 36"
Cohesion: 0.22
Nodes (15): go.opentelemetry.io/proto/otlp/collector/trace/v1.ExportTraceServiceRequest, go.opentelemetry.io/proto/otlp/common/v1.ArrayValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValueList, go.opentelemetry.io/proto/otlp/trace/v1.Span, go.opentelemetry.io/proto/otlp/trace/v1.Span_Event, marshalProtoValue(), protobufAnyValue() (+7 more)

### Community 37 - "Grupo 37"
Cohesion: 0.24
Nodes (13): Identificadores de catálogo ordenáveis por tempo, encode(), extract(), New(), TestAlfabetoNaoTemCaracteresAmbiguos(), TestClassificabilidade(), TestClassificabilidadeDistingueMilissegundos(), TestDeterminismoPreservaIdempotencia() (+5 more)

### Community 38 - "Grupo 38"
Cohesion: 0.14
Nodes (15): Módulo reporting, Módulo storage, Persistência SQLite, Snapshot imutável de diagnóstico, Artefatos gerados em faultmap-out, Comando faultmap blame trace, CLI faultmap, Comando faultmap diagnose incident (+7 more)

### Community 39 - "Grupo 39"
Cohesion: 0.15
Nodes (15): Detector retry_storm, Critérios de aceite do MVP, Métricas de sucesso, Sistema de demonstração demo-shop, Primeira demonstração obrigatória, Roadmap de dez marcos, Serviço payment-service, Serviço postgres da demo (+7 more)

### Community 40 - "Grupo 40"
Cohesion: 0.28
Nodes (14): apply_harmless_migration(), assert_contains(), assert_no_finding(), cleanup(), collect_schema(), compose(), diagnose(), generate_burst_traffic() (+6 more)

### Community 41 - "Grupo 41"
Cohesion: 0.29
Nodes (14): copyIntPointer(), newReport(), orderedSuspects(), reportFinding(), reportSuspect(), reportTime(), sortedStrings(), contribution (+6 more)

### Community 42 - "Grupo 42"
Cohesion: 0.26
Nodes (10): InvestigationWindow, TimeWindow, NewInvestigationWindow(), NewTimeWindow(), TestNewInvestigationWindowAllowsBaselineToMeetIncidentBoundary(), TestNewInvestigationWindowFromIncidentCalculatesContiguousBaseline(), TestNewInvestigationWindowFromIncidentRejectsInvalidInputs(), TestNewInvestigationWindowRejectsOverlappingOrInvalidWindows() (+2 more)

### Community 43 - "Grupo 43"
Cohesion: 0.40
Nodes (13): database/sql.DB, assertIndexColumns(), assertIndexExists(), assertMigrationVersion(), assertTableExists(), createVersionOneSchema(), migrationRowCount(), openMigrationTestDatabase() (+5 more)

### Community 44 - "Grupo 44"
Cohesion: 0.24
Nodes (13): go.opentelemetry.io/proto/otlp/common/v1.AnyValue, ParseOTLPJSON(), assertSignalEqual(), intAnyValue(), stringAnyValue(), TestParseOTLPJSONHonorsCancelledContext(), TestParseOTLPJSONNormalizesResourceSpans(), TestParseOTLPJSONPreservaExceçãoESuprimeStacktrace() (+5 more)

### Community 45 - "Grupo 45"
Cohesion: 0.23
Nodes (10): loadConfig(), main(), run(), NewHTTPServer(), RunHTTPServer(), SignalContext(), TestNewHTTPServerConfiguraLimitesEHealth(), TestRunHTTPServerEncerraComContexto() (+2 more)

### Community 46 - "Grupo 46"
Cohesion: 0.19
Nodes (10): _current_fault(), _fault_file(), _FaultInjector, _install_database_delay(), _parse_fault(), Ponto de entrada que instrumenta o DuckDB antes de carregar a aplicação. Fica…, Atrasa cada consulta quando a falha ativa for db_slow. O patch é no execute da…, Arquivo consultado a cada requisição para saber se a falha está ligada. Ler de… (+2 more)

### Community 47 - "Grupo 47"
Cohesion: 0.27
Nodes (10): io.Reader, classifyOTLPError(), OTLPEncoding, contextError(), TestParseOTLPTracesRejeitaCodificacaoDesconhecida(), ParseOTLPLogs(), ParseOTLPTraces(), parseOTLPProtobuf() (+2 more)

### Community 48 - "Grupo 48"
Cohesion: 0.38
Nodes (10): IngestionResult, SignalStore, contextError(), IngestLogs(), IngestTelemetry(), IngestTelemetryFile(), TestIngestTelemetryFileNormalizaEPersiste(), TestIngestTelemetryPreservaClassificacaoDePayloadInvalido() (+2 more)

### Community 49 - "Grupo 49"
Cohesion: 0.42
Nodes (11): readRequiredFile(), requireContains(), TestDemoPublicaPortasSomenteNoLoopback(), TestDemoUsaPortaDeHostDedicada(), TestLoadGeneratorRecebeIdentidadeOTel(), TestReadmesDocumentamExecucaoDaDemo(), TestRunnerE2EDeclaraMatrizLimitesELimpeza(), TestScenariosDocumentamContratoReproduzivel() (+3 more)

### Community 50 - "Grupo 50"
Cohesion: 0.36
Nodes (10): DetectLogCorrelation(), errorLogs(), filterLogSignals(), logsDeErro(), requisicoes(), TestLogCorrelationExigeCorrelaçãoComTrace(), TestLogCorrelationIgnoraRuídoConstante(), TestLogCorrelationLigaErrosDeLogAoTraceQueFalhou() (+2 more)

### Community 51 - "Grupo 51"
Cohesion: 0.40
Nodes (10): executar(), gravarColetasDeCatalogo(), gravarTelemetriaDeBanco(), sessaoMCP(), TestIngestSchemaCommandNaoGravaCredencialNoWorkspace(), TestIngestSchemaCommandValidaEntradaAntesDeAbrirConexao(), TestMCPCommandFalaOProtocoloDePontaAPonta(), TestMCPCommandNaoPoluiStdout() (+2 more)

### Community 52 - "Grupo 52"
Cohesion: 0.29
Nodes (5): Environment, loadConfig(), main(), run(), config

### Community 53 - "Grupo 53"
Cohesion: 0.18
Nodes (11): Módulo telemetry, Allowlist de atributos de Resource, Ingestão OTLP, InvestigationWindow, Janelas de investigação (baseline e incidente), Metadados legados anuláveis, Signal, Testes obrigatórios (+3 more)

### Community 54 - "Grupo 54"
Cohesion: 0.33
Nodes (8): RetentionRequest, RetentionResult, SignalRetentionRemover, ApplyRetention(), TestApplyRetentionCalculaCorteEEncerraQuandoNãoHáMaisSinais(), TestApplyRetentionPropagaFalhaSemMascararProgresso(), TestApplyRetentionRespeitaTetoDeLotes(), TestApplyRetentionValidaEntrada()

### Community 55 - "Grupo 55"
Cohesion: 0.33
Nodes (7): mapLookup(), TestLoadConfigConvertePoolPostgres(), TestLoadConfigRejeitaIdleMaiorQueOpen(), Lookup, NewEnvironment(), TestEnvironmentRejeitaConfiguracaoInsegura(), TestEnvironmentValidaValoresObrigatorios()

### Community 56 - "Grupo 56"
Cohesion: 0.42
Nodes (8): DetectDatabaseLatencyDelta(), exceedsDatabaseLatencyNoise(), bancoComLatência(), TestDatabaseLatencyDeltaAcusaBancoQueDegradouSemFalhar(), TestDatabaseLatencyDeltaExigeDuasJanelas(), TestDatabaseLatencyDeltaIgnoraOperaçõesQueFalharam(), TestDatabaseLatencyDeltaIgnoraOscilaçãoNormal(), TestDatabaseLatencyDeltaMedeAsOperaçõesQueConcluíram()

### Community 57 - "Grupo 57"
Cohesion: 0.38
Nodes (8): RenderRanking(), rankingSnapshot(), TestRankingPreservaAOrdemDoSnapshot(), TestRenderRankingNormalizaInstanteParaUTC(), TestRenderRankingPreservaOrdemDoSnapshotEOrdenaContribuicoesPorRegra(), TestRenderRankingProduzBytesIdenticosEmDuasExecucoes(), TestRenderRankingSemSuspeitosEscreveListaVaziaENaoNula(), rankingDocument

### Community 58 - "Grupo 58"
Cohesion: 0.39
Nodes (7): config, loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCheckout(), TestLoadConfigRejeitaRetryIlimitado()

### Community 59 - "Grupo 59"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteGitHubMock(), TestLoadConfigExigeSHA(), config

### Community 60 - "Grupo 60"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCargaLimitada(), TestLoadConfigRejeitaConcorrenciaExcessiva(), config

### Community 61 - "Grupo 61"
Cohesion: 0.36
Nodes (6): Setup(), TestConfigValidateExigeIdentidadeCompleta(), TestDisabledMantemShutdownSeguro(), TestTraceEndpointAcrescentaCaminhoOTLP(), traceEndpoint(), Config

### Community 62 - "Grupo 62"
Cohesion: 0.36
Nodes (7): ensureContext(), InitializeProject(), assertDirectoryExists(), assertFileExists(), TestInitializeProjectCreatesLocalWorkspace(), TestInitializeProjectDoesNotOverwriteExistingConfiguration(), TestInitializeProjectHonorsCancelledContext()

### Community 63 - "Grupo 63"
Cohesion: 0.42
Nodes (8): carregarFixtureReal(), separarPorFalha(), serviçosDaFixture(), TestDetectorDeBancoDisparaComTelemetriaReal(), TestDetectoresNovosNãoAcusamTelemetriaRealSaudável(), TestFalhaRealDeBancoÉReconhecida(), TestTelemetriaRealDeBancoÉEnxergada(), TestTelemetriaRealHTTPÉEnxergada()

### Community 64 - "Grupo 64"
Cohesion: 0.47
Nodes (8): RenderIncidentSummary(), summarySnapshot(), TestRenderIncidentSummaryNaoDuplicaRelatorioCompleto(), TestRenderIncidentSummaryProduzBytesIdenticosEmDuasExecucoes(), TestRenderIncidentSummaryResumeIdentificacaoJanelasEContagens(), TestRenderIncidentSummarySemMetadadosCompletosOmiteBaseline(), TestRenderIncidentSummarySemSuspeitosOmiteSuspeitoPrincipal(), summaryDocument

### Community 65 - "Grupo 65"
Cohesion: 0.36
Nodes (7): encoding/json.Decoder, rejectTrailingJSON(), ParseOTLPLogsJSON(), TestParseOTLPLogsNuncaArmazenaOTextoDaMensagem(), TestParseOTLPLogsPreservaSeveridadeECorrelação(), TestParseOTLPLogsRejeitaEnvelopeInválido(), TestParseOTLPLogsÉIdempotentePorRegistro()

### Community 66 - "Grupo 66"
Cohesion: 0.54
Nodes (7): logRecordID(), logSeverity(), normalizeLogRecord(), exportLogsServiceRequest, logRecord, resourceLog, scopeLog

### Community 67 - "Grupo 67"
Cohesion: 0.43
Nodes (5): SignalReader, ListSignals(), TestListSignalsEncaminhaConsulta(), TestListSignalsRespeitaContextoCancelado(), TestListSignalsValidaEntradaAntesDeConsultar()

### Community 68 - "Grupo 68"
Cohesion: 0.67
Nodes (6): testing.F, exigirErroClassificado(), exigirSinaisCoerentes(), FuzzParseOTLPLogsJSONNãoDevolveOCorpo(), FuzzParseOTLPTracesJSON(), FuzzParseOTLPTracesProtobuf()

### Community 69 - "Grupo 69"
Cohesion: 0.33
Nodes (6): Receiver OTLP HTTP POST /v1/traces, Comando faultmap serve, Receiver OTLP sem autenticação nem TLS, Segurança e privacidade, Pipeline de traces do Collector da demo, Defeito: blocked_attributes substituía os padrões

### Community 70 - "Grupo 70"
Cohesion: 0.40
Nodes (5): Módulo evidence, EvidenceEdge, EvidenceNode, Fallback de parentesco de spans, Grafo de evidências

### Community 71 - "Grupo 71"
Cohesion: 0.70
Nodes (4): Render(), reportDiagnosis(), TestRenderProduzContratoVersionadoEDeterministico(), TestRenderRepresentaBaselineLegadaComoNull()

### Community 72 - "Grupo 72"
Cohesion: 0.83
Nodes (3): limpar(), rodar-ranking.sh script, trafego()

## Ambiguous Edges - Review These
- `database-slow/generate-traffic.sh` → `Cenário: banco lento`  [AMBIGUOUS]
  examples/demo-shop/scenarios/database-slow/README.md · relation: calls
- `server.go` → `MCP`  [AMBIGUOUS]
  docs/images/faultmap-mvp-architecture.png · relation: references
- `PostgreSQL (DB signals / stats)` → `Detection Engine`  [AMBIGUOUS]
  docs/images/faultmap-mvp-architecture.png · relation: shares_data_with
- `PostgreSQL (DB signals / stats)` → `Evidence Graph`  [AMBIGUOUS]
  docs/images/faultmap-mvp-architecture.png · relation: shares_data_with

## Knowledge Gaps
- **49 isolated node(s):** `Request`, `results`, `generate-traffic.sh script`, `generate-traffic.sh script`, `generate-traffic.sh script` (+44 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **13 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `database-slow/generate-traffic.sh` and `Cenário: banco lento`?**
  _Edge tagged AMBIGUOUS (relation: calls) - confidence is low._
- **What is the exact relationship between `server.go` and `MCP`?**
  _Edge tagged AMBIGUOUS (relation: references) - confidence is low._
- **What is the exact relationship between `PostgreSQL (DB signals / stats)` and `Detection Engine`?**
  _Edge tagged AMBIGUOUS (relation: shares_data_with) - confidence is low._
- **What is the exact relationship between `PostgreSQL (DB signals / stats)` and `Evidence Graph`?**
  _Edge tagged AMBIGUOUS (relation: shares_data_with) - confidence is low._
- **Why does `Signal` connect `Grupo 34` to `Grafo de Conhecimento e Changelog`, `Grafo de Evidências do Trace`, `Vínculo Banco-Serviço`, `Retenção de Telemetria`, `CI, Release e Cenários de Teste`, `Detectores de Banco e Convenções`, `Propagação entre Serviços`, `Taxa de Erro e Ruído Amostral`, `Relatório JSON de Incidente`, `Grupo 21`, `Grupo 24`, `Grupo 25`, `Grupo 27`, `Grupo 29`, `Grupo 31`, `Grupo 35`, `Grupo 44`, `Grupo 47`, `Grupo 50`, `Grupo 56`, `Grupo 63`, `Grupo 65`, `Grupo 66`, `Grupo 67`, `Grupo 68`?**
  _High betweenness centrality (0.116) - this node is a cross-community bridge._
- **Why does `Finding` connect `Detectores de Banco e Convenções` to `Grafo de Conhecimento e Changelog`, `Explicação de Suspeito e Causas Comuns`, `CI, Release e Cenários de Teste`, `Propagação entre Serviços`, `Taxa de Erro e Ruído Amostral`, `Relatório JSON de Incidente`, `Snapshots de Diagnóstico`, `Grupo 21`, `Grupo 22`, `Grupo 23`, `Grupo 26`, `Grupo 27`, `Grupo 29`, `Grupo 31`, `Grupo 33`, `Grupo 35`, `Grupo 41`, `Grupo 50`, `Grupo 56`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Why does `DetectSchemaChangeProximity()` connect `CI, Release e Cenários de Teste` to `Ingestão de Catálogo`, `Grafo de Conhecimento e Changelog`, `Vínculo Banco-Serviço`, `Detectores de Banco e Convenções`, `Relatório JSON de Incidente`, `Grupo 21`, `Grupo 27`?**
  _High betweenness centrality (0.030) - this node is a cross-community bridge._