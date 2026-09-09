> Contrato atualizado: consulte [intelligence-contrato-atual.md](intelligence-contrato-atual.md).
> Os trechos abaixo registram etapas anteriores. Campos futuros foram removidos
> do JSON; complete/partial considera apenas indicadores implementados.
> O calendário observado já está conectado e a cobertura anual exige includeDetails=true.

# Caso de uso de Asset Intelligence

> Atualização: o provedor observado está conectado no ponto de entrada da API.
> A leitura de histórico e calendário usa o mesmo snapshot somente leitura.
> A política e os metadados atuais estão em
> [calendario-intelligence.md](calendario-intelligence.md), substituindo as
> referências abaixo ao calendário como dependência ainda não fornecida.

> Implementado na camada de aplicação e no adaptador PostgreSQL, com testes
> unitários e de integração. Handler HTTP e rota também estão implementados.
> A fonte de calendário verificado ainda precisa ser fornecida.

`NewGetAssetIntelligenceService(repository, logger)` implementa
`inbound.GetAssetIntelligenceUseCase`. Recebe ticker, mercado e `AsOf` opcional.
Normaliza o ticker, assume mercado 10 quando omitido e rejeita datas futuras
considerando o dia civil de São Paulo. Datas de pregão são normalizadas para
meia-noite UTC, sem mudar sua data civil.

O serviço faz uma chamada a `outbound.IntelligenceQuoteRepository`, copia e
ordena as cotações e exclui qualquer observação posterior à referência. Retorna
a última cotação disponível até essa data; sem cotação retorna `ErrQuoteNotFound`.
Preserva erros de infraestrutura e cancelamento do contexto. Datas duplicadas,
datas vazias, ticker/mercado incompatíveis ou moeda diferente de BRL são erros
de integridade da consulta, não ausência de histórico.

## Histórico e calendário

A porta nova exige o histórico publicado completo até a data solicitada,
incluindo uma versão dos dados. A semente do RSI fica nas primeiras 15 cotações
desse histórico, com suavização nas seguintes. `RSISeedFrom` informa o início.
Isso evita reiniciar o RSI em uma janela móvel; correções ou importação de
histórico anterior podem mudar o valor e devem mudar `DataVersion`.

Essa escolha prioriza uma inicialização definida nesta etapa. O adaptador ainda
deverá avaliar custo, cobertura e versionamento antes de disponibilizar o caso
de uso em produção. Uma futura otimização por estado acumulado deve preservar
os mesmos resultados.

O repositório também deverá fornecer calendário completo de pregões entre o
início do histórico e a referência. `CalendarVerified` só pode ser verdadeiro
com cobertura verificada; derivar calendário das próprias cotações esconderia
lacunas e viola o contrato. Sem calendário, o serviço entrega preço e marca
indicadores como `calendar_unavailable` quando há observações suficientes.

As janelas usam pregões consecutivos até a cotação efetiva. Lacunas produzem
`missing_sessions`. Como o RSI utiliza toda a série, uma lacuna antiga impede
esse cálculo mesmo se as janelas recentes estiverem completas. Nenhum preço
é preenchido artificialmente. A data efetiva pode ser anterior à solicitada,
conforme o contrato proposto; o consumidor deve observar `AsOf`.

## Resultado da etapa

O resultado de aplicação tem campos opcionais para retorno de 7 pregões,
SMA20 em centavos fracionários, distância da SMA20, RSI14, volatilidade de 30
retornos, drawdown atual e máximo de 252 pregões e volume médio de 20 pregões.
Os cálculos reutilizam `internal/domain/analytics`.

`Unavailable` identifica cada campo indisponível e seu motivo: histórico
insuficiente, calendário ausente, lacunas, dado inválido ou recurso ainda não
implementado. Preços de fechamento não positivos invalidam os indicadores das
janelas afetadas; volume negativo invalida sua média. Se o fechamento mais
recente for inválido, `price.close` também é marcado como inválido. O objeto
`Price` conserva o registro de origem; o futuro mapper HTTP deverá respeitar
esses metadados e não apresentá-lo como um preço válido.

