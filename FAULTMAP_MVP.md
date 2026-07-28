# Faultmap — Especificação do MVP

## Leitura obrigatória

Este arquivo é o **índice normativo** da especificação do MVP; ele não é suficiente isoladamente. Antes de desenhar, implementar, revisar ou aprovar qualquer parte do Faultmap, é obrigatório ler integralmente, além deste arquivo, todos os documentos abaixo. Os links fazem parte da especificação e nenhum deles é opcional.

1. [Produto e arquitetura](docs/mvp/01-produto-e-arquitetura.md)
2. [Domínio, dados e telemetria](docs/mvp/02-dominio-dados-e-telemetria.md)
3. [Diagnóstico, integrações e saídas](docs/mvp/03-diagnostico-integracoes-e-saidas.md)
4. [Operação, qualidade e escopo](docs/mvp/04-operacao-qualidade-e-escopo.md)
5. [Entrega e diretrizes de implementação](docs/mvp/05-entrega-e-diretrizes.md)

A leitura deve ocorrer nesta ordem. Em caso de aparente conflito, este índice estabelece apenas a obrigatoriedade e a ordem de leitura; o documento temático responsável pelo assunto é a fonte normativa.

## Resumo executivo

O Faultmap MVP é um monólito modular em Go, distribuído como binário único e operado localmente. Ele ingere telemetria OpenTelemetry, compara a janela de um incidente com uma baseline, correlaciona sinais a deploys, commits e operações PostgreSQL, constrói um grafo de evidências e retorna um ranking determinístico, explicável e auditável de suspeitos.

O objetivo é reduzir o tempo para que uma pessoa desenvolvedora descubra onde iniciar a investigação e por quê — não monitorar tudo nem alegar causalidade sem evidências.

## Mapa de assuntos

| Assunto | Documento normativo |
| --- | --- |
| Visão, princípios, arquitetura, stack, módulos e dependências | [01 — Produto e arquitetura](docs/mvp/01-produto-e-arquitetura.md) |
| Modelo de domínio, grafo, SQLite, OTLP e janelas | [02 — Domínio, dados e telemetria](docs/mvp/02-dominio-dados-e-telemetria.md) |
| Detectores, ranking, GitHub, PostgreSQL, CLI e artefatos | [03 — Diagnóstico, integrações e saídas](docs/mvp/03-diagnostico-integracoes-e-saidas.md) |
| Demo, aceite, métricas, privacidade, configuração, testes e escopo | [04 — Operação, qualidade e escopo](docs/mvp/04-operacao-qualidade-e-escopo.md) |
| Marcos, primeira demonstração e regras de implementação | [05 — Entrega e diretrizes](docs/mvp/05-entrega-e-diretrizes.md) |
