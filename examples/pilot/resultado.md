# Resultado do piloto cego

Preenchido após a revelação de cada incidente. O valor deste documento está nas
respostas escritas do investigador, não na taxa de acerto: três incidentes são
amostra pequena demais para estatística.

## Por incidente

Repita o bloco para cada um.

```text
Incidente: pilot-01
Causa real:
Top-1 retornado:
Top-3 retornado:
Causa apareceu no top-1: sim/não
Causa apareceu no top-3: sim/não
Tempo do diagnóstico:
Tempo humano até a primeira hipótese:
Hipótese humana estava certa: sim/não/parcial
Falsos positivos:
Evidências sem proveniência:
Dados sensíveis encontrados:
Faultmap indicou um bom ponto inicial: sim/não
Comentários do investigador:
```

## Critérios mínimos

```text
Top-1 ≥ 60%           (com três incidentes: 2 de 3)
Top-3 ≥ 80%
Diagnóstico < 10 s
Proveniência = 100%
Nenhum dado sensível armazenado
```

## Cobertura dos detectores

Marcar quais dispararam em telemetria real durante o piloto. **A coluna mais
importante é a dos que nunca dispararam**: um detector que nunca fala em uso real
é código não exercitado, e é honesto dizer isso em vez de contá-lo como pronto.

| Detector | Disparou | Foi útil |
| --- | --- | --- |
| `error_rate_delta` | | |
| `latency_delta` | | |
| `database_timeout` | | |
| `database_error` | | |
| `database_latency_delta` | | |
| `database_http_trace_correlation` | | |
| `retry_storm` | | |
| `deployment_proximity` | | |
| `version_regression` | | |
| `dependency_failure` | | |
| `trace_break` | | |
| `log_correlation` | | |

## O que este piloto NÃO valida

Registrar aqui as limitações, para que o resultado não seja lido como mais amplo
do que é.

```text
Correlação com GitHub: só é exercitada se a aplicação registrar deployments na
API. Sem isso, deployment_proximity trabalha apenas com as versões observadas na
telemetria, sem a evidência temporal do deploy.

Ambiente: um piloto em máquina local não reproduz latência de rede, concorrência
entre serviços nem volume de produção.

Amostra: três incidentes desenhados por quem conhece o produto. Um incidente
inesperado vale mais que os três.
```

## Conclusão

```text
O ranking ajudou um humano que não sabia a resposta: sim/não/parcial
O que mudar no produto por causa deste piloto:
O que medir no próximo:
```
