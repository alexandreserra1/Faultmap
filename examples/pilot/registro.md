# Registro do investigador — incidente `pilot-__`

Preencha **antes** de saber qual falha foi injetada. Este formulário é o
instrumento de medida do piloto: respondido depois da revelação, ele só confirma
o que já acreditávamos.

Copie o arquivo para `pilot-01.md`, `pilot-02.md` e assim por diante.

## Identificação

```text
Incidente:
Serviço investigado:
Janela de incidente (--since):
Janela de baseline (--baseline):
--until usado:
ID do diagnóstico (inc_...):
```

## Antes de olhar as evidências

Responda só com o ranking à vista, sem abrir `explain` nem `blame`.

```text
Quem é o top-1?
Você acredita nele? Por quê?
```

## Depois de ler as evidências

```text
A explicação diz o que aconteceu, ou só que algo mudou?
Qual evidência foi a mais útil?
Qual evidência foi ruído?
Por onde você começaria a investigar de verdade?
Alguma acusação pareceu injusta com um serviço que é vítima e não causa?
Faltou alguma informação que você esperava encontrar?
```

## Medidas

```text
Tempo até você formar uma primeira hipótese:
Tempo que o comando levou (linha "Tempo de diagnóstico"):
Quantidade de suspeitos listados:
```

## Sua hipótese final

Escreva antes da revelação. Vale palpite; o objetivo é registrar o que o produto
levou você a pensar, não acertar.

```text
O que você acha que quebrou:
Grau de confiança (chute / provável / tenho certeza):
```
