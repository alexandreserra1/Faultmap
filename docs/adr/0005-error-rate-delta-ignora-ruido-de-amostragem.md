# ADR 0005 — `error_rate_delta` ignora variação de amostragem

- Status: aceito
- Data: 2026-08-07

## Contexto

O detector acusava regressão sempre que a taxa de erro do incidente fosse maior
que a da baseline, por qualquer margem:

```go
if delta <= 0 {
    return Finding{}, false
}
```

Um cenário de teste com 25% de erro permanente expôs o problema. A baseline
amostrou 3 falhas em 16 requisições (18,75%) e o incidente, 4 em 16 (25%). O
sistema não mudou — apenas caíram requisições diferentes em cada janela. Ainda
assim o Faultmap apresentou "taxa de erro aumentou de 18,75% para 25,00%" como
evidência, com confiança alta.

Duas janelas de um sistema estável quase nunca produzem taxas idênticas.
Comparar as taxas diretamente transforma essa variação natural em hipótese.

## Decisão

O aumento precisa superar duas barreiras para virar finding:

1. um piso absoluto de 2 pontos percentuais, que impede alarme sobre diferenças
   irrelevantes na operação apenas porque o volume é grande o bastante para
   torná-las mensuráveis;
2. o dobro do erro padrão da diferença entre as duas proporções, que estima
   quanta variação seria esperada só pelo tamanho das amostras.

O cálculo é aritmético e não usa sorteio nem reamostragem, então o mesmo par de
janelas sempre produz a mesma decisão e o diagnóstico continua reproduzível.

## Consequências

- Ruído deixa de aparecer como evidência. Num serviço com falha intermitente
  crônica, o Faultmap se cala em vez de acusar todo diagnóstico.
- A troca é deliberada e conservadora: uma regressão real, pequena e em volume
  baixo pode passar despercebida. Preferimos perder um sinal fraco a apresentar
  ruído como hipótese, porque um produto de diagnóstico que sempre encontra um
  culpado é indistinguível de um que adivinha.
- Os seis cenários da matriz E2E continuam passando: todos têm aumento de 0%
  para 100%, muito além das duas barreiras.
- `latency_delta` ainda não tem proteção equivalente. Os cenários executados não
  produziram falso positivo de latência, então nenhuma mudança foi feita ali sem
  evidência que a justifique.
