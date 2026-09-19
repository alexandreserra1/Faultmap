#!/usr/bin/env python3
"""Analisa as coletas saudáveis produzidas por medir-ruido-do-banco.sh.

Responde uma pergunta só: com o sistema comprovadamente saudável, o critério de
`database_latency_delta` dispararia? O critério é o do produto — aumento de pelo
menos 2 ms no p95 e pelo menos o dobro da baseline — e está repetido aqui de
propósito, para que a medição não dependa de importar o código que ela julga.
"""
import datetime
import glob
import json
import math
import os
import re
import sqlite3
import sys

PISO_MS = 2.0      # minimumDatabaseLatencyDeltaMilliseconds
RAZAO_MINIMA = 1.0  # minimumDatabaseLatencyRatio


def percentil(valores, q):
    ordenados = sorted(valores)
    return ordenados[max(0, math.ceil(len(ordenados) * q) - 1)]


def instante(texto):
    # O SQLite guarda "2026-09-19 22:18:53.123456789 +0000 UTC".
    return datetime.datetime.strptime(
        re.match(r"(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})", texto).group(1),
        "%Y-%m-%d %H:%M:%S",
    )


def operações_de_banco(caminho):
    """Só as que concluíram: é o que o detector mede."""
    conexão = sqlite3.connect(caminho)
    consulta = """SELECT timestamp, attributes_json, measurements_json FROM signals
                  WHERE json_extract(attributes_json,'$."db.system.name"') IS NOT NULL"""
    saída = []
    for carimbo, atributos, medidas in conexão.execute(consulta):
        atributos, medidas = json.loads(atributos), json.loads(medidas)
        if atributos.get("otel.status_code") == "STATUS_CODE_ERROR" or atributos.get("error.type"):
            continue
        if medidas.get("duration_ms") is None:
            continue
        saída.append((instante(carimbo), float(medidas["duration_ms"])))
    conexão.close()
    return sorted(saída)


def janelas(caminho):
    resultado = {}
    for linha in open(caminho, encoding="utf-8"):
        chave, _, valor = linha.strip().partition("=")
        resultado[chave] = datetime.datetime.strptime(valor, "%Y-%m-%dT%H:%M:%SZ")
    return resultado


def main(diretório):
    pares = []
    for banco in sorted(glob.glob(os.path.join(diretório, "db-*.db"))):
        rodada = os.path.basename(banco)[3:-3]
        arquivo_janelas = os.path.join(diretório, f"janelas-{rodada}.txt")
        if not os.path.exists(arquivo_janelas):
            continue
        janela, operações = janelas(arquivo_janelas), operações_de_banco(banco)
        baseline = [d for t, d in operações if janela["INICIO_BASELINE"] <= t < janela["INICIO_INCIDENTE"]]
        incidente = [d for t, d in operações if janela["INICIO_INCIDENTE"] <= t <= janela["FIM"]]
        if len(baseline) >= 5 and len(incidente) >= 5:
            pares.append((rodada, baseline, incidente))

    if not pares:
        print("Nenhuma rodada utilizável: a coleta falhou ou as janelas não têm volume.")
        return 1

    print(f"\n{len(pares)} janela(s) saudáveis medidas\n")
    print("rodada  baseP95  baseMax   incP95        Δ   razão   dispara?")
    disparos = 0
    for rodada, baseline, incidente in pares:
        base_p95, base_max = percentil(baseline, 0.95), max(baseline)
        inc_p95 = percentil(incidente, 0.95)
        delta = inc_p95 - base_p95
        razão = delta / base_p95 if base_p95 > 0 else float("inf")
        dispara = delta >= PISO_MS and razão >= RAZAO_MINIMA
        disparos += dispara
        print("%6s %8.2f %8.2f %8.2f %8.2f %7.2f   %s" % (
            rodada, base_p95, base_max, inc_p95, delta, razão, "SIM" if dispara else "não"))

    print()
    print(f"Falsos positivos com o critério atual: {disparos} de {len(pares)}")
    print("Maior p95 de incidente observado com o sistema saudável: %.2f ms"
          % max(percentil(i, 0.95) for _, _, i in pares))
    # O aquecimento é o mecanismo que torna um falso positivo possível: ele
    # aparece nos primeiros segundos de vida do processo e some depois. Só vira
    # acusação se calhar de cair na janela do incidente e não na da baseline —
    # por isso é medido sobre toda a telemetria, e não sobre as janelas, que
    # começam depois da subida.
    print("\nPerfil de aquecimento, em baldes de 8 s desde o primeiro span:")
    for banco in sorted(glob.glob(os.path.join(diretório, "db-*.db")))[:1]:
        operações = operações_de_banco(banco)
        if not operações:
            break
        início = operações[0][0]
        baldes = {}
        for momento, duração in operações:
            baldes.setdefault(int((momento - início).total_seconds() // 8), []).append(duração)
        for índice in sorted(baldes)[:6]:
            amostra = baldes[índice]
            if len(amostra) < 5:
                continue
            print("  %3d-%3d s  n=%3d  p95 %7.2f ms" % (índice * 8, índice * 8 + 8, len(amostra),
                                                        percentil(amostra, 0.95)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else "."))
