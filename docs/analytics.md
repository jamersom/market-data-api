# Cálculos compartilhados

[Instruções para agentes](../AGENTS.md) · [Guia de desenvolvimento](desenvolvimento.md)

O pacote [internal/domain/analytics](../internal/domain/analytics) contém funções puras, sem HTTP, banco ou IA.
Os indicadores abaixo estão implementados e expostos por `/assets/{ticker}/intelligence`.

## Regras financeiras compartilhadas

- Reutilize este pacote para fórmulas financeiras compartilhadas; não duplique
  cálculos em handlers ou serviços individuais. As funções recebem séries já
  validadas, sem consultar dados nem formatar respostas HTTP.
- Mantenha os cálculos determinísticos. Não use LLM para calcular ou preencher
  valores financeiros ausentes e não crie preços sintéticos para pregões ausentes.
  Preserve a representação explícita de indisponibilidade ou insuficiência de dados.
- Mantenha preços e valores monetários armazenados em centavos inteiros, sem
  ponto flutuante. Resultados calculados preservam a precisão prevista para cada
  fórmula, incluindo frações de centavo da SMA. Na saída HTTP, formate valores
  monetários como strings decimais conforme o contrato de cada campo.
- Percentuais usam unidades percentuais: `20` significa 20%, não `0.20`.
  Arredonde somente no mapper de resposta quando previsto no contrato.
- Use ponteiros nos resultados opcionais quando zero for válido e precisar ser
  distinguido de ausência. Falta de dados não equivale a indicador zero.
- As funções não alteram os slices recebidos. Preserve também slices e mapas do
  chamador quando o código existente os copia antes de ordenar ou normalizar.

## Indicadores de análise individual

| Função | Resultado | Histórico mínimo |
|---|---|---|
| `SMA(closes, 20)` | Média dos últimos 20 fechamentos, em centavos com parte fracionária | 20 fechamentos |
| `DistanceFromSMA(closes, 20)` | Distância percentual do último fechamento para a média não arredondada | 20 fechamentos |
| `RSI(closes, 14)` | RSI de Wilder, na escala 0 a 100 | 15 fechamentos |
| `RSIHistory(closes, 14)` | Todos os RSI válidos com a mesma semente de Wilder | 15 fechamentos |
| `PercentileMidrank(value, observations)` | Percentil com meio peso para observações iguais | 1 observação |
| `CurrentDrawdown(closes, 252)` | Variação percentual do último fechamento contra o pico dos últimos 252 fechamentos | 252 fechamentos |

Os períodos são parâmetros para permitir reutilização e testes. As janelas
padronizadas são definidas pelo caso de uso atual. Os preços de entrada são
inteiros em centavos, positivos e em ordem cronológica. As funções retornam
`(valor, ok)`; `ok=false` indica período inválido, histórico insuficiente ou preço
inválido na série utilizada. Não interpretar esse retorno como indicador zero.

SMA, distância e drawdown usam apenas a janela final. RSI utiliza toda a série
fornecida: inicializa as médias com os primeiros 14 ganhos/perdas e aplica a
suavização de Wilder aos seguintes. Série constante produz 50, somente ganhos
produzem 100 e somente perdas produzem 0. O caso de uso utiliza o início do histórico publicado como semente; fornecer
apenas os últimos 15 fechamentos reinicializa o RSI a cada consulta.

`PercentileMidrank` é independente da duração da janela. O caso de uso seleciona
os RSI históricos pelas datas, exclui a referência e aplica a função à população
válida. Valores `NaN` ou infinitos tornam o cálculo inválido.

O drawdown atual difere do máximo: após recuperação para um novo pico, o atual
é zero, mas a maior queda anterior continua registrada no drawdown máximo.
O atual implementado aqui é relativo à janela, não ao histórico inteiro do ativo.

A SMA evita overflow na soma e preserva frações de centavo.

## Responsabilidades do serviço

Os slices não contêm datas. O serviço valida ordenação, unicidade,
calendário de pregões, lacunas, moeda, ajuste dos preços e limite `asOf` antes de
chamar as funções. Ter 252 observações não comprova, por si só, 252 pregões
consecutivos. O serviço também converte indisponibilidade em metadados e aplica a política de inicialização do RSI descrita acima.

Retorno, volatilidade anualizada, drawdown máximo e volume médio já são
reutilizados por `/comparisons`, conforme [comparison-application.md](comparison-application.md).
O percentil do RSI é exposto somente pelo Asset Intelligence. O rules evaluator
consome preço, RSI e médias já calculados; ele não pertence a este pacote e não
reimplementa as fórmulas quantitativas.

## Validação

[indicators_test.go](../internal/domain/analytics/indicators_test.go) cobre valores conhecidos, suavização após a semente do RSI,
séries constantes e direcionais, limites de histórico, períodos e preços
inválidos, janela final, preservação dos dados e soma de preços acima de `int64`.

```sh
go test ./internal/domain/analytics
```

Para a validação completa e as condições de execução, siga o
[checklist de validação](operations/validation-checklist.md).
