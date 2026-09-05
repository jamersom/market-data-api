# Camada de aplicação de comparações

`services.NewCompareQuotesService(repository, logger)` implementa
`inbound.CompareQuotesUseCase`. O caso de uso recebe `CompareQuotesInput` e
retorna `CompareQuotesOutput`, sem depender de HTTP ou PostgreSQL.

A porta `outbound.ComparisonQuoteRepository` recebe todos os tickers em uma
única chamada, incluindo o benchmark quando necessário. Sua implementação deve
retornar o histórico completo do período inclusivo, sem paginação, com um único
registro por ticker/data no mercado solicitado. Ausência de dados é representada
por uma lista vazia, não por um erro que interrompa a consulta inteira.

## Comportamento

- Usa as validações de tickers e métricas do domínio.
- Exige datas e limita o intervalo a cinco anos; considera a data civil de cada
  parâmetro e a representa à meia-noite UTC.
- Aplica mercado 10 quando omitido e rejeita valores negativos.
- Ordena uma cópia das cotações, preservando os dados do repositório.
- Exige dois fechamentos positivos em datas distintas para um ativo participar.
  Datas duplicadas, datas vazias e moedas explicitamente diferentes de BRL
  tornam o ativo insuficiente. Moeda vazia assume a convenção BRL da API.
- Exige pelo menos dois ativos disponíveis, sem contar o benchmark adicional.
- Preserva a ordem solicitada e informa ativos com dados insuficientes.
- Calcula somente as métricas solicitadas; preços inicial/final e datas de
  disponibilidade acompanham todo ativo disponível.
- Inclui séries apenas com `IncludeSeries=true`.
- O benchmark é um ticker consultado pela mesma fonte. Retorna seu resumo
  separadamente, sem incluí-lo automaticamente nos rankings ou correlações.
  Identificadores como IBOV só terão dados se o repositório os disponibilizar;
  não há resolução de índices ou cálculo de excesso de retorno nesta camada.

## Convenções de cálculo — versão 1.0

- Retorno: diferença dos fechamentos em centavos e variação percentual.
- Retornos diários: razão entre fechamentos consecutivos disponíveis menos um.
- Volatilidade: desvio padrão **amostral** dos retornos, multiplicado por
  `sqrt(252) * 100`. Exige pelo menos dois retornos (três cotações); caso
  contrário o campo fica ausente, mesmo que o ativo participe da comparação.
- Drawdown: menor variação percentual em relação ao maior fechamento acumulado.
  É negativo ou zero; o ranking de menor drawdown favorece o mais próximo de zero.
- Volume médio: soma em inteiros de precisão arbitrária, dividida pela quantidade
  de pregões. Como o domínio representa a média em centavos inteiros, a fração
  de centavo é truncada somente ao produzir o resultado final.
- Máxima/mínima: extremos dos preços intradiários, não dos fechamentos.
- Melhor/pior dia: extremos dos retornos diários; empate mantém a primeira data.
- Correlação: Pearson dos retornos com as mesmas datas inicial e final. Exige
  dois intervalos comuns e variância positiva nas duas séries. Pares indefinidos
  são omitidos; não são substituídos por zero. A ordem é determinística.
- Rankings consideram apenas métricas disponíveis; empate mantém o primeiro
  ticker solicitado.
- Séries normalizadas começam em 100. Preços são não ajustados por eventos
  corporativos, conforme `PriceAdjustment = "unadjusted"`.

## Integração HTTP e PostgreSQL

`postgres.QuoteRepository` implementa a consulta conjunta com `ANY($1::text[])`,
filtra mercado e período inclusivo e retorna os registros ordenados por ticker
e data. A consulta usa a visão `published_historical_quotes` e não possui
paginação. Dados ausentes retornam listas vazias.

`handlers.ComparisonsHandler` expõe `GET /comparisons` e `POST /comparisons`,
registrados em `routes.go` e conectados ao serviço em `cmd/main.go`. Ambos usam
o mesmo mapper de entrada e resposta. Valores monetários são strings com duas
casas; percentuais e séries normalizadas são arredondados apenas na resposta.
O coeficiente de correlação mantém a precisão do cálculo.

O POST exige `application/json`, aceita apenas um objeto com campos conhecidos
e limita o corpo a 64 KiB. O GET rejeita parâmetros repetidos ou desconhecidos.
Mercado explicitamente informado deve ser positivo. Cada execução possui
timeout de cinco segundos, propagado ao PostgreSQL pelo contexto.

Erros retornam o envelope padrão da API: `400` para entradas inválidas, `413`
para corpo excessivo, `415` para mídia incompatível, `422` para dados
insuficientes, `500` para erros internos e `504` para timeout. O contrato está
descrito no `openapi.yaml`, também servido pela API e pelo Swagger em `/docs/`.

Exemplo de consulta:

```http
GET /comparisons?tickers=PETR4,VALE3&from=2025-01-01&to=2025-12-31&metrics=return,volatility,drawdown&includeSeries=true
```

Os testes de integração usam apenas leitura de dados publicados no banco
configurado por `.env` ou `DATABASE_URL`. Exigem históricos de PETR4 e VALE3.
Para executá-los no PowerShell:

```powershell
$env:DATABASE_INTEGRATION_TEST = '1'
go test ./internal/adapters/outbound/postgres -count=1 -v
```

Validação local: `go test ./...`, `go vet ./...` e `go build ./...`.
