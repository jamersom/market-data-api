# Caso de uso de Asset Intelligence

`NewGetAssetIntelligenceService(repository, logger)` implementa
`inbound.GetAssetIntelligenceUseCase`. O endpoint, handler e adaptador estão
conectados em `cmd/main.go`. Veja o [contrato atual](intelligence-contrato-atual.md)
e as [pendências reais](asset-intelligence-pendencias.md).

## Entrada e validação

O serviço normaliza ticker, assume mercado 10 quando omitido e rejeita datas
futuras considerando o dia civil de São Paulo. Datas de pregão são normalizadas
para meia-noite UTC sem mudar sua data civil. Sem cotação até a referência,
retorna `ErrQuoteNotFound`.

O histórico é ordenado e limitado à referência. Datas duplicadas ou vazias,
ticker/mercado incompatíveis e moeda diferente de BRL são erros de integridade.
Cancelamento e erros de infraestrutura são preservados.

## Histórico e calendário

`NewObservedIntelligenceQuoteRepository(pool)` lê cotações publicadas e as views
`observed_trading_sessions` e `observed_calendar_coverage` na mesma transação
PostgreSQL `REPEATABLE READ`, somente leitura. Requer migrations 003/004 do
`b3-data-hub`. A leitura de cotações reutiliza `NewIntelligenceQuoteRepository`.

A política `observed_import_integrity_v1` aceita o calendário consolidado das
negociações do mercado, com integridade das importações e cobertura por ano.
A fonte é `cotahist_observed`, com `official_verified=false`. O calendário não
é inferido apenas das cotações do ticker consultado. A política completa e
suas limitações estão em [calendario-intelligence.md](calendario-intelligence.md).

As janelas exigem pregões consecutivos até a cotação efetiva. Lacunas produzem
`missing_sessions`; cobertura ausente ou inválida afeta apenas as janelas
correspondentes. Não são criados preços artificiais.

RSI14 usa a semente nas primeiras quinze cotações do histórico publicado e
suavização de Wilder nas seguintes. `rsi_seed_from` informa o início. Uma lacuna
antiga pode impedir RSI mesmo com janelas recentes completas. Importações de
histórico anterior ou correções podem mudar seu valor.

## Resultado e versões

O serviço calcula retorno de sete pregões, SMA20, distância da média, RSI14,
volatilidade anualizada de 30 retornos, drawdown atual e máximo de 252 pregões
e volume financeiro médio de 20 pregões usando `internal/domain/analytics`.

`complete` significa que todos os indicadores oferecidos foram calculados.
Caso contrário retorna `partial`, campos `null` e motivos em `unavailable`.
Recursos futuros não aparecem no JSON e não geram `not_implemented`.
Fechamentos não positivos invalidam indicadores afetados; volume negativo
invalida a média. O mapper não apresenta fechamento inválido como preço válido.

`data_version` é um hash do histórico ordenado e metadados de publicação,
incluindo a identidade do ticker/mercado. O calendário possui hash separado.
Caches futuros devem considerar ambas as versões e a versão dos cálculos.
`asOf` consulta a versão atualmente publicada, não reconstrói o que se sabia
naquela data. Nenhuma cotação ou sessão posterior à referência entra no cálculo.

## HTTP

```http
GET /assets/PETR4/intelligence?asOf=2023-12-28&marketType=10
GET /assets/PETR4/intelligence?includeDetails=true
```

O handler rejeita parâmetros desconhecidos (inclusive `benchmark`), repetidos,
vazios ou malformados. `includeDetails` aceita `true` ou `false` e assume `false`;
controla a presença de `meta.calendar`, `meta.calculation_version`,
`meta.data_version` e `meta.rsi_seed_from`. `meta.requested_as_of` é omitido
quando `asOf` não é informado. Timeout de cinco segundos e cancelamento são
propagados ao repositório.

Erros: 400 para validação, 404 sem cotação, 500 para infraestrutura e 504 para
timeout. Respostas completas e parciais retornam 200. O envelope `data`/`meta`
usa `snake_case`; preços, SMA e volume financeiro são strings decimais com duas
casas. RSI e percentuais são arredondados apenas na saída. Zero válido permanece
zero. O contrato de `/comparisons` permanece independente.

## Validação

```powershell
go test ./...
go vet ./...
go build ./...
$env:CALENDAR_INTEGRATION_TEST = '1'
go test ./internal/adapters/outbound/postgres -run '^TestObservedIntelligenceIntegration$' -count=1 -v
Remove-Item Env:CALENDAR_INTEGRATION_TEST
```

O teste de integração usa `.env` ou configuração do banco e executa somente
leituras. Verifica histórico, calendário, limite temporal e resposta HTTP.
Requer PETR4 com histórico suficiente e as views consolidadas disponíveis.
