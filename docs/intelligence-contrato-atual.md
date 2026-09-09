# Asset Intelligence: contrato atual

`GET /assets/{ticker}/intelligence` consolida cálculos determinísticos sobre
cotações publicadas do COTAHIST, sem ajuste por eventos corporativos.

## Indicadores oferecidos

| Bloco | Campos |
|---|---|
| price | close, currency |
| returns | return_7d |
| trend | sma20, distance_sma20 |
| momentum | rsi14 |
| risk | volatility_30d, drawdown_current, maximum_drawdown_252d |
| liquidity | average_daily_volume_20d |

Janelas são medidas em pregões. RSI14 usa suavização de Wilder desde o início
do histórico publicado; drawdown atual usa janela de 252 pregões. Volatilidade
usa 30 retornos e é anualizada. Percentuais usam escala 100 (20 significa 20%).
Preços e volumes monetários são strings decimais; RSI e percentuais são números.

Não são publicados placeholders para percentil de RSI em três anos, benchmark,
força relativa, contexto de mercado, sinais, alertas, score ou rules_version.
Esses recursos continuam como evolução futura, exigindo metodologia e/ou fontes.

## Disponibilidade

`meta.status` é `complete` quando todos os indicadores oferecidos foram calculados.
É `partial` quando algum deles está indisponível. Nesse caso o campo retorna
`null` e `meta.unavailable` informa campo e motivo. Zero válido permanece zero.
Quando tudo está disponível, `unavailable` retorna `[]`, indicando ausência de
problemas; recursos futuros não geram entradas `not_implemented`.

Mantemos referência efetiva, fonte, unidades e versões para interpretar os
valores e identificar mudanças nas séries. `requested_as_of: null` significa
que o cliente não informou uma data. Não representa um indicador faltante.

## Consultas

```http
GET /assets/PETR4/intelligence
GET /assets/PETR4/intelligence?asOf=2023-12-28&marketType=10
GET /assets/PETR4/intelligence?includeDetails=true
```

`includeDetails` aceita somente `true` ou `false` e assume `false`.
A cobertura anual `meta.calendar.coverage` é omitida por padrão e incluída
quando solicitada e disponível. Fonte, política e versão do calendário continuam
na resposta compacta. Esse parâmetro altera apenas a apresentação, não os cálculos.

O calendário observado é lido junto às cotações no mesmo snapshot PostgreSQL
somente leitura. Veja [a política de calendário](calendario-intelligence.md).
Histórico insuficiente, lacunas e cobertura inválida impedem somente os cálculos
afetados. A IA pode interpretar resultados, sem substituir os cálculos.

A remoção de campos altera o contrato JSON anterior: consumidores que exigiam
placeholders devem ser atualizados. O contrato está descrito em [OpenAPI](../openapi.yaml).
