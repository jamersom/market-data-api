# Visão geral do projeto

O `market-data-api` é uma API HTTP somente leitura para dados de mercado publicados do B3 COTAHIST. Oferece a última cotação, histórico paginado, comparações entre ativos e indicadores determinísticos de análise individual. A aplicação lê dados publicados pelo projeto separado `b3-data-hub` e disponibiliza seu contrato OpenAPI e a interface Swagger UI.

Rotas HTTP atuais:

- `GET /quotes/{ticker}`
- `GET /quotes/{ticker}/history`
- `GET /comparisons`
- `POST /comparisons`
- `GET /assets/{ticker}/intelligence`
- `GET /openapi.yaml`
- `GET /docs` e `GET /docs/`

Não há `README.md` neste repositório. Considere `openapi.yaml` o contrato público atual da API e consulte `docs/` para detalhes da implementação, convenções de cálculo, arquitetura e trabalho planejado. Alguns documentos preservam propostas antigas; dê preferência aos explicitamente identificados como contrato atual ou lista atual de pendências.

# Tecnologias utilizadas

- Go `1.26.4`, conforme declarado em `go.mod`.
- `net/http` da biblioteca padrão, com rotas do `ServeMux` que incluem o método HTTP; não é utilizado um framework HTTP externo.
- PostgreSQL por meio de `github.com/jackc/pgx/v5` e `pgxpool` (`v5.10.0`).
- Logs estruturados com `log/slog` e `github.com/jamersom/go-logging` (`v1.0.0`).
- Carregamento opcional do `.env` local com `github.com/joho/godotenv` (`v1.5.1`).
- OpenAPI `3.1.0`; a versão atual do documento da API é `1.4.0`.
- Builds Docker em múltiplas etapas com `golang:1.26-alpine` e `alpine:3.22`.
- GitHub Actions para validação e publicação de imagens no GHCR.

# Arquitetura

O código segue uma organização de portas e adaptadores (arquitetura hexagonal), com dependências direcionadas para as camadas internas:

```text
Adaptadores HTTP/PostgreSQL -> portas e serviços de aplicação -> domínio
```

- `internal/domain` contém entidades, tipos de valores, regras de validação, erros de domínio, tipos de comparação e metadados do calendário. Não deve depender de HTTP, JSON, PostgreSQL ou `pgx`.
- `internal/domain/analytics` contém cálculos puros e determinísticos. Essas funções recebem séries previamente validadas e não consultam dados nem formatam respostas HTTP.
- `internal/application/ports/inbound` define entradas, saídas e interfaces dos casos de uso.
- `internal/application/ports/outbound` define os contratos de repositório necessários aos casos de uso.
- `internal/application/services` valida e normaliza entradas da aplicação, coordena repositórios, aplica cálculos do domínio e monta as saídas da aplicação.
- `internal/adapters/inbound/http` registra as rotas. Seu pacote `handlers` interpreta entradas HTTP, aplica limites e timeouts, invoca portas de entrada, converte saídas em DTOs de resposta e mapeia erros para respostas HTTP.
- `internal/adapters/outbound/postgres` implementa as portas de repositório com SQL parametrizado sobre objetos publicados no PostgreSQL.
- `internal/infra/database` cria e verifica o pool de conexões PostgreSQL.
- `cmd/main.go` é o ponto de entrada do executável e o local de composição das dependências. Cria o logger, pool do banco, repositórios, serviços, handlers, roteador e servidor HTTP.

Fluxo principal de uma requisição:

```text
Requisição HTTP -> handler/interpretação de parâmetros -> caso de uso de entrada
-> serviço de aplicação -> repositório de saída -> PostgreSQL
-> saída da aplicação -> mapper/DTO HTTP -> JSON
```

As dependências são conectadas manualmente em `cmd/main.go`; não há framework de injeção de dependências. A API realiza somente leituras. A ingestão, a publicação e a responsabilidade pelo schema pertencem ao `b3-data-hub`.

# Estrutura do projeto

