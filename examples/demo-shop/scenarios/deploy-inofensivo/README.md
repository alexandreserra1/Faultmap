# deploy-inofensivo

Um deploy aconteceu há um minuto, o commit implantado corresponde à
`service.version` observada nos spans, e **nada quebrou**.

É o cenário `timeout-after-deploy` sem o defeito injetado: mesma proximidade
temporal, mesma correspondência de versão, sistema saudável.

## O que ele verifica

Que `deployment_proximity` **não aparece sozinho**. Proximidade de mudança não é
sintoma — ela não mede nada do serviço, apenas constata que algo mudou por perto.
Apresentá-la sem nenhuma evidência medida ao lado seria acusar quem está bem.

O cenário não exige silêncio absoluto. Estes cenários medem processos reais, e
numa máquina ocupada o aquecimento aparece como regressão de latência de poucos
milissegundos — legítima e medida. Com sintoma presente, apresentar o deploy
passa a ser o comportamento correto.

## Por que ele existe

Nenhum dos cinco cenários originais do modo difícil ingeria deployments, então
`deployment_proximity` passava por todos sem ser executado uma única vez. O
`sem-culpado` se declara "a rede de proteção de todo detector novo" e não
protegia esta regra. `migracao-inofensiva` fechou a metade do schema; este fecha
a do deploy.
