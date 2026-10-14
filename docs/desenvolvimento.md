# Desenvolvimento e arquitetura atual

Este documento reúne a estrutura implementada e as convenções técnicas do
`market-data-api`. O ponto de entrada para agentes é o [AGENTS.md](../AGENTS.md).
A API HTTP consulta somente dados de mercado publicados do B3 COTAHIST pelo
projeto separado `b3-data-hub`: última cotação, histórico paginado, comparações
entre ativos e indicadores determinísticos de análise individual.

## Fontes e escopo

- O [OpenAPI](../openapi.yaml) define o contrato público atual e é incorporado ao
  binário por [docs.go](../docs.go). Não há `README.md` neste repositório.
- Para cálculos, leia [analytics](analytics.md); para comparações, a
  [camada de aplicação de comparações](comparison-application.md).
- Para Intelligence, consulte o [contrato atual](intelligence-contrato-atual.md),
  a [aplicação](asset-intelligence-application.md) e o
  [calendário observado](calendario-intelligence.md).
- Execução, configuração, Docker e CI/CD estão no
  [guia operacional de desenvolvimento](operations/development-guide.md).
  Comandos e pré-requisitos de testes estão no
  [checklist de validação](operations/validation-checklist.md).
- Logs e comportamento operacional estão no
  [núcleo operacional](operations/operational-core.md).
- [Arquitetura e evolução da plataforma](arquitetura-plataforma.md) inclui
  recomendações e uma árvore de módulos futura. Essa árvore não descreve a
  estrutura atual nem exige uma reorganização do código.

As [pendências consolidadas](pendencias-consolidadas.md) distinguem implementação,
contrato atual e propostas. Sua Etapa 0 registra duas ressalvas documentais:
`comparison-application.md` ainda afirma que Intelligence não foi implementado;
`calendario-intelligence.md` ainda associa `partial` a recursos futuros ausentes.
O endpoint já existe e o estado considera os indicadores oferecidos pelo contrato
vigente. Não deduza comportamentos de propostas quando o código, o contrato atual
ou o OpenAPI indicarem algo diferente; informe inconsistências não resolvidas.

## Rotas HTTP atuais

| Método e caminho | Responsabilidade |
|---|---|
| `GET /quotes/{ticker}` | Última cotação publicada |
| `GET /quotes/{ticker}/history` | Histórico paginado |
| `GET /comparisons` | Comparação por parâmetros de query |
| `POST /comparisons` | Comparação por corpo JSON, sem persistência |
| `GET /assets/{ticker}/intelligence` | Indicadores determinísticos do ativo |
| `GET /health/live` | Liveness do processo HTTP |
| `GET /health/ready` | Readiness com verificação do PostgreSQL |
| `GET /metrics` | Métricas operacionais agregadas |
| `GET /openapi.yaml` | Contrato OpenAPI |
| `GET /docs` e `GET /docs/` | Interface Swagger UI |

As rotas são registradas em [routes.go](../internal/adapters/inbound/http/routes.go).
Preserve os endpoints existentes, salvo quando uma alteração de contrato
explicitamente solicitada exigir o contrário.

## Tecnologias

As versões das dependências são declaradas em [go.mod](../go.mod):

| Componente | Tecnologia atual |
|---|---|
| Linguagem | Go `1.26.4` |
| HTTP | `net/http` da biblioteca padrão e rotas do `ServeMux` com método HTTP; sem framework HTTP externo |
| PostgreSQL | `github.com/jackc/pgx/v5` e `pgxpool`, versão `v5.10.0` |
| Logs | `log/slog` e `github.com/jamersom/go-logging`, versão `v1.0.0` |
| Ambiente local | Carregamento opcional de `.env` com `github.com/joho/godotenv`, versão `v1.5.1` |
| Contrato HTTP | OpenAPI `3.1.0`, documento da API na versão `1.8.0` |

Os builds Docker e a validação/publicação por GitHub Actions no GHCR são
detalhados no [guia operacional](operations/development-guide.md).

## Arquitetura implementada

O projeto segue portas e adaptadores, com dependências direcionadas para dentro:

```text
Adaptadores HTTP/PostgreSQL -> portas e serviços de aplicação -> domínio
```

