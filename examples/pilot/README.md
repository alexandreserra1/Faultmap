# Piloto cego

O Faultmap passa em todos os cenários automatizados. Isso significa que ele
acerta os incidentes que **nós** desenhamos, o que é bem menos do que parece.

Este piloto mede outra coisa: se o produto ajuda **uma pessoa que não sabe a
resposta**. Por isso é cego — quem investiga não sabe qual falha foi injetada, e
registra a hipótese antes da revelação.

## Papéis

- **Operador**: escolhe e injeta a falha, e guarda a verdade em segredo.
- **Investigador**: usa o Faultmap sem saber o que foi quebrado.

Se você tem duas pessoas, o protocolo abaixo vale como está. Com uma pessoa só,
não há piloto: quem injeta não pode investigar. A adaptação possível é o operador
ser um assistente ou colega que só precisa saber rodar um comando.

## O que é preciso

- Ambiente de **staging** — nunca injete falhas em produção.
- Um serviço HTTP com pelo menos uma dependência, de preferência PostgreSQL.
- Tráfego realista ou um gerador de carga.
- Deploys registrados no GitHub, se quiser exercitar a correlação com commits.

## 1. Compilar a versão sob teste

```bash
mkdir -p ./bin
go build -trimpath -o ./bin/faultmap ./cmd/faultmap
./bin/faultmap --help
```

Use o **mesmo binário** durante todo o experimento. Trocar de versão no meio
mistura resultados de produtos diferentes.

## 2. Criar o workspace

```bash
./bin/faultmap init --directory ./faultmap-pilot
```

Isso cria `faultmap.yaml`, `faultmap.db` e o diretório `faultmap-out/`. Não
reutilize o banco da demo-shop: telemetria sintética misturada à real invalida a
baseline.

Copie [`faultmap.yaml`](faultmap.yaml) sobre o arquivo gerado e ajuste o
repositório.

> **Caminhos são relativos ao arquivo de config, não ao diretório onde você roda
> o comando.** O valor certo em `storage.path` é `./faultmap.db`, mesmo
> executando da raiz do projeto. Escrever `./faultmap-pilot/faultmap.db` aponta
> para um banco aninhado e vazio.

> **`privacy.blocked_attributes` soma aos padrões**, não os substitui. Declarar
> o vocabulário sensível do seu negócio acrescenta proteção; SQL bruto e caminho
> de arquivo seguem bloqueados.

## 3. Iniciar o Faultmap

```bash
./bin/faultmap serve --config ./faultmap-pilot/faultmap.yaml
```

Em outro terminal:

```bash
curl --fail http://127.0.0.1:8081/health   # {"status":"ok"}
```

**Deixe o terminal do `serve` visível.** É onde aparecem os lotes recusados —
inclusive o motivo, que a resposta HTTP não pode conter.

## 4. Identidade OpenTelemetry na aplicação

```bash
export OTEL_SERVICE_NAME=checkout-service
export OTEL_RESOURCE_ATTRIBUTES="service.version=${GITHUB_SHA},deployment.environment.name=staging"
export OTEL_EXPORTER_OTLP_ENDPOINT="http://seu-collector:4318"
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"
```

`service.version` deve ser o **SHA completo** do commit implantado — é o que liga
a telemetria ao commit importado do GitHub. E o contexto de trace precisa ser
propagado entre os serviços, senão não há grafo para analisar.

## 5. Encaminhar o Collector

Use [`collector.yaml`](collector.yaml). O detalhe que custa caro descobrir
sozinho: **traces e logs precisam de exportadores separados**, porque o Faultmap
aceita logs apenas em JSON. Com um único exportador em protobuf os traces entram,
os logs tomam `400` e desaparecem — e o piloto concluiria, erradamente, que o
detector de logs não funciona.

## 6. Confirmar que a telemetria chega

Faça algumas requisições saudáveis, espere o batch e verifique:

```bash
sleep 2
./bin/faultmap telemetry list \
  --config ./faultmap-pilot/faultmap.yaml \
  --service checkout-service --since 10m --limit 20
```

Antes de seguir, confirme: nome do serviço, trace IDs presentes, status HTTP,
duração, spans de banco, `service.version` correta e nenhuma informação sensível
na saída.

**Se a lista estiver vazia, pare.** Um piloto sobre telemetria ausente mede o
Collector, não o Faultmap.

## 7. Coletar a baseline

Deixe o tráfego normal correr por **20 a 30 minutos**, sem mudar configuração
nenhuma. Registre:

