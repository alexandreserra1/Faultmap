"""Ponto de entrada que instrumenta o DuckDB antes de carregar a aplicação.

Fica fora do repositório do StriderEdge: o uvicorn aponta para este módulo, que
liga a instrumentação DBAPI e só então importa o app real. Um sitecustomize não
serviria — o opentelemetry-instrument instala o seu próprio e o Python carrega
apenas um deles.

Este módulo também hospeda a injeção de falha do piloto cego. Ela vive aqui, e
não no repositório do StriderEdge, porque a aplicação sob teste não pode ser
modificada para ser testada. Tudo é ligado por variável de ambiente e some
quando ela é removida.

REGRA QUE NÃO SE QUEBRA: nenhuma falha escreve no banco. O StriderEdge tem
usuários reais, e um piloto que corrompesse dados para medir um diagnóstico não
teria medido nada que valha o custo.
"""
import asyncio
import os
import random
import time

import duckdb
from opentelemetry.instrumentation import dbapi

# A conexão do StriderEdge é criada de forma preguiçosa, no lifespan, portanto
# depois deste ponto: o patch alcança a conexão que a aplicação vai usar.
dbapi.trace_integration(duckdb, "connect", "duckdb")

from api.main import app as _app  # noqa: E402  (precisa vir depois do patch)


def _fault_file():
    """Arquivo consultado a cada requisição para saber se a falha está ligada.

    Ler de um arquivo, em vez de uma variável de ambiente, evita reiniciar a
    aplicação entre a baseline e o incidente. O reinício era a parte frágil do
    roteiro — travava esperando o processo antigo morrer — e também partia a
    janela de tempo em duas, com um intervalo morto no meio.

    O caminho não aparece em log de acesso, então ligar a falha não revela qual
    é: o piloto continua cego para quem investiga.
    """
    return os.environ.get(
        "FAULTMAP_PILOT_FAULT_FILE",
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "falha-atual.txt"),
    )


def _current_fault():
    try:
        with open(_fault_file()) as handle:
            return _parse_fault(handle.read().strip())
    except (OSError, ValueError):
        return None


def _parse_fault(raw):
    """Interpreta FAULTMAP_PILOT_FAULT no formato `tipo:alvo:valor`.

    O alvo é um trecho do caminho, casado por substring: rotas reais têm
    identificadores no meio da URL e um casamento exato erraria quase sempre.
    """
    if not raw:
        return None
    parts = raw.split(":")
    if len(parts) != 3:
        raise ValueError(
            "FAULTMAP_PILOT_FAULT deve ter o formato tipo:alvo:valor, "
            f"recebido {raw!r}"
        )
    kind, target, value = parts[0].strip(), parts[1].strip(), parts[2].strip()
    if kind not in {"latency", "error500", "db_slow"}:
        raise ValueError(f"tipo de falha desconhecido: {kind!r}")
    return {"kind": kind, "target": target, "value": float(value)}


# Falha inválida no arquivo é tratada como ausência, para que um erro de escrita
# não derrube a aplicação no meio de um incidente. Aqui, na inicialização, ela é
# validada de imediato: FAULTMAP_PILOT_FAULT mal escrita deve falhar alto.
_parse_fault(os.environ.get("FAULTMAP_PILOT_FAULT"))


class _FaultInjector:
    """Middleware ASGI que atrasa ou falha requisições de uma rota.

    Fica por fora da instrumentação OpenTelemetry, então o span do servidor já
    está aberto quando o atraso acontece: é assim que a telemetria enxerga o
    efeito do mesmo jeito que enxergaria uma dependência lenta de verdade.

    A falha é lida do arquivo a cada requisição, então ligar e desligar não
    requer reiniciar a aplicação.
    """

    def __init__(self, application):
        self._application = application

    async def __call__(self, scope, receive, send):
        fault = _current_fault()
        if (
            fault is None
            or scope["type"] != "http"
            or fault["target"] not in scope.get("path", "")
        ):
            await self._application(scope, receive, send)
            return

        kind, value = fault["kind"], fault["value"]
        if kind == "latency":
            await asyncio.sleep(value / 1000.0)
        elif kind == "error500" and random.random() < value:
            # A resposta é produzida aqui, sem chegar à aplicação: o objetivo é
            # o sintoma observável, não exercitar um caminho de erro real.
            await send({
                "type": "http.response.start",
                "status": 500,
                "headers": [(b"content-type", b"application/json")],
            })
            await send({"type": "http.response.body", "body": b'{"detail":"erro interno"}'})
            return
        await self._application(scope, receive, send)


def _install_database_delay():
    """Atrasa cada consulta quando a falha ativa for db_slow.

    O patch é no execute da DBAPI instrumentada, então o atraso cai dentro do
    span de banco — que é exatamente onde um banco lento apareceria. Ele é
    instalado sempre, e consulta o arquivo em tempo de execução: assim o mesmo
    processo serve a baseline e o incidente.
    """
    original_execute = duckdb.DuckDBPyConnection.execute

    def execute_with_optional_delay(self, *args, **kwargs):
        fault = _current_fault()
        if fault is not None and fault["kind"] == "db_slow":
            time.sleep(fault["value"] / 1000.0)
        return original_execute(self, *args, **kwargs)

    duckdb.DuckDBPyConnection.execute = execute_with_optional_delay


_install_database_delay()
app = _FaultInjector(_app)

__all__ = ["app"]
