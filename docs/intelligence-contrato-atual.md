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

Mantemos referência efetiva, fonte e unidades para interpretar os valores.
Versões, calendário e semente do RSI ficam disponíveis na resposta detalhada.
`requested_as_of` é omitido quando o cliente não informa uma data.

## Consultas

```http
GET /assets/PETR4/intelligence
GET /assets/PETR4/intelligence?asOf=2023-12-28&marketType=10
GET /assets/PETR4/intelligence?includeDetails=true
```

`includeDetails` aceita somente `true` ou `false` e assume `false`.
`meta.calendar`, `meta.calculation_version`, `meta.data_version` e
`meta.rsi_seed_from` são omitidos por padrão e incluídos quando solicitado.
Quando o calendário é incluído, sua cobertura anual também é retornada quando
disponível. Esse parâmetro altera apenas a apresentação, não os cálculos.

## Exemplos de metadados

Resposta compacta, sem `asOf`:

```json
{
  "status": "complete",
  "market_type": 10,
  "as_of": "2026-09-04",
  "source": "B3 COTAHIST",
  "price_adjustment": "unadjusted",
  "window_unit": "trading_sessions",
  "percentage_unit": "percent",
  "unavailable": []
}
```

Resposta com `asOf` e `includeDetails=true`:

```json
{
  "calendar": {
    "source": "cotahist_observed",
    "version": "sha256-v1:<hash>",
    "policy": "observed_import_integrity_v1",
    "official_verified": false,
    "coverage": []
  },
  "status": "complete",
  "market_type": 10,
  "requested_as_of": "2026-09-04",
  "as_of": "2026-09-04",
  "source": "B3 COTAHIST",
  "price_adjustment": "unadjusted",
  "window_unit": "trading_sessions",
  "percentage_unit": "percent",
  "calculation_version": "1.0",
  "data_version": "sha256-v1:<hash>",
  "rsi_seed_from": "2023-01-02",
  "unavailable": []
}
```

O calendário observado é lido junto às cotações no mesmo snapshot PostgreSQL
somente leitura. Veja [a política de calendário](calendario-intelligence.md).
Histórico insuficiente, lacunas e cobertura inválida impedem somente os cálculos
afetados. A IA pode interpretar resultados, sem substituir os cálculos.

A remoção de campos altera o contrato JSON anterior: consumidores que exigiam
placeholders devem ser atualizados. O contrato está descrito em [OpenAPI](../openapi.yaml).
