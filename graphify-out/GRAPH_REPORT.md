# Graph Report - Faultmap  (2026-09-22)

## Corpus Check
- 265 files · ~341,349 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2082 nodes · 5976 edges · 112 communities (97 shown, 15 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 801 edges (avg confidence: 0.82)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `a880d5f4`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- time.Duration
- Load
- Rank
- NewHandler
- graph.go
- IngestFunc
- historicoComUmIncidente
- attributes.go
- payment/handler_test.go
- NewSignalRepository
- client.go
- DetectSchemaChangeProximity
- New
- server.go
- newRootCommand
- DetectTraceBreak
- Integração GitHub
- detectors.go
- Faultmap MVP — Arquitetura
- context.Context
- openDiagnosisRepository
- diagnose_scope.go
- IncidentHistoryReader
- conformidade.go
- otlp_json.go
- DiagnoseIncidentInScope
- database/sql.Tx
- Render
- run-e2e.sh
- DetectDatabaseError
- Environment
- DetectRetryStorm
- openSchemaRepository
- CommonCauses
- Faultmap
- DetectVersionRegression
- otlp_protobuf.go
- New
- CLI faultmap
- Handler
- run-hard-mode.sh
- json/report.go
- InvestigationWindow
- database/sql.DB
- ParseOTLPJSON
- medir-ruido-do-banco.py
- faultmap_app.py
- timeline.go
- IngestTelemetry
- acceptance_test.go
- DetectLogCorrelation
- schema_mcp_integration_test.go
- NewEnvironment
- filterDatabaseSignals
- run
- Demo Shop
- DetectDatabaseLatencyDelta
- openChangeRepository
- sortear-incidente-test.sh
- sqlite/diagnosis_repository.go
- SchemaSnapshot
- medir-ruido-do-banco.sh
- InitializeProject
- run
- run
- Deployment
- otlp_logs_json.go
- io.Writer
- ParseOTLPTraces
- Fixtures OpenTelemetry
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
- IngestChanges
- PersistedDiagnosis
- run
- Piloto cego
- postgres/signal_repository.go
- repositorioComRetencao
- deploy-inofensivo
- RenderSuspectExplanation
- ParseOTLPLogsJSON
- ADR 0019 — Mudança de catálogo dentro do incidente não é acusada
- snapshotDeExemplo
- postgres-demo
- Finding
- Sorteio cego na demo-shop
- RenderRanking
- Setup
- Open
- RenderIncidentSummary
- testSignal
- Migrate
- TestConformidadeIntegracaoPostgres
- ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada
- testing.T
- Signal
- sortear-incidente.sh

## God Nodes (most connected - your core abstractions)
1. `Signal` - 143 edges
2. `Finding` - 59 edges
3. `Rank()` - 45 edges
4. `newRootCommand()` - 42 edges
5. `DetectSchemaChangeProximity()` - 42 edges
6. `DiagnoseIncidentInScope()` - 41 edges
7. `Load()` - 34 edges
8. `PersistedDiagnosis` - 32 edges
9. `NewInvestigationWindowFromIncident()` - 31 edges
10. `Open()` - 29 edges

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
- **Fluxo de ingestão OTLP da demo até o SQLite** — examples_demo_shop_compose_checkout_service, examples_demo_shop_compose_payment_service, examples_demo_shop_otel_collector_pipeline_de_traces, docs_mvp_02_dominio_dados_e_telemetria_receiver_otlp_http, docs_mvp_02_dominio_dados_e_telemetria_persistencia_sqlite [EXTRACTED 1.00]
- **Protocolo do piloto cego** — examples_pilot_readme_piloto_cego, examples_pilot_readme_operador_e_investigador, examples_pilot_collector_collector_do_piloto, examples_pilot_registro_registro_do_investigador, examples_pilot_resultado_resultado_do_piloto_cego [EXTRACTED 1.00]
- **Cegueira contra instrumentação real** — docs_adr_0006_detectores_aceitam_duas_convencoes_http_duas_convencoes_http, docs_adr_0007_telemetria_real_como_base_de_teste_telemetria_real_como_base_de_teste, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_semconv_attributes, readme_compatibilidade_com_instrumentacao_real, _claude_skills_consultar_grafo_skill_consultar_o_grafo_antes_de_mudar [INFERRED 0.85]
- **Cenários que exercitam evidência de banco** — examples_demo_shop_scenarios_database_slow_readme_cenario_banco_lento, examples_demo_shop_scenarios_small_pool_readme_cenario_pool_pequeno, examples_demo_shop_scenarios_table_lock_readme_cenario_lock_na_tabela_de_pagamentos, internal_detection_detectors_detectdatabasetimeout, internal_detection_detectors_detectlatencydelta [INFERRED 0.85]
- **Orçamento de peso por classe no ranking** — docs_adr_0001_ranking_reutiliza_peso_graph_proximity_reuso_do_peso_graph_proximity, docs_adr_0010_teto_por_classe_de_peso_no_ranking_teto_por_classe_de_peso, changelog_teto_relativo_para_evidencia_de_apoio, internal_ranking_ranking_weightclassforrule, internal_ranking_ranking_weightforclass, internal_ranking_ranking_rank [INFERRED 0.85]
- **Privacidade aplicada antes do disco** — docs_adr_0008_politica_de_privacidade_aplicada_na_ingestao_politica_aplicada_na_ingestao, docs_adr_0011_logs_guardam_apenas_metadados_logs_sem_texto_da_mensagem, docs_adr_0012_bloqueios_de_privacidade_somam_em_vez_de_substituir_uniao_de_bloqueios, docs_adr_0014_schema_guarda_identificadores_nao_expressoes_identificadores_nao_expressoes, internal_telemetry_privacy_policy, readme_privacidade [INFERRED 0.85]

## Communities (112 total, 15 thin omitted)

### Community 0 - "time.Duration"
Cohesion: 0.18
Nodes (15): TestPostgresRepositoryAtrasoOcupaConexaoDoPool(), TestPostgresRepositoryIntegracao(), NewPostgresRepository(), recordDatabaseSpanError(), mustPostgresRepository(), TestNewPostgresRepositoryRejeitaPoolAusente(), TestPostgresRepositoryAtrasoRespeitaContexto(), TestPostgresRepositoryCreateUsaConsultaParametrizada() (+7 more)

### Community 1 - "Load"
Cohesion: 0.05
Nodes (104): Conexao, RepositorioDeDiagnostico, RepositorioDeMudancas, diagnosisEnd(), githubImportEnd(), newBlameCommand(), newBlameTraceCommand(), newDiagnoseCommand() (+96 more)

### Community 2 - "Rank"
Cohesion: 0.05
Nodes (63): Consultar o grafo antes de mudar, Grafo de conhecimento em graphify-out, graphify affected — travessia reversa, Limites do grafo, Changelog do Faultmap, Piloto cego, Servidor MCP somente leitura, Teto relativo para evidência de apoio (+55 more)

### Community 3 - "NewHandler"
Cohesion: 0.24
Nodes (13): roundTripperFunc, NewHandler(), TestHandlerEncaminhaPagamentoComSucesso(), TestHandlerFanOutFalhaQuandoUmaChamadaFalha(), TestHandlerFanOutFazChamadasParalelasBemSucedidas(), TestHandlerRepetePagamentoAteLimite(), TestHandlerRespeitaCancelamentoDoContexto(), TestHandlerValidaEntradaEMetodo() (+5 more)

### Community 4 - "graph.go"
Cohesion: 0.07
Nodes (54): TraceInvestigation, RepositorioDeSinais, edgeAccumulator, nodeAccumulator, NodeKind, BlameTrace(), TraceSignalReader, hasRelation() (+46 more)

### Community 5 - "IngestFunc"
Cohesion: 0.07
Nodes (52): mapOTLPEncoding(), authenticate(), NewHandler(), TestHandlerEntregaCommitEDeploymentCoerentes(), TestHandlerProtegeRotasEValidaConfiguracao(), validateConfig(), writeJSON(), delayFromFile() (+44 more)

### Community 6 - "historicoComUmIncidente"
Cohesion: 0.16
Nodes (25): TestAuditoriaNaoEscreveNaSaidaDoProtocolo(), TestAuditoriaNaoRegistraOsArgumentos(), TestArgumentosComTipoErradoViramErroDeTool(), TestContextoCanceladoEncerraASessao(), TestErroDoRepositorioViraErroDeToolENaoDerrubaOServidor(), TestIncidenteInexistenteExplicaOQueAconteceu(), TestLinhaGiganteNaoDerrubaASessao(), TestLoteJSONRPCRecebeRespostaEmVezDeSilencio() (+17 more)

### Community 7 - "attributes.go"
Cohesion: 0.10
Nodes (36): databaseTargetsInWindow(), sortedKeys(), databaseSystem(), safeRetryIdentity(), databaseTargetsQueriedBy(), signalsForChange(), databaseDetail(), displayDatabaseSystem() (+28 more)

### Community 8 - "payment/handler_test.go"
Cohesion: 0.17
Nodes (19): chronicFailure(), NewHandler(), executePayment(), mustHandler(), TestHandlerConverteFalhaDoBancoSemVazarDetalhes(), TestHandlerErroCrônicoÉDeterminísticoPorPedido(), TestHandlerForcaStatusSemPersistir(), TestHandlerPersistePagamento() (+11 more)

### Community 9 - "NewSignalRepository"
Cohesion: 0.22
Nodes (19): TestConformidadeSQLite(), NewDiagnosisRepository(), RetentionRepository, NewRetentionRepository(), openRetentionDatabase(), signalIDs(), TestRetentionRepositoryPreservaSnapshotsDeIncidentes(), TestRetentionRepositoryRejeitaLimiteInválido() (+11 more)

### Community 10 - "client.go"
Cohesion: 0.17
Nodes (17): Ingestão do GitHub limitada a uma página, sem N+1, FetchRequest, Client, commitResponse, deploymentResponse, net/url.URL, net/url.Values, isLoopbackHost() (+9 more)

### Community 11 - "DetectSchemaChangeProximity"
Cohesion: 0.05
Nodes (76): Workflow de CI, Matriz E2E fora do gate automático, Binários reproduzíveis com checksums, Workflow de Release, escritorDeSchemaFake, SchemaSource, Cenário migracao-inofensiva, Matriz E2E (+68 more)

### Community 12 - "New"
Cohesion: 0.27
Nodes (9): New(), TestGeneratorEnviaQuantidadeLimitada(), TestGeneratorLimitaConcorrencia(), TestGeneratorRejeitaLimitesPerigosos(), TestGeneratorRespeitaCancelamento(), net/http.Client, Config, Generator (+1 more)

### Community 13 - "server.go"
Cohesion: 0.30
Nodes (10): bufio.Reader, encoding/json.Encoder, dispatch(), handle(), protocolError(), readLine(), Options, request (+2 more)

### Community 14 - "newRootCommand"
Cohesion: 0.13
Nodes (29): main(), newRootCommand(), assertTableCount(), preparePersistedIncident(), reserveTCPAddress(), TestBlameTraceCommandExplicaFluxoHTTPPostgreSQL(), TestDiagnoseIncidentCommandDetectaRetryStorm(), TestDiagnoseIncidentCommandExplicaAmostraRepresentativa() (+21 more)

### Community 15 - "DetectTraceBreak"
Cohesion: 0.12
Nodes (31): propagationSummary, serviceLink, Detector dependency_failure, Detector trace_break, DetectDependencyFailure(), failingAncestorServices(), indexSpansByID(), isFailedSpan() (+23 more)

### Community 16 - "Integração GitHub"
Cohesion: 0.25
Nodes (8): Módulo integrations, Índice composto (trace_id, timestamp, id), Integração GitHub, Integração PostgreSQL, Uma página por execução na coleta GitHub, Pool *sql.DB único por banco, Proibição de consultas N+1, Regras de implementação para código e banco

### Community 17 - "detectors.go"
Cohesion: 0.09
Nodes (58): Evidence, Input, error_rate_delta ignora variação de amostragem, Detectores aceitam as duas convenções HTTP e ignoram spans internos, Detector database_http_trace_correlation, Detector database_timeout, Detector error_rate_delta, Detector latency_delta (+50 more)

### Community 18 - "Faultmap MVP — Arquitetura"
Cohesion: 0.16
Nodes (29): Agents, Applications (Go / Python / Node), CLI Output, Detection Engine, Diagnostic Score, Faultmap MVP — Arquitetura, Entradas, Evidence Graph (+21 more)

### Community 19 - "context.Context"
Cohesion: 0.08
Nodes (19): leitorDeSchemaQueFalha, retentionRemoverStub, scopedSignalReaderFake, scopeReaderFake, scopeReaderNiveis, signalReaderFake, Retenção apaga telemetria e preserva snapshots, context.Context (+11 more)

### Community 20 - "openDiagnosisRepository"
Cohesion: 0.15
Nodes (27): DiagnosisRepository, assertIntPointerEqual(), assertTimePointerEqual(), testDiagnosisAt(), TestDiagnosisRepositoryGetRespectsCanceledContext(), TestDiagnosisRepositoryGetRestoresCompleteSnapshot(), TestDiagnosisRepositoryGetReturnsTypedNotFound(), TestDiagnosisRepositoryGetSupportsLegacySnapshot() (+19 more)

### Community 21 - "diagnose_scope.go"
Cohesion: 0.17
Nodes (23): ScopedDiagnosisRequest, ScopeDiscovery, RepositorioDeCatalogo, A investigação compara serviços descobertos pelos traces, applyDependencyTieBreak(), containsService(), expandScopeByTraces(), DiagnosisScope (+15 more)

### Community 22 - "IncidentHistoryReader"
Cohesion: 0.24
Nodes (15): IncidentHistoryReader, ListIncidents(), decodeArguments(), describeFindings(), marshalIndented(), toolDefinitions(), toolError(), callTool() (+7 more)

### Community 23 - "conformidade.go"
Cohesion: 0.06
Nodes (66): diagnosisStoreFake, podadorDeCatalogoFake, RetentionRequest, RetentionResult, RetentionRepository, ApplyRetention(), SchemaCatalogPruner, SignalRetentionRemover (+58 more)

### Community 24 - "otlp_json.go"
Cohesion: 0.15
Nodes (28): encoding/json.RawMessage, go.opentelemetry.io/proto/otlp/collector/trace/v1.ExportTraceServiceRequest, go.opentelemetry.io/proto/otlp/common/v1.KeyValue, go.opentelemetry.io/proto/otlp/trace/v1.Span, go.opentelemetry.io/proto/otlp/trace/v1.Span_Event, exceptionAttributes(), normalizeAttributes(), normalizeExportRequest() (+20 more)

### Community 25 - "DiagnoseIncidentInScope"
Cohesion: 0.26
Nodes (25): DiagnoseIncidentInScope(), cadeiaSignal(), escopoHTTPSignal(), escopoSpanDeBanco(), sinalComVersao(), TestDiagnoseScopeAcusaOCommitImplantado(), TestDiagnoseScopeAcusaSchemaQuandoHaSintoma(), TestDiagnoseScopeAlcançaSegundoSalto() (+17 more)

### Community 26 - "database/sql.Tx"
Cohesion: 0.16
Nodes (16): database/sql.NullInt64, database/sql.NullTime, database/sql.Tx, DiagnosisRepository, nullIntPointer(), nullTimePointer(), readPersistedIncident(), readPersistedRanking() (+8 more)

### Community 27 - "Render"
Cohesion: 0.36
Nodes (11): strings.Builder, inlineCode(), markdownText(), markdownTime(), Render(), renderFindings(), renderLimitations(), renderRanking() (+3 more)

### Community 28 - "run-e2e.sh"
Cohesion: 0.16
Nodes (12): activate_scenario(), assert_contains(), assert_json(), cleanup(), compose(), diagnose(), first_trace_id(), run_scenario() (+4 more)

### Community 29 - "DetectDatabaseError"
Cohesion: 0.32
Nodes (12): Detector database_error, DetectDatabaseError(), bancoComFalhas(), databaseErrorSignals(), TestDatabaseErrorAcusaFalhaVerdadeira(), TestDatabaseErrorDetectaCrescimentoDeFalhasDeConexão(), TestDatabaseErrorIgnoraCancelamentoDeInstrumentaçãoReal(), TestDatabaseErrorIgnoraCancelamentoDoCliente() (+4 more)

### Community 30 - "Environment"
Cohesion: 0.29
Nodes (5): loadConfig(), main(), run(), Environment, config

### Community 31 - "DetectRetryStorm"
Cohesion: 0.23
Nodes (16): retryCandidate, retryOperationStats, DetectRetryStorm(), retryStats(), bancoNodeRepetido(), databaseRetrySignals(), retrySignals(), serverRetrySignals() (+8 more)

### Community 32 - "openSchemaRepository"
Cohesion: 0.21
Nodes (19): gravarColetasDeCatalogo(), TestListSchemaChangesLimitaAConsulta(), TestSaveSnapshotAceitaBaseVaziaDesdeOInicio(), TestSaveSnapshotFalhaComBancoFechado(), TestSaveSnapshotFalhaComColetaAnteriorCorrompida(), TestSaveSnapshotRecusaColetaGigante(), TestSaveSnapshotRecusaColetaVaziaContraCatalogoPovoado(), TestSaveSnapshotRejeitaColetaSemIdentidade() (+11 more)

### Community 33 - "CommonCauses"
Cohesion: 0.25
Nodes (9): Frases de causas comuns, Lacuna: detector de atraso de consumidor, Catálogo de falhas do OpenTelemetry Demo, CommonCauses(), TestCausasComunsCobremCatálogoDeTerceiro(), TestCausasComunsIgnoraRegraDesconhecida(), TestCausasComunsOferecemAlternativas(), TestTodaRegraDizOQueAquelePadrãoCostumaSignificar() (+1 more)

### Community 34 - "Faultmap"
Cohesion: 0.15
Nodes (13): Backend determinístico, CLI-first e local-first, Explicabilidade, Faultmap, Infraestrutura existente e evolução controlada, Interface Detector, Módulo detection, Monólito modular em Go (+5 more)

### Community 35 - "DetectVersionRegression"
Cohesion: 0.26
Nodes (15): versionStats, Detector version_regression, percentile95(), comparableVersionStats(), DetectVersionRegression(), TestVersionRegressionComparaDuasVersõesNaMesmaJanela(), TestVersionRegressionDetectaLatênciaPiorEmUmaVersão(), TestVersionRegressionIgnoraDiferençaDentroDoRuído() (+7 more)

### Community 36 - "otlp_protobuf.go"
Cohesion: 0.43
Nodes (7): go.opentelemetry.io/proto/otlp/common/v1.ArrayValue, go.opentelemetry.io/proto/otlp/common/v1.KeyValueList, marshalProtoValue(), protobufAnyValue(), protobufArrayValue(), protobufKVListValue(), protobufNativeValue()

### Community 37 - "New"
Cohesion: 0.24
Nodes (13): Identificadores de catálogo ordenáveis por tempo, encode(), extract(), New(), TestAlfabetoNaoTemCaracteresAmbiguos(), TestClassificabilidade(), TestClassificabilidadeDistingueMilissegundos(), TestDeterminismoPreservaIdempotencia() (+5 more)

### Community 38 - "CLI faultmap"
Cohesion: 0.11
Nodes (20): Módulo reporting, Módulo storage, InvestigationWindow, Janelas de investigação (baseline e incidente), Metadados legados anuláveis, Persistência SQLite, Snapshot imutável de diagnóstico, Artefatos gerados em faultmap-out (+12 more)

### Community 39 - "Handler"
Cohesion: 0.27
Nodes (5): Config, Handler, Request, net/http.Request, net/http.Response

### Community 40 - "run-hard-mode.sh"
Cohesion: 0.26
Nodes (15): apply_harmless_migration(), assert_contains(), assert_no_finding(), cleanup(), collect_schema(), compose(), diagnose(), generate_burst_traffic() (+7 more)

### Community 41 - "json/report.go"
Cohesion: 0.29
Nodes (14): copyIntPointer(), newReport(), orderedSuspects(), reportFinding(), reportSuspect(), reportTime(), sortedStrings(), contribution (+6 more)

### Community 42 - "InvestigationWindow"
Cohesion: 0.16
Nodes (14): diagnosisReaderFake, InvestigationWindow, TimeWindow, diagnosisDatabaseSignal(), hasContribution(), hasFinding(), NewInvestigationWindow(), NewTimeWindow() (+6 more)

### Community 43 - "database/sql.DB"
Cohesion: 0.21
Nodes (20): database/sql.DB, closeAfterFailure(), Open(), applyMigration(), isMigrationApplied(), Migrate(), rollbackMigration(), assertIndexColumns() (+12 more)

### Community 44 - "ParseOTLPJSON"
Cohesion: 0.30
Nodes (11): go.opentelemetry.io/proto/otlp/common/v1.AnyValue, ParseOTLPJSON(), assertSignalEqual(), intAnyValue(), stringAnyValue(), TestParseOTLPJSONHonorsCancelledContext(), TestParseOTLPJSONNormalizesResourceSpans(), TestParseOTLPJSONPreservaExceçãoESuprimeStacktrace() (+3 more)

### Community 45 - "medir-ruido-do-banco.py"
Cohesion: 0.48
Nodes (6): instante(), janelas(), main(), operações_de_banco(), percentil(), Só as que concluíram: é o que o detector mede.

### Community 46 - "faultmap_app.py"
Cohesion: 0.19
Nodes (10): _current_fault(), _fault_file(), _FaultInjector, _install_database_delay(), _parse_fault(), Ponto de entrada que instrumenta o DuckDB antes de carregar a aplicação. Fica…, Atrasa cada consulta quando a falha ativa for db_slow. O patch é no execute da…, Arquivo consultado a cada requisição para saber se a falha está ligada. Ler de… (+2 more)

### Community 47 - "timeline.go"
Cohesion: 0.24
Nodes (15): timeline.json ancora findings na janela do incidente, collectChangeIDs(), collectSignalIDs(), copyIntPointer(), findingSummary(), newDocument(), Render(), sortedStrings() (+7 more)

### Community 48 - "IngestTelemetry"
Cohesion: 0.28
Nodes (12): IngestionResult, io.Reader, contextError(), SignalStore, IngestLogs(), IngestTelemetry(), IngestTelemetryFile(), TestIngestTelemetryFileNormalizaEPersiste() (+4 more)

### Community 49 - "acceptance_test.go"
Cohesion: 0.42
Nodes (11): readRequiredFile(), requireContains(), TestDemoPublicaPortasSomenteNoLoopback(), TestDemoUsaPortaDeHostDedicada(), TestLoadGeneratorRecebeIdentidadeOTel(), TestReadmesDocumentamExecucaoDaDemo(), TestRunnerE2EDeclaraMatrizLimitesELimpeza(), TestScenariosDocumentamContratoReproduzivel() (+3 more)

### Community 50 - "DetectLogCorrelation"
Cohesion: 0.36
Nodes (10): DetectLogCorrelation(), errorLogs(), filterLogSignals(), logsDeErro(), requisicoes(), TestLogCorrelationExigeCorrelaçãoComTrace(), TestLogCorrelationIgnoraRuídoConstante(), TestLogCorrelationLigaErrosDeLogAoTraceQueFalhou() (+2 more)

### Community 51 - "schema_mcp_integration_test.go"
Cohesion: 0.24
Nodes (16): TestBackendPostgresNaoCriaArquivoLocal(), TestComandosDeLeituraFuncionamComPostgres(), TestRetencaoFuncionaComPostgres(), workspacePostgres(), executar(), gravarTelemetriaDeBanco(), sessaoMCP(), TestIngestSchemaCommandNaoGravaCredencialNoWorkspace() (+8 more)

### Community 52 - "NewEnvironment"
Cohesion: 0.33
Nodes (7): mapLookup(), TestLoadConfigConverteGitHubMock(), TestLoadConfigExigeSHA(), Lookup, NewEnvironment(), TestEnvironmentRejeitaConfiguracaoInsegura(), TestEnvironmentValidaValoresObrigatorios()

### Community 53 - "filterDatabaseSignals"
Cohesion: 0.38
Nodes (9): filterDatabaseSignals(), carregarFixtureReal(), separarPorFalha(), serviçosDaFixture(), TestDetectorDeBancoDisparaComTelemetriaReal(), TestDetectoresNovosNãoAcusamTelemetriaRealSaudável(), TestFalhaRealDeBancoÉReconhecida(), TestTelemetriaRealDeBancoÉEnxergada() (+1 more)

### Community 54 - "run"
Cohesion: 0.39
Nodes (7): config, loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCheckout(), TestLoadConfigRejeitaRetryIlimitado()

### Community 55 - "Demo Shop"
Cohesion: 0.18
Nodes (15): Serviço checkout-service, Serviço load-generator, Serviço payment-service, Serviço postgres da demo, Contratos de runtime dos overrides, Demo Shop, Override database-slow (DB_DELAY 750ms), generate-traffic.sh script (+7 more)

### Community 56 - "DetectDatabaseLatencyDelta"
Cohesion: 0.19
Nodes (23): DetectDatabaseLatencyDelta(), exceedsDatabaseLatencyNoise(), succeededDatabaseSignals(), bancoComLatência(), TestDatabaseLatencyDeltaAcusaBancoQueDegradouSemFalhar(), TestDatabaseLatencyDeltaExigeDuasJanelas(), TestDatabaseLatencyDeltaIgnoraOperaçõesQueFalharam(), TestDatabaseLatencyDeltaIgnoraOscilaçãoNormal() (+15 more)

### Community 57 - "openChangeRepository"
Cohesion: 0.42
Nodes (8): ChangeRepository, NewChangeRepository(), changeSnapshot(), deploymentAt(), openChangeRepository(), TestChangeRepositoryListDeploymentsFiltersOrdersAndLimits(), TestChangeRepositorySaveChangesIsAtomicAndIdempotent(), TestChangeRepositoryValidatesBeforeDatabaseAndRespectsContext()

### Community 59 - "sqlite/diagnosis_repository.go"
Cohesion: 0.31
Nodes (8): findingID(), preparedFinding, DiagnosisRepository, prepareDiagnosis(), rollbackDiagnosisTransaction(), subjectIdentifier(), preparedDiagnosis, preparedFinding

### Community 60 - "SchemaSnapshot"
Cohesion: 0.06
Nodes (72): fonteDeSchemaFake, scopedSchemaReaderFake, SchemaChange, SchemaChangeKind, SchemaObject, SchemaObjectKind, SchemaSnapshot, alteredDetail() (+64 more)

### Community 62 - "InitializeProject"
Cohesion: 0.27
Nodes (11): ensureContext(), EphemeralProjectDir(), InitializeProject(), assertDirectoryExists(), assertFileExists(), TestEphemeralProjectDirCriaForaDoProjeto(), TestEphemeralProjectDirNaoColide(), TestEphemeralProjectDirServeAoInitCompleto() (+3 more)

### Community 63 - "run"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConverteCargaLimitada(), TestLoadConfigRejeitaConcorrenciaExcessiva(), config

### Community 64 - "run"
Cohesion: 0.23
Nodes (10): NewHTTPServer(), RunHTTPServer(), SignalContext(), TestNewHTTPServerConfiguraLimitesEHealth(), TestRunHTTPServerEncerraComContexto(), loadConfig(), main(), run() (+2 more)

### Community 65 - "Deployment"
Cohesion: 0.08
Nodes (28): changeSourceFake, scopedDeploymentReaderFake, Commit, Deployment, ImportRequest, Snapshot, corroboratedProximityFindings(), deploymentsForService() (+20 more)

### Community 66 - "otlp_logs_json.go"
Cohesion: 0.54
Nodes (7): logRecordID(), logSeverity(), normalizeLogRecord(), exportLogsServiceRequest, logRecord, resourceLog, scopeLog

### Community 67 - "io.Writer"
Cohesion: 0.23
Nodes (9): io.Writer, newAuditor(), RenderScopeSummary(), formatIncidentTime(), RenderIncidentList(), RenderPersistedDiagnosis(), TestRenderIncidentListOrdenaEExibeResumo(), TestRenderPersistedDiagnosisExplicaMetadataLegada() (+1 more)

### Community 68 - "ParseOTLPTraces"
Cohesion: 0.20
Nodes (16): testing.F, exigirErroClassificado(), exigirSinaisCoerentes(), FuzzParseOTLPLogsJSONNãoDevolveOCorpo(), FuzzParseOTLPTracesJSON(), FuzzParseOTLPTracesProtobuf(), classifyOTLPError(), OTLPEncoding (+8 more)

### Community 69 - "Fixtures OpenTelemetry"
Cohesion: 0.12
Nodes (17): Módulo telemetry, Allowlist de atributos de Resource, Atributos OpenTelemetry prioritários, Ingestão OTLP, Receiver OTLP HTTP POST /v1/traces, Signal, Detector deployment_proximity, Comando faultmap serve (+9 more)

### Community 70 - "Grafo de evidências"
Cohesion: 0.40
Nodes (5): Módulo evidence, EvidenceEdge, EvidenceNode, Fallback de parentesco de spans, Grafo de evidências

### Community 71 - "Write"
Cohesion: 0.37
Nodes (11): graph(), snapshot(), TestArtefatosConcordamSobreAOrdemDosSuspeitos(), TestWriteExigeDiretorioInformado(), TestWriteFalhaQuandoCaminhoNaoEDiretorio(), TestWriteFalhaQuandoDiretorioNaoExiste(), TestWriteGravaOsCincoArtefatosNoDiretorio(), TestWriteProduzBytesIdenticosEmDuasExecucoes() (+3 more)

### Community 72 - "rodar-ranking.sh"
Cohesion: 0.83
Nodes (3): limpar(), rodar-ranking.sh script, trafego()

### Community 86 - "IngestChanges"
Cohesion: 0.33
Nodes (8): ChangeSource, changeWriterFake, ChangeImportResult, ChangeWriter, IngestChanges(), TestIngestChangesFetchesAndPersistsOneBoundedSnapshot(), TestIngestChangesStopsBeforePersistenceWhenSourceFails(), validChangesRequest()

### Community 87 - "PersistedDiagnosis"
Cohesion: 0.24
Nodes (8): incidentHistoryReaderFake, IncidentSummary, PersistedDiagnosis, Render(), reportDiagnosis(), TestRenderProduzContratoVersionadoEDeterministico(), TestRenderRepresentaBaselineLegadaComoNull(), historicoFake

### Community 88 - "run"
Cohesion: 0.39
Nodes (7): loadConfig(), main(), run(), mapLookup(), TestLoadConfigConvertePoolPostgres(), TestLoadConfigRejeitaIdleMaiorQueOpen(), config

### Community 89 - "Piloto cego"
Cohesion: 0.18
Nodes (13): Detector retry_storm, Critérios de aceite do MVP, Métricas de sucesso, Sistema de demonstração demo-shop, Primeira demonstração obrigatória, Roadmap de dez marcos, Collector do piloto, Exportadores separados para traces e logs (+5 more)

### Community 90 - "postgres/signal_repository.go"
Cohesion: 0.13
Nodes (14): SignalType, database/sql.Rows, marshalFloatMap(), marshalStringMap(), NewSignalRepository(), rollbackSignalTransaction(), scanSignal(), scanSignals() (+6 more)

### Community 91 - "repositorioComRetencao"
Cohesion: 0.42
Nodes (9): coletaDe(), SchemaRepository, repositorioComRetencao(), TestColetaContraLinhaDeBaseEsvaziadaFalhaEmVezDeInventarMigracao(), TestPruneAvancaEmLotes(), TestPruneEhIdempotente(), TestPruneLiberaOCatalogoAntigoSemApagarAsMudancas(), TestPrunePreservaAColetaMaisRecenteDeCadaBase() (+1 more)

### Community 92 - "deploy-inofensivo"
Cohesion: 0.50
Nodes (3): deploy-inofensivo, O que ele verifica, Por que ele existe

### Community 93 - "RenderSuspectExplanation"
Cohesion: 0.36
Nodes (7): provenanceSummary(), RenderSuspectExplanation(), renderSuspectFinding(), explicacaoDeExemplo(), TestRenderizarExplicacaoMostraProvenienciaELimitacoes(), TestRenderizarExplicacaoSemContribuicoesEhExplicita(), TestRenderSuspectExplanationResumeProveniênciaLonga()

### Community 94 - "ParseOTLPLogsJSON"
Cohesion: 0.36
Nodes (7): encoding/json.Decoder, rejectTrailingJSON(), ParseOTLPLogsJSON(), TestParseOTLPLogsNuncaArmazenaOTextoDaMensagem(), TestParseOTLPLogsPreservaSeveridadeECorrelação(), TestParseOTLPLogsRejeitaEnvelopeInválido(), TestParseOTLPLogsÉIdempotentePorRegistro()

### Community 95 - "ADR 0019 — Mudança de catálogo dentro do incidente não é acusada"
Cohesion: 0.33
Nodes (5): ADR 0019 — Mudança de catálogo dentro do incidente não é acusada, Consequências, Contexto, Decisão, O que reabriria a questão

### Community 96 - "snapshotDeExemplo"
Cohesion: 0.60
Nodes (5): snapshotDeExemplo(), TestExplicarSuspeitoAgrupaContribuicoesEvidenciasEProveniencia(), TestExplicarSuspeitoExigeNomeNaoVazio(), TestExplicarSuspeitoIgnoraCaixaEEspacos(), TestExplicarSuspeitoInexistenteDevolveErroDeDominio()

### Community 97 - "postgres-demo"
Cohesion: 0.40
Nodes (4): DATABASE_URI, uvx, postgres-demo, postgres-mcp

### Community 99 - "Finding"
Cohesion: 0.17
Nodes (25): SuspectContribution, ExplainSuspect(), findingsForService(), findSuspect(), SuspectExplanation, groupContributions(), normalizeFinding(), remainingFindings() (+17 more)

### Community 100 - "Sorteio cego na demo-shop"
Cohesion: 0.12
Nodes (15): ADR 0017 — A cauda do banco é uma pergunta própria, não outro percentil, Contexto, Decisão, O que se aceita em troca, Por que não mudar o percentil, Como repetir, O achado sobre o produto, O mecanismo existe, mas não foi confirmado como a causa (+7 more)

### Community 101 - "RenderRanking"
Cohesion: 0.38
Nodes (8): RenderRanking(), rankingSnapshot(), TestRankingPreservaAOrdemDoSnapshot(), TestRenderRankingNormalizaInstanteParaUTC(), TestRenderRankingPreservaOrdemDoSnapshotEOrdenaContribuicoesPorRegra(), TestRenderRankingProduzBytesIdenticosEmDuasExecucoes(), TestRenderRankingSemSuspeitosEscreveListaVaziaENaoNula(), rankingDocument

### Community 102 - "Setup"
Cohesion: 0.80
Nodes (3): Setup(), traceEndpoint(), Config

### Community 103 - "Open"
Cohesion: 0.50
Nodes (3): closeAfterFailure(), Open(), TestOpenConfiguresSQLiteForFaultmap()

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
Cohesion: 0.29
Nodes (12): abrirEmSchemaIsolado(), comSearchPath(), TestConformidadeIntegracaoPostgres(), exigirDSNDeIntegracao(), idsDeCatalogoTravados(), idsRestantes(), idsTravados(), lerIDs() (+4 more)

### Community 112 - "ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada"
Cohesion: 0.12
Nodes (14): ADR 0015 — A retenção libera o catálogo e preserva as mudanças, Consequências, Contexto, Decisão, Adendo — a divergência de concorrência passou a ser coberta, Adendo — a retenção passou a reservar o lote, ADR 0016 — PostgreSQL é backend alternativo, provado por uma bateria compartilhada, Consequências (+6 more)

### Community 113 - "testing.T"
Cohesion: 0.10
Nodes (28): TestConfigValidateExigeIdentidadeCompleta(), TestDisabledMantemShutdownSeguro(), TestTraceEndpointAcrescentaCaminhoOTLP(), testing.T, TestGetIncidentValidaIDEPropagaAusencia(), TestListIncidentsPreservaOrdemDoRepositorio(), TestListIncidentsValidaLimiteAntesDoRepositorio(), TestPersistedDiagnosisMetadataComplete() (+20 more)

### Community 115 - "Signal"
Cohesion: 0.19
Nodes (19): signalStoreFake, traceReaderFake, Telemetria de instrumentação real como base de teste, math/rand.Rand, attributeValueOrEmpty(), databaseFailureTypeSuffix(), databaseNonTimeoutFailures(), isClientCancellation() (+11 more)

### Community 117 - "sortear-incidente.sh"
Cohesion: 0.42
Nodes (7): base(), carga(), diagnosticar(), falha(), revelar(), sortear-incidente.sh script, sortear()

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
- **84 isolated node(s):** `uvx`, `postgres-mcp`, `DATABASE_URI`, `Request`, `results` (+79 more)
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
- **Why does `Signal` connect `Signal` to `Rank`, `graph.go`, `attributes.go`, `NewSignalRepository`, `DetectSchemaChangeProximity`, `DetectTraceBreak`, `detectors.go`, `context.Context`, `diagnose_scope.go`, `conformidade.go`, `otlp_json.go`, `DiagnoseIncidentInScope`, `DetectDatabaseError`, `DetectRetryStorm`, `DetectVersionRegression`, `InvestigationWindow`, `ParseOTLPJSON`, `DetectLogCorrelation`, `filterDatabaseSignals`, `DetectDatabaseLatencyDelta`, `Deployment`, `otlp_logs_json.go`, `ParseOTLPTraces`, `postgres/signal_repository.go`, `ParseOTLPLogsJSON`, `testSignal`?**
  _High betweenness centrality (0.101) - this node is a cross-community bridge._
- **Why does `Finding` connect `Finding` to `Rank`, `DetectSchemaChangeProximity`, `DetectTraceBreak`, `detectors.go`, `openDiagnosisRepository`, `IncidentHistoryReader`, `conformidade.go`, `database/sql.Tx`, `Render`, `DetectDatabaseError`, `DetectRetryStorm`, `DetectVersionRegression`, `json/report.go`, `InvestigationWindow`, `timeline.go`, `DetectLogCorrelation`, `DetectDatabaseLatencyDelta`, `sqlite/diagnosis_repository.go`, `Deployment`, `PersistedDiagnosis`, `RenderSuspectExplanation`, `testing.T`?**
  _High betweenness centrality (0.031) - this node is a cross-community bridge._
- **Why does `DiagnoseIncidentInScope()` connect `DiagnoseIncidentInScope` to `Deployment`, `Load`, `Rank`, `graph.go`, `DetectTraceBreak`, `detectors.go`, `context.Context`, `diagnose_scope.go`, `conformidade.go`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._