```text
.
|-- cmd/main.go                         executável e conexão das dependências
|-- internal/
|   |-- domain/                         entidades, regras, erros e cálculos
|   |-- application/
|   |   |-- ports/inbound/              contratos dos casos de uso e DTOs da aplicação
|   |   |-- ports/outbound/             contratos de repositório
|   |   `-- services/                   implementações dos casos de uso
|   |-- adapters/
|   |   |-- inbound/http/               rotas, handlers, DTOs de resposta e mappers
|   |   `-- outbound/postgres/          implementações de repositório com pgx
|   `-- infra/database/                 configuração do pool PostgreSQL
|-- docs/                               documentação da arquitetura e funcionalidades
|-- openapi.yaml                        contrato público HTTP
|-- docs.go                             incorpora openapi.yaml ao binário
|-- Dockerfile                          build da imagem de produção
|-- docker-compose.yml                  configuração local do contêiner da API
|-- docker-stack.yml                    configuração de implantação no Swarm
`-- .github/workflows/                  CI e publicação da imagem de desenvolvimento
```

Não existem `pkg/`, `migrations/` ou `Makefile` neste repositório. Adicione código à camada existente responsável pela funcionalidade, evitando criar uma nova estrutura na raiz sem necessidade demonstrada.

# Diretrizes de desenvolvimento

- Preserve a direção das dependências e a separação atual entre domínio, aplicação e adaptadores.
- Coloque regras de negócio e fórmulas quantitativas no domínio ou no pacote de cálculos puros. Coloque a orquestração e a validação dos casos de uso nos serviços de aplicação. Não coloque regras de negócio em handlers HTTP ou adaptadores SQL.
- Defina portas de entrada e saída com escopo restrito para novos casos de uso. Verifique as portas existentes antes de adicionar outra abstração.
- Mantenha os handlers responsáveis pelas questões HTTP: caminhos, interpretação da query e do corpo, tipo de conteúdo, limites de tamanho, timeouts e mapeamento de respostas e erros.
- Mantenha os repositórios responsáveis pela persistência, retornando tipos do domínio ou da aplicação em vez de DTOs HTTP.
- Atualize `openapi.yaml`, DTOs de resposta, mappers, handlers e testes em conjunto ao alterar o contrato público.
- Preserve os endpoints existentes, salvo quando uma alteração de contrato explicitamente solicitada exigir o contrário.
- Reutilize `internal/domain/analytics` para cálculos financeiros compartilhados. Não duplique fórmulas em handlers ou serviços individuais.
- Os cálculos financeiros devem permanecer determinísticos. Não use um LLM para calcular ou preencher valores financeiros ausentes.
- Não crie preços sintéticos para pregões ausentes. Preserve o comportamento explícito de indisponibilidade ou insuficiência de dados.
- Mantenha preços e valores monetários em centavos inteiros internamente. Formate-os como strings decimais na saída HTTP. Evite ponto flutuante para valores monetários armazenados.
- Percentuais usam unidades percentuais: `20` significa 20%, e não `0.20`. Arredonde somente no mapeamento da resposta quando essa for a convenção do contrato.
- Use ponteiros para valores calculados opcionais quando zero for um resultado válido e precisar ser distinguido de ausência.
- Preserve slices e mapas pertencentes ao chamador quando o código atual os copia antes de ordenar ou normalizar.
- Propague `context.Context` entre handlers, serviços e repositórios; respeite cancelamento e timeouts estabelecidos para as requisições.
- Prefira alterações pequenas, incrementais e fáceis de revisar. Evite refatorações sem relação com a tarefa e dependências desnecessárias.

# Convenções de código