| Caminho existente | Responsabilidade e limites |
|---|---|
| [internal/domain](../internal/domain) | Entidades, tipos de valores, validação, erros, comparação e metadados do calendário. Não depende de HTTP, JSON, PostgreSQL ou `pgx`. |
| [internal/domain/analytics](../internal/domain/analytics) | Cálculos puros e determinísticos sobre séries previamente validadas, sem consultas a dados nem formatação HTTP. |
| [internal/application/ports/inbound](../internal/application/ports/inbound) | Entradas, saídas e interfaces dos casos de uso. |
| [internal/application/ports/outbound](../internal/application/ports/outbound) | Contratos mínimos de repositório exigidos pelos casos de uso. |
| [internal/application/services](../internal/application/services) | Validação e normalização da aplicação, coordenação de repositórios, uso dos cálculos do domínio e montagem das saídas. |
| [internal/adapters/inbound/http](../internal/adapters/inbound/http) | Registro de rotas e adaptação HTTP. |
| [handlers](../internal/adapters/inbound/http/handlers) | Interpretação de path, query e corpo, tipo de conteúdo, limites de tamanho, timeouts, chamada das portas de entrada e mapeamento de respostas/erros. |
| [response](../internal/adapters/inbound/http/response) | DTOs de resposta HTTP. |
| [internal/adapters/outbound/postgres](../internal/adapters/outbound/postgres) | Implementações de repositório com SQL parametrizado sobre objetos publicados. |
| [internal/infra/database](../internal/infra/database) | Criação e verificação do pool PostgreSQL. |
| [internal/observability](../internal/observability) | Métricas operacionais agregadas em memória. |
| [cmd/main.go](../cmd/main.go) | Executável e composição manual de logger, pool, repositórios, serviços, handlers, roteador e servidor HTTP. |

Fluxo principal de uma requisição:

```text
Requisição HTTP -> handler/interpretação de parâmetros -> caso de uso de entrada
-> serviço de aplicação -> repositório de saída -> PostgreSQL
-> saída da aplicação -> mapper/DTO HTTP -> JSON
```

Não há framework de injeção de dependências. A ingestão, a publicação e a
responsabilidade pelo schema pertencem ao `b3-data-hub`.

Outros arquivos e diretórios da estrutura atual:

| Caminho | Finalidade |
|---|---|
| [docs](.) | Documentação técnica, operacional e de evolução |
| [openapi.yaml](../openapi.yaml) e [docs.go](../docs.go) | Contrato público e incorporação ao binário |
| [Dockerfile](../Dockerfile) | Build da imagem de produção |
| [docker-compose.yml](../docker-compose.yml) | Configuração local do contêiner da API |
| [docker-stack.yml](../docker-stack.yml) | Configuração de implantação no Swarm |
| [.github/workflows](../.github/workflows) | CI e publicação da imagem de desenvolvimento |

Não existem `pkg/`, `migrations/` ou `Makefile` neste repositório. Adicione código
à camada existente responsável pela funcionalidade; evite criar estruturas na
raiz sem necessidade demonstrada.

## Distribuição de responsabilidades

- Coloque regras de negócio e fórmulas quantitativas no domínio ou no pacote de
  cálculos puros. Orquestração e validação dos casos de uso ficam nos serviços.
  Não coloque regras de negócio em handlers HTTP ou adaptadores SQL.
- Valide e normalize entradas de negócio reutilizáveis no domínio/aplicação.
  A validação sintática específica do transporte pertence aos handlers.
- Repositórios retornam tipos do domínio ou da aplicação, nunca DTOs HTTP.
- Defina portas de entrada e saída com escopo restrito. Reutilize interfaces,
  portas, DTOs, mappers, cálculos, auxiliares de validação e projeções existentes
  antes de criar outra abstração.
- Reutilize `internal/domain/analytics` para fórmulas compartilhadas, sem
  duplicá-las em handlers ou serviços individuais. As regras de determinismo,
  ausência de dados, dinheiro, percentuais e arredondamento estão em
  [analytics](analytics.md) e devem ser preservadas.
- Preserve slices e mapas pertencentes ao chamador quando o código atual os
  copia antes de ordenar ou normalizar. Use ponteiros para valores calculados
  opcionais quando zero válido precisar ser distinguido de ausência.
- Propague `context.Context` entre handlers, serviços e repositórios, respeitando
  cancelamento e os timeouts estabelecidos para as requisições.
- Prefira mudanças pequenas e incrementais, sem refatorações fora do escopo.
  Não adicione dependências sem necessidade clara que a biblioteca padrão ou os
  pacotes existentes não atendam.

## Convenções Go, erros e HTTP

- Use formatação e padrões idiomáticos de Go: pacotes curtos e em minúsculas,
  identificadores exportados conforme as convenções da linguagem.
- Construtores usam `New<Type>`; serviços expõem `Execute(ctx, input)` pelas
  interfaces de entrada. Use verificações de interface em compilação, como
  `var _ Port = (*Implementation)(nil)`, onde o código próximo segue esse padrão.