Metadados incluem referência solicitada/efetiva, versão do cálculo e dos dados,
fonte B3 COTAHIST, base `unadjusted`, percentuais em pontos percentuais de escala
(`20` representa 20%) e janelas em pregões. O resultado é sempre `partial`
nesta etapa: percentil, benchmark, força relativa, contexto, sinais, alertas e
score estão explicitamente marcados como `not_implemented`. Não há parâmetro
de benchmark ainda, para não aceitar uma opção que seria ignorada.

O resultado é um tipo de aplicação, sem tags JSON. O mapper HTTP publica o
envelope `data`/`meta` em `snake_case`, sem alterar o contrato de comparações.

## Adaptador PostgreSQL

`postgres.NewIntelligenceQuoteRepository(pool, calendar)` implementa a porta
com uma consulta parametrizada a `published_historical_quotes`, associada a
`historical_imports`. Filtra ticker, mercado e data final inclusiva e retorna
todo o histórico publicado em ordem crescente, sem paginação ou limite de 252
registros. A projeção e o scanner de cotações são compartilhados com o adaptador
existente. O histórico representa a versão atualmente publicada para datas de
pregão até `AsOf`, não uma reconstrução do que havia sido publicado naquela época.

`DataVersion` usa `sha256-v1:` seguido do hash dos registros ordenados, incluindo
preços e metadados de publicação, além da identidade do ticker/mercado. Corrigir
valores históricos altera a versão mesmo que a última data permaneça igual.
O hash não inclui o calendário e não deve ser usado sozinho como chave de cache
de indicadores quando a fonte de calendário puder mudar.

A dependência opcional `IntelligenceCalendar` recebe o intervalo completo, do
primeiro registro até a data solicitada. O adaptador fecha as linhas SQL antes
de consultá-la. Sem provedor, ou com cobertura não verificada, mantém
`CalendarVerified=false` e não retorna sessões presumidas. Erros do provedor
são propagados como falhas de infraestrutura.

Não foi encontrada tabela de calendário no esquema atual do `b3-data-hub`.
O calendário de agendamento daquele projeto não é automaticamente tratado como
fonte completa para todo o histórico. Nenhuma migração ou alteração no banco foi
necessária nesta etapa. Fornecer um calendário verificado continua pendente para
obter indicadores disponíveis com o adaptador real.

O teste `TestIntelligenceQuoteRepositoryIntegration` verifica, somente com
leituras, contagem completa, datas ordenadas, ausência de dados futuros, limites
inclusivos e filtros. Para executar usando `.env` ou `DATABASE_URL`:

```powershell
$env:DATABASE_INTEGRATION_TEST = '1'
go test ./internal/adapters/outbound/postgres -run '^TestIntelligenceQuoteRepositoryIntegration$' -count=1 -v
```

## Próximo passo

Fornecer calendário verificado e revisar o deploy. Handler, mapper, rota e
OpenAPI estão conectados ao caso de uso e adaptador real. `/comparisons`
permanece independente e compatível.

## HTTP

```http
GET /assets/PETR4/intelligence?asOf=2026-09-04&marketType=10
```

O handler rejeita parâmetros desconhecidos (inclusive `benchmark`), repetidos,
vazios ou malformados. Datas futuras são rejeitadas pelo serviço. Timeout de
cinco segundos e cancelamento são propagados ao repositório. Erros seguem o
envelope existente: 400 para validação, 404 sem cotação, 500 para infraestrutura
e 504 para timeout. Respostas parciais retornam 200.

Moeda e SMA são strings decimais com duas casas; percentuais e RSI são
arredondados a duas casas apenas na saída. Um zero disponível permanece zero;
um indicador indisponível retorna `null`. `signals` e `alerts` retornam `[]`
com `not_implemented` em `meta.unavailable`, e não indicam avaliação executada.

Exemplo de fragmento de resposta sem calendário (os demais blocos também são
incluídos na resposta completa):

```json
{
  "data": {"momentum": {"rsi14": null, "rsi14_percentile_3y": null}},
  "meta": {
    "status": "partial",
    "unavailable": [
      {"field": "momentum.rsi14", "reason": "calendar_unavailable"},
      {"field": "momentum.rsi14_percentile_3y", "reason": "not_implemented"}
    ]
  }
}
```