- Use a formatação e os padrões idiomáticos de Go. Os nomes dos pacotes são curtos e em minúsculas; identificadores exportados seguem as convenções de nomenclatura de Go.
- Construtores usam nomes no formato `New<Type>`. Serviços expõem `Execute(ctx, input)` por meio das interfaces de entrada.
- Use verificações de implementação de interface em tempo de compilação, como `var _ Port = (*Implementation)(nil)`, onde o código ao redor segue esse padrão.
- Normalize e valide entradas de negócio reutilizáveis nas camadas de domínio e aplicação. A validação sintática específica do transporte fica nos handlers.
- A validação de domínio usa erros sentinela e `domain.ValidationError`. Recursos ausentes usam `domain.ResourceNotFoundError`, que permite acessar o sentinela correspondente por meio de `Unwrap`.
- Acrescente contexto útil às falhas de infraestrutura e repositório com `fmt.Errorf("...: %w", err)`, preservando o funcionamento de `errors.Is` e `errors.As`.
- Centralize o mapeamento de erros no envelope HTTP definido em `handlers/error_handler.go`. Não exponha erros internos do banco ou segredos em respostas HTTP.
- O JSON segue o contrato estabelecido para cada endpoint. Respostas de cotações e comparações usam atualmente `camelCase`; Asset Intelligence usa `snake_case`. Não uniformize essa diferença sem uma alteração explícita do contrato da API.
- Datas recebidas por HTTP usam `YYYY-MM-DD` (`time.DateOnly`). A lógica de datas de comparação e Intelligence normaliza datas civis para meia-noite UTC nos pontos em que os serviços existentes fazem isso.
- As consultas PostgreSQL devem ser parametrizadas. O único trecho SQL dinâmico atual é a escolha validada entre `ASC` e `DESC` no histórico de cotações.
- Feche os resultados das consultas e propague erros de iteração e leitura. Mantenha o histórico de Intelligence e o calendário observado no mesmo snapshot somente leitura com isolamento `REPEATABLE READ`.
- Preserve as garantias de ordenação documentadas por cada porta de repositório.
- Adicione logs estruturados por meio do `*slog.Logger` injetado, usando chaves estáveis para ticker, tipo de mercado, quantidade de registros e duração. Nunca registre credenciais ou séries completas de dados de mercado.
- O tratamento HTTP atual de erros inesperados contém um TODO para logging estruturado no servidor; não afirme que isso já está implementado.

# Compilação e execução

Não há script próprio de automação dos comandos nem `Makefile`. Execute os comandos a partir da raiz do repositório.

Baixar dependências:

```sh
go mod download
```

Executar a aplicação localmente:

```sh
go run ./cmd
```

`DATABASE_URL` é obrigatória. `cmd/main.go` tenta carregar o `.env` da raiz e utiliza as variáveis de ambiente do processo quando esse arquivo não está disponível. O servidor escuta em `SERVER_ADDRESS`, cujo padrão é `:8080`.

Compilar todos os pacotes:

```sh
go build ./...
```

Compilar o executável com a variável de versão usada pelas imagens de produção:

```sh
go build -trimpath -ldflags="-X main.version=development" -o market-data-api ./cmd
```

Não há ferramenta de lint separada configurada. O repositório e o CI usam `go vet` para análise estática.

# Configuração

Variáveis da aplicação lidas diretamente pelo código Go:

- `DATABASE_URL`: string de conexão PostgreSQL obrigatória.
- `SERVER_ADDRESS`: endereço de escuta HTTP; padrão `:8080`.
- `APP_ENV`: incluída na configuração do logger.
- `LOG_LEVEL`: nível dos logs.
- `LOG_FORMAT`: formato de saída dos logs.
- `DB_MAX_CONNECTIONS`: máximo de conexões do pool; padrão `10`. Valores inválidos usam o padrão.
- `DB_MIN_CONNECTIONS`: mínimo de conexões do pool; padrão `2`. Valores inválidos usam o padrão.
- `DB_MAX_CONN_LIFETIME`: duração no formato de Go; padrão `30m`. Valores inválidos usam o padrão.
- `DB_CONNECT_TIMEOUT`: duração no formato de Go; padrão `5s`. Valores inválidos usam o padrão.