- Mantenha nomenclatura, erros, logs e estilo dos testes compatíveis com as
  implementações semelhantes existentes.
- A validação de domínio usa sentinelas e `domain.ValidationError`. Recursos
  ausentes usam `domain.ResourceNotFoundError`, que expõe o sentinela por `Unwrap`.
- Acrescente contexto útil a falhas de infraestrutura e repositório com
  `fmt.Errorf("...: %w", err)`, preservando `errors.Is` e `errors.As`.
- Centralize o mapeamento de erros no envelope definido em
  [error_handler.go](../internal/adapters/inbound/http/handlers/error_handler.go).
  Não exponha erros internos do banco nem segredos em respostas HTTP.
- Preserve o JSON de cada endpoint: cotações e comparações usam `camelCase`;
  Asset Intelligence usa `snake_case`. Não uniformize sem alteração explícita
  do contrato da API.
- Datas HTTP usam `YYYY-MM-DD` (`time.DateOnly`). Comparações e Intelligence
  normalizam datas civis para meia-noite UTC nos pontos em que seus serviços
  existentes fazem isso.
- Ao alterar o contrato público, atualize conjuntamente OpenAPI, DTOs de
  resposta, mappers, handlers e testes. O contrato incorporado e o comportamento
  publicado precisam permanecer sincronizados.
- Use o `*slog.Logger` injetado, com chaves estáveis para ticker, tipo de mercado,
  quantidade de registros e duração. Nunca registre credenciais ou séries
  completas de dados de mercado. Erros HTTP inesperados registram método, rota
  normalizada, status e categoria; a resposta permanece genérica. Consulte as
  regras de logs no [núcleo operacional](operations/operational-core.md).

## Persistência somente leitura

O [pool PostgreSQL](../internal/infra/database/postgres.go) interpreta
`DATABASE_URL`, aplica limites e tempo de vida das conexões, cria o pool com
timeout de conexão e verifica a conectividade com `Ping`. Variáveis e padrões
estão no [guia operacional](operations/development-guide.md).

Os repositórios consultam objetos publicados no schema `public`, incluindo
`published_historical_quotes`, `historical_imports`,
`observed_trading_sessions` e `observed_calendar_coverage`.

- Use consultas parametrizadas. O único trecho SQL dinâmico atual é a escolha
  validada entre `ASC` e `DESC` no histórico de cotações.
- Feche resultados de consultas e propague erros de leitura e iteração.
  Preserve as garantias de ordenação documentadas por cada porta de repositório.
- Nas projeções SQL, converta preços numéricos do PostgreSQL para centavos
  inteiros, conforme as convenções de [analytics](analytics.md).
- O histórico de cotações usa consultas de contagem e de dados para paginação.
  Comparações carregam todos os tickers solicitados em uma única consulta com
  `ANY($1::text[])`, evitando N+1; veja a
  [aplicação de comparações](comparison-application.md).
- Asset Intelligence carrega todo o histórico publicado até `asOf`. Histórico
  e calendário observado devem permanecer no mesmo snapshot: uma transação
  somente leitura com isolamento `REPEATABLE READ`, conforme a
  [política do calendário](calendario-intelligence.md).

Mantenha o acesso ao banco somente leitura, salvo alteração explícita do escopo
do projeto e da responsabilidade pelo schema. Este repositório não contém
migrations nem é responsável pelo schema dos dados de mercado. O schema e as
migrations do calendário observado pertencem ao `b3-data-hub`; a integração
atual exige as migrations 003/004 desse projeto.

Não edite migrations já aplicadas. Se for necessário alterar o schema, crie uma
nova migration no repositório responsável e coordene a alteração da API. Quando
a responsabilidade não estiver clara, informe isso antes de alterar código
relacionado ao schema.

## Convenções de testes

Testes unitários e HTTP usam `testing`, tabelas quando úteis, fakes e stubs das
portas e `net/http/httptest` para handlers. Adicione testes próximos ao código
testado. Quando esses comportamentos mudarem, cubra limites de validação, valores
padrão, encadeamento de erros, cancelamento, valores opcionais indisponíveis,
ordenação e contratos HTTP exatos.

Os testes de integração usam dados publicados existentes e apenas leituras;
não criam massas de dados nem modificam o banco. Consulte o
[checklist de validação](operations/validation-checklist.md) para comandos,
ativação explícita, dados necessários e diferenças de configuração entre os
testes. Relate as verificações executadas e aquelas que não puderam ser feitas.
