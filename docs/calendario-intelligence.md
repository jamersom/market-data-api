# Calendário do Asset Intelligence

O endpoint `GET /assets/{ticker}/intelligence` usa o calendário consolidado do
`b3-data-hub`, por meio de `NewObservedIntelligenceQuoteRepository`.

## Banco necessário

Aplicar as migrations 003 e 004 do `b3-data-hub` no mesmo PostgreSQL das cotações.
A API lê `public.observed_trading_sessions` e
`public.observed_calendar_coverage`; não consulta as views pesadas `_source` e
não grava nem reconstrói o calendário. Permissões de leitura são necessárias
para ambas as views. Falta de migration é erro de infraestrutura, não histórico
insuficiente.

## Política de aceitação

`observed_import_integrity_v1` aceita datas reconstruídas a partir das negociações
de todos os ativos no mesmo mercado, com origem `cotahist_observed`. Isso permite
calcular indicadores sem marcar o calendário como oficialmente verificado.

Para cada janela solicitada, o serviço exige:

- Um registro de cobertura por ano envolvido, sem duplicidades.
- Integridade validada das importações e limites observados válidos.
- Primeira/última cotação dentro dos limites observados dos respectivos anos.
- Cotações em sessões consecutivas no calendário consultado.

Nos anos intermediários, as sessões observadas de uma importação íntegra definem
o conjunto aceito pela política. Isso não detecta um dia inteiro omitido pelo
arquivo de origem, nem prova que dias ausentes sejam feriados. Não são criados
preços sintéticos. O calendário oficial independente continua sendo uma possível
fonte adicional, não uma classificação atribuída a esses dados.

O RSI usa a semente no início do histórico publicado e exige cobertura de todos
os anos até a referência. Um ano ausente impede RSI, mas não impede SMA20 ou
retorno de sete pregões se suas janelas recentes forem completas.

## Datas e consistência

Histórico e calendário são consultados em uma transação PostgreSQL somente
leitura, `REPEATABLE READ`. Uma publicação concorrente não mistura versões na
mesma resposta. O período de sessões vai da primeira cotação à última disponível
até `asOf`; fins de semana ou datas posteriores à última cotação não estendem
artificialmente a janela. Datas posteriores à referência nunca entram nos cálculos.

Os limites anuais de cobertura nos metadados representam a publicação atual;
por isso `observed_to` pode ser posterior a uma consulta histórica. Isso não
significa que sessões futuras tenham sido usadas. O endpoint não reconstrói a
versão dos arquivos que era conhecida no passado.

## Metadados HTTP

`meta.calendar` contém:

- `source`: `cotahist_observed`.
- `policy`: `observed_import_integrity_v1`.
- `version`: SHA-256 da política, mercado, intervalo, cobertura e sessões.
- `official_verified`: `false` para a fonte observada.
- `coverage`: ano, limites observados, integridade e versão de cada publicação; somente com `includeDetails=true` e cobertura disponível.

O hash do calendário deve acompanhar `data_version` e `calculation_version`
em qualquer cache futuro. Alterações de cobertura/versão mudam esse hash.

Motivos adicionais em `meta.unavailable`:

| Motivo | Significado |
|---|---|
| `calendar_missing_year` | Ano da janela sem cobertura publicada no mercado |
| `calendar_invalid_coverage` | Integridade falsa, duplicidade ou limites inválidos |
| `calendar_outside_coverage` | Janela extrapola os limites observados |
| `calendar_unavailable` | Nenhuma sessão ou fonte reconhecida |
| `missing_sessions` | Cotações não correspondem a sessões consecutivas |

Esses motivos não devem ser convertidos em indicadores zero. A resposta continua
parcial enquanto percentis, benchmark e regras futuras estiverem indisponíveis.

## Testes

```powershell
go test ./...
go vet ./...
go build ./...

$env:CALENDAR_INTEGRATION_TEST = '1'
go test ./internal/adapters/outbound/postgres -run '^TestObservedIntelligenceIntegration$' -count=1 -v
Remove-Item Env:CALENDAR_INTEGRATION_TEST
```

O teste de integração usa somente leituras, passando por repositório, serviço e
handler HTTP. Requer histórico de PETR4 com janelas recentes suficientes e as
views consolidadas. Os testes unitários cobrem transição de ano, cobertura
ausente/inválida, limites e indisponibilidade apenas das janelas afetadas.
