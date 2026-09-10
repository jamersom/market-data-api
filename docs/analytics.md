# Cálculos compartilhados

O pacote `internal/domain/analytics` contém funções puras, sem HTTP, banco ou IA.
Os indicadores abaixo estão implementados e expostos por `/assets/{ticker}/intelligence`.

## Indicadores de análise individual

| Função | Resultado | Histórico mínimo |
|---|---|---|
| `SMA(closes, 20)` | Média dos últimos 20 fechamentos, em centavos com parte fracionária | 20 fechamentos |
| `DistanceFromSMA(closes, 20)` | Distância percentual do último fechamento para a média não arredondada | 20 fechamentos |
| `RSI(closes, 14)` | RSI de Wilder, na escala 0 a 100 | 15 fechamentos |
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

O drawdown atual difere do máximo: após recuperação para um novo pico, o atual
é zero, mas a maior queda anterior continua registrada no drawdown máximo.
O atual implementado aqui é relativo à janela, não ao histórico inteiro do ativo.

As funções não alteram os slices recebidos. A SMA evita overflow na soma e
preserva frações de centavo. Nenhuma função arredonda percentuais para exibição.
Por exemplo, `20` significa 20%, não 0,20.

## Responsabilidades do serviço

Os slices não contêm datas. O serviço valida ordenação, unicidade,
calendário de pregões, lacunas, moeda, ajuste dos preços e limite `asOf` antes de
chamar as funções. Ter 252 observações não comprova, por si só, 252 pregões
consecutivos. O serviço também converte indisponibilidade em metadados e aplica a política de inicialização do RSI descrita acima.

Retorno, volatilidade anualizada, drawdown máximo e volume médio já são
reutilizados por `/comparisons`, conforme [comparison-application.md](comparison-application.md).
Esta etapa não muda seu contrato nem adiciona percentis, benchmark ou regras de
sinais e score.

## Validação

`indicators_test.go` cobre valores conhecidos, suavização após a semente do RSI,
séries constantes e direcionais, limites de histórico, períodos e preços
inválidos, janela final, preservação dos dados e soma de preços acima de `int64`.

```sh
go test ./internal/domain/analytics
go test ./...
go vet ./...
go build ./...
```