Os manifestos de contêiner e implantação também interpolam `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `API_PORT`, `API_REPLICAS` e `IMAGE_TAG`. `.env.example` é o modelo documentado. Nunca inclua `.env` ou credenciais em commits.

# Testes

Executar a suíte regular de testes:

```sh
go test ./...
```

Executar a validação local completa indicada na documentação, com formatação dos arquivos Go alterados:

```sh
gofmt -w <arquivos-go-alterados>
go test ./...
go vet ./...
go build ./...
```

Os testes unitários e HTTP usam o pacote padrão `testing`, casos organizados em tabelas quando útil, fakes e stubs para as portas e `net/http/httptest` para os handlers. Adicione testes próximos ao código testado. Cubra limites de validação, valores padrão, encadeamento de erros, cancelamento, valores opcionais indisponíveis, ordenação e contratos HTTP exatos quando esses comportamentos forem alterados.

Os testes de integração PostgreSQL são habilitados explicitamente e usam dados publicados existentes. Eles não criam massas de dados nem modificam o banco:

```powershell
$env:DATABASE_INTEGRATION_TEST = '1'
go test ./internal/adapters/outbound/postgres -count=1 -v
Remove-Item Env:DATABASE_INTEGRATION_TEST
```

Esses testes exigem banco configurado e históricos publicados de PETR4 e VALE3. O teste de integração do repositório de cotações exige especificamente o `.env` da raiz; o teste de comparação aceita `.env` ou uma `DATABASE_URL` já definida.

A integração com o calendário observado é habilitada separadamente:

```powershell
$env:CALENDAR_INTEGRATION_TEST = '1'
go test ./internal/adapters/outbound/postgres -run '^TestObservedIntelligenceIntegration$' -count=1 -v
Remove-Item Env:CALENDAR_INTEGRATION_TEST
```

Ela exige histórico de PETR4 e as views de calendário observado fornecidas pelas migrations 003/004 do `b3-data-hub`. A execução normal de `go test ./...` pula esses testes dependentes do banco, a menos que as variáveis de habilitação estejam definidas.

# Banco de dados

A API usa PostgreSQL por meio de `pgxpool`. A criação do pool interpreta `DATABASE_URL`, aplica limites e tempo de vida das conexões, cria o pool com timeout de conexão e verifica a conectividade com `Ping`.

Os repositórios consultam objetos publicados no schema `public`, incluindo:

- `published_historical_quotes`
- `historical_imports`
- `observed_trading_sessions`
- `observed_calendar_coverage`

Preços armazenados como valores numéricos no PostgreSQL são convertidos para centavos inteiros nas projeções das consultas. O histórico de cotações é paginado com consultas de contagem e de dados. Comparações carregam todos os tickers solicitados em uma única consulta com `ANY($1::text[])`, evitando consultas N+1. Asset Intelligence carrega todo o histórico publicado até `asOf`; o adaptador de calendário observado lê histórico e calendário em uma única transação somente leitura com isolamento `REPEATABLE READ`.

Este repositório não contém migrations e não é responsável pelo schema dos dados de mercado. O schema necessário e as migrations do calendário observado ficam no `b3-data-hub`; a documentação atual de Intelligence exige as migrations 003/004 desse projeto. Não edite uma migration já aplicada. Quando uma alteração de schema for necessária, crie uma nova migration no repositório responsável pelo schema e coordene a alteração da API com ela. Se a responsabilidade não estiver clara, informe isso explicitamente antes de alterar código relacionado ao schema.

# Docker

Construir a imagem diretamente:

```sh
docker build --build-arg VERSION=development -t market-data-api .
```

A imagem compila um binário Linux estático e o executa com o usuário `app`, sem privilégios de root, na porta 8080.

O `docker-compose.yml` versionado constrói a API e exige uma rede Docker externa existente chamada `b3-data-hub-network`, com PostgreSQL acessível pelo nome `postgres`. Com esse pré-requisito e um `.env` configurado, execute:

```sh
docker compose up --build
```

O `docker-stack.yml` versionado implanta a imagem do GHCR no Docker Swarm, exige redes externas chamadas `b3-data-hub_b3_data_hub` e `DockerNet` e configura o roteamento Traefik para `marketdata.techcomp.net.br`. O repositório não documenta um nome padrão de stack nem um comando automatizado de implantação. Nenhum dos dois manifestos define uma verificação de saúde do contêiner.

# CI/CD

O workflow `.github/workflows/ci.yml` é executado para pull requests destinados à branch `dev`, por acionamento manual e por chamadas como workflow reutilizável. Seu job de validação:

1. Faz o checkout do repositório.
2. Seleciona a versão de Go a partir de `go.mod` e habilita o cache de dependências.
3. Executa `go mod download`.
4. Executa `go test ./...`.
5. Executa `go vet ./...`.
6. Constrói a imagem Docker Linux/amd64 sem publicá-la.

O workflow `.github/workflows/publish-dev.yml` é executado após pushes para `dev` ou por acionamento manual. Primeiro chama o workflow de validação; depois constrói e publica imagens `linux/amd64` no GHCR com a tag imutável `sha-<short-sha>` e a tag atualizável `dev`. Injeta `dev-<short-sha>` em `main.version` por meio do argumento de build Docker.

Antes de abrir um PR, execute `gofmt` nos arquivos Go alterados, `go test ./...`, `go vet ./...` e `go build ./...`. Não há limite mínimo de cobertura configurado, linter independente, workflow de release ou workflow automatizado de implantação visível neste repositório.

# Instruções para agentes

- Leia este arquivo, `openapi.yaml` e o documento atual relevante em `docs/` antes de alterar comportamentos.
- Verifique `git status` antes de editar. Preserve alterações do usuário e não sobrescreva arquivos modificados ou não rastreados que não estejam relacionados à tarefa.
- Preserve a arquitetura existente e a direção das dependências.
- Evite refatorações fora do escopo solicitado.
- Não adicione dependências, a menos que o comportamento solicitado exija claramente uma e a biblioteca padrão ou os pacotes existentes não atendam à necessidade.
- Reutilize interfaces, portas, DTOs, mappers, cálculos, auxiliares de validação e projeções dos repositórios antes de criar uma nova abstração.
- Mantenha compatibilidade com a nomenclatura, o tratamento de erros, os logs e o estilo dos testes próximos ao código alterado.
- Não coloque regras de negócio em handlers HTTP.
- Respeite os limites entre regras de domínio, casos de uso da aplicação e adaptadores de entrada e saída.
- Mantenha o acesso ao banco somente leitura, salvo alteração explícita do escopo do projeto e da responsabilidade pelo schema.
- Não modifique migrations existentes sem necessidade demonstrada. Crie uma nova migration para alterações de schema no repositório responsável por ele.
- Mantenha `openapi.yaml` sincronizado com alterações públicas de HTTP; ele é incorporado ao binário por `docs.go`.
- Não deduza comportamentos implementados a partir de propostas quando o código, o contrato atual ou o OpenAPI indicar algo diferente. Informe explicitamente inconsistências não resolvidas.
- Não inclua segredos de `.env`, logs, strings de conexão ou credenciais de CI em código, testes, documentação ou respostas.
- Execute ou informe claramente a execução de `gofmt`, `go vet ./...` e `go test ./...` após alterar código Go. Execute `go build ./...` quando a mudança puder afetar a compilação ou a conexão das dependências.
- Prefira alterações pequenas, incrementais e fáceis de revisar.
- Informe o que mudou, por quê, quais verificações foram executadas e quais não puderam ser executadas.

# Antes de fazer alterações

1. Entenda a solicitação e seus critérios de aceite.
2. Identifique os arquivos e contratos públicos envolvidos.
3. Localize e siga implementações e testes semelhantes já existentes.
4. Avalie o impacto na arquitetura, na API, nos dados e na compatibilidade.
5. Implemente a menor mudança necessária.
6. Formate os arquivos Go alterados e execute os testes e verificações relevantes.
7. Informe claramente o que foi alterado, a validação realizada e as limitações não resolvidas.