```text
Início da baseline:
Fim da baseline:
Versão implantada:
Volume aproximado:
Taxa de erro esperada:
Latência normal:
```

A baseline é metade do diagnóstico: todo detector compara janelas, e nenhum
reporta estado absoluto.

## 8. Importar commits e deployments

```bash
export GITHUB_TOKEN="seu-token"      # só como variável de ambiente

./bin/faultmap ingest github \
  --config ./faultmap-pilot/faultmap.yaml \
  --repo sua-organizacao/seu-repositorio \
  --commits --deployments \
  --service checkout-service --environment staging \
  --since 168h --limit 100
```

Se o seu processo de deploy não cria deployments na API do GitHub, o Faultmap
observa as versões pela telemetria mas perde a evidência temporal — e o piloto
**não valida** essa correlação. Registre isso no resultado em vez de contornar.

## 9. Injetar a falha (operador)

Registre a verdade **em segredo**, fora do alcance do investigador:

```text
Falha real:
Serviço alterado:
Commit:
Horário da mudança:
Horário do primeiro erro:
Comportamento esperado:
```

Escolha **uma** falha por incidente:

- reduzir timeout HTTP;
- adicionar atraso controlado no banco;
- diminuir o pool de conexões;
- retornar HTTP 500 em uma rota;
- aumentar retries;
- manter um lock curto;
- implantar uma versão com regressão conhecida.

Ao implantar uma versão nova, mude `service.version` para outro SHA — é o que
permite comparar as duas versões.

## 10. Gerar o tráfego do incidente

Use volume semelhante ao da baseline; volumes diferentes viram, eles mesmos, uma
diferença entre as janelas.

```bash
for numero in {1..20}; do
  curl --silent --output /dev/null \
    --write-out "requisicao=${numero} status=%{http_code}\n" \
    --max-time 5 https://staging.exemplo.com/checkout
done
sleep 5   # espera o batch do Collector
```

Anote o horário exato do fim.

## 11. Diagnosticar (investigador)

```bash
time ./bin/faultmap diagnose incident \
  --config ./faultmap-pilot/faultmap.yaml \
  --service checkout-service --environment staging \
  --since 10m --baseline 30m \
  --until 2026-08-16T18:30:00Z \
  --limit 500
```

`--until` aceita **RFC 3339 estrito** e torna o experimento reproduzível: sem
ele, a janela anda a cada execução e dois diagnósticos do mesmo incidente deixam
de ser comparáveis. Guarde o `inc_...` impresso.

## 12. Registrar a hipótese — antes da revelação

Preencha [`registro.md`](registro.md). **Este é o passo que faz o piloto valer
alguma coisa**; respondê-lo depois de saber a resposta transforma a medida em
confirmação.

Ferramentas para investigar:

```bash
./bin/faultmap incident show   --config ./faultmap-pilot/faultmap.yaml --id inc_...
./bin/faultmap explain suspect checkout-service \
                               --config ./faultmap-pilot/faultmap.yaml --incident inc_...
./bin/faultmap blame trace     --config ./faultmap-pilot/faultmap.yaml --trace TRACE_ID --limit 100
```

## 13. Exportar os artefatos

```bash
mkdir -p ./faultmap-pilot/faultmap-out/pilot-01   # o diretório precisa existir
./bin/faultmap export artifacts \
  --config ./faultmap-pilot/faultmap.yaml \
  --incident inc_... \
  --output ./faultmap-pilot/faultmap-out/pilot-01
```

Devem aparecer `report.md`, `ranking.json`, `evidence-graph.mmd`,
`incident-summary.json` e `timeline.json`. Guarde-os junto do registro humano.

## 14. Revelar e comparar

Só agora o operador conta o que fez. Preencha [`resultado.md`](resultado.md).

## 15. Aplicar a retenção

```bash
./bin/faultmap retention apply --config ./faultmap-pilot/faultmap.yaml
```

Remove telemetria expirada em lotes e preserva os snapshots de diagnóstico. Logs
têm volume muito maior que traces; se você os ingere, rode isto com regularidade.

## Sequência recomendada

1. três incidentes controlados em staging;
2. três incidentes históricos reproduzidos;
3. uma semana apenas observando staging;
4. o primeiro incidente **inesperado** em staging;
5. só então, observação passiva em produção.

Em produção não se injeta falha. Use apenas incidentes naturais, e mantenha
Collector e Faultmap em rede privada — o receiver não tem autenticação nem TLS.

O passo 4 vale mais que os três primeiros somados: é o único em que ninguém
sabia a resposta de antemão.